// SPDX-License-Identifier: MIT
pragma solidity ^0.8.26;

import {console, Script} from "forge-std/Script.sol";
import {PulsarProtocol} from "../src/PulsarProtocol.sol";
import {PulsarProtocolOps} from "../src/PulsarProtocolOps.sol";
import {PulsarSwapHook} from "../src/v4/PulsarSwapHook.sol";
import {IUniswapV2Factory} from "../src/interfaces/IUniswapV2Factory.sol";
import {UUPSUpgradeable} from "@openzeppelin/contracts/proxy/utils/UUPSUpgradeable.sol";
import {IPoolManager} from "@uniswap/v4-core/src/interfaces/IPoolManager.sol";
import {Hooks} from "@uniswap/v4-core/src/libraries/Hooks.sol";
import {HookMiner} from "@uniswap/v4-periphery/src/utils/HookMiner.sol";
import {FullMath} from "@uniswap/v4-core/src/libraries/FullMath.sol";

interface IV2Pair {
    function getReserves() external view returns (uint112, uint112, uint32);
}

/// @notice Executes the V4-native cutover against the LIVE PulsarProtocol proxy:
///         upgrade to this branch's implementation, deploy PulsarProtocolOps (the
///         delegatecall target for the 20 functions moved out to fit under
///         EIP-170 — see PulsarProtocolStorage's docs) and wire it, deploy +
///         configure the fee hook, then migrate BOTH existing tickers (BUMIP,
///         ENRGP) from V2 to V4. Mirrors test/v4/PulsarProtocolLiveCutoverFork.t.sol,
///         which dry-ran this identical sequence against a fork of the real
///         proxy state before this script ever runs for real.
///
///         Two distinct signers, matching each action's actual on-chain role:
///         PRIVATE_KEY (admin) does the upgrade + ops/hook wiring; CUSTODIAN_1_PRIVATE_KEY
///         does migrateV2ToV4 (CUSTODIAN_ROLE, no multisig threshold on this call).
contract V4CutoverScript is Script {
    address constant POOL_MANAGER = 0xFB3e0C6F74eB1a21CC1Da29aeC80D2Dfe6C9a317;
    // Deterministic CREATE2 deployment proxy — present on virtually all EVM
    // chains including Arbitrum Sepolia (verified on-chain this session).
    // forge script routes `new X{salt: ...}(...)` through this factory during
    // broadcast, so HookMiner must mine against ITS address, not the admin EOA
    // or this script contract (see HookMiner's own docs on this distinction).
    address constant CREATE2_DEPLOYER = 0x4e59b44847b379578588920cA78FbF26c0B4956C;
    int24 constant TICK_SPACING = 60;
    uint24 constant LP_FEE = 3000;

    function _sqrt(uint256 x) internal pure returns (uint256 y) {
        if (x == 0) return 0;
        uint256 z = (x + 1) / 2;
        y = x;
        while (z < y) {
            y = z;
            z = (x / z + z) / 2;
        }
    }

    /// V4 pool-oriented sqrtPriceX96 (currency1/currency0, raw units) from the
    /// ticker's current V2 reserves. V2 sorts token0/token1 by address exactly
    /// like V4 sorts currency0/currency1, so r1/r0 IS the V4 pool price directly.
    /// Uses FullMath.mulDiv (512-bit-safe) — a naive `(r1<<192)/r0` silently wraps
    /// mod 2^256 for an 18-decimal token at realistic supply (learned the hard
    /// way in the dry run).
    function _v2PoolOrientedSqrtPrice(address pair) internal view returns (uint160) {
        (uint112 r0, uint112 r1,) = IV2Pair(pair).getReserves();
        uint256 ratioX192 = FullMath.mulDiv(uint256(r1), uint256(1) << 192, uint256(r0));
        return uint160(_sqrt(ratioX192));
    }

    function _migrate(PulsarProtocol protocol, address idrxAddr, address factoryAddr, string memory ticker)
        internal
    {
        address stockAddress = protocol.stocks(ticker);
        address pair = IUniswapV2Factory(factoryAddr).getPair(stockAddress, idrxAddr);
        uint160 sqrtPriceX96 = _v2PoolOrientedSqrtPrice(pair);
        protocol.migrateV2ToV4(ticker, sqrtPriceX96, TICK_SPACING, LP_FEE, 0, 0);
        console.log(string.concat(ticker, " migrated. isV4Migrated:"), protocol.isV4Migrated(ticker));
    }

    function run() external {
        uint256 adminKey = vm.envUint("PRIVATE_KEY");
        uint256 custodianKey = vm.envUint("CUSTODIAN_1_PRIVATE_KEY");
        address proxy = vm.envAddress("PULSAR_PROTOCOL_PROXY");
        address admin = vm.addr(adminKey);

        PulsarProtocol protocol = PulsarProtocol(proxy);
        address idrxAddr = protocol.idrx();
        address factoryAddr = protocol.router().factory();

        // ── ADMIN: upgrade the proxy, wire the ops split, deploy + configure the hook ──
        vm.startBroadcast(adminKey);

        PulsarProtocol newImpl = new PulsarProtocol();
        console.log("New implementation deployed:", address(newImpl));

        UUPSUpgradeable(proxy).upgradeToAndCall(address(newImpl), "");
        console.log("Proxy upgraded.");

        PulsarProtocolOps ops = new PulsarProtocolOps();
        protocol.setOpsContract(address(ops));
        console.log("PulsarProtocolOps deployed and wired:", address(ops));

        uint160 flags = uint160(
            Hooks.BEFORE_SWAP_FLAG | Hooks.AFTER_SWAP_FLAG | Hooks.BEFORE_SWAP_RETURNS_DELTA_FLAG
                | Hooks.AFTER_SWAP_RETURNS_DELTA_FLAG
        );
        bytes memory hookArgs = abi.encode(POOL_MANAGER, proxy, idrxAddr, admin, proxy);
        (address hookAddr, bytes32 salt) =
            HookMiner.find(CREATE2_DEPLOYER, flags, type(PulsarSwapHook).creationCode, hookArgs);

        PulsarSwapHook hook =
            new PulsarSwapHook{salt: salt}(IPoolManager(POOL_MANAGER), proxy, idrxAddr, admin, proxy);
        require(address(hook) == hookAddr, "hook mined address mismatch");
        console.log("Hook deployed:", address(hook));

        protocol.configureV4(POOL_MANAGER, address(hook));
        hook.setFeeRecipient(proxy);
        console.log("V4 configured; hook feeRecipient set to proxy.");

        vm.stopBroadcast();

        // ── CUSTODIAN: migrate both existing tickers ──
        vm.startBroadcast(custodianKey);

        _migrate(protocol, idrxAddr, factoryAddr, "BUMIP");
        _migrate(protocol, idrxAddr, factoryAddr, "ENRGP");

        vm.stopBroadcast();

        console.log("V4 cutover complete.");
    }
}
