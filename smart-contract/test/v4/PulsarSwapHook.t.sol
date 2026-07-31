// SPDX-License-Identifier: MIT
pragma solidity ^0.8.26;

import {Test} from "forge-std/Test.sol";
import {PoolManager} from "@uniswap/v4-core/src/PoolManager.sol";
import {Hooks} from "@uniswap/v4-core/src/libraries/Hooks.sol";
import {HookMiner} from "@uniswap/v4-periphery/src/utils/HookMiner.sol";
import {PulsarSwapHook} from "../../src/v4/PulsarSwapHook.sol";

/// @notice Minimal stand-in exposing swapFeeBps() so the hook can be deployed
///         in isolation without the full PulsarProtocol.
contract MockFeeConfig {
    uint256 public swapFeeBps = 20;
}

/// @notice Proves the hook-address-mining step works end to end: find a salt,
///         deploy via CREATE2 at the mined address, and confirm the deployed
///         hook's permission flags match what BaseHook validated in-constructor.
contract PulsarSwapHookMiningTest is Test {
    PoolManager poolManager;
    MockFeeConfig feeConfig;
    address idrx = makeAddr("idrx");
    address admin = makeAddr("admin");
    address pauser = makeAddr("pauser");

    function test_mineAndDeployHookAddress() public {
        poolManager = new PoolManager(address(this));
        feeConfig = new MockFeeConfig();

        uint160 flags = uint160(
            Hooks.BEFORE_SWAP_FLAG | Hooks.AFTER_SWAP_FLAG | Hooks.BEFORE_SWAP_RETURNS_DELTA_FLAG
                | Hooks.AFTER_SWAP_RETURNS_DELTA_FLAG
        );

        bytes memory constructorArgs = abi.encode(address(poolManager), address(feeConfig), idrx, admin, pauser);

        (address minedAddress, bytes32 salt) =
            HookMiner.find(address(this), flags, type(PulsarSwapHook).creationCode, constructorArgs);

        PulsarSwapHook hook =
            new PulsarSwapHook{salt: salt}(poolManager, address(feeConfig), idrx, admin, pauser);

        assertEq(address(hook), minedAddress, "deployed address must match the mined address");
        assertEq(
            uint160(address(hook)) & Hooks.ALL_HOOK_MASK,
            flags,
            "deployed address must carry the exact permission flags"
        );

        // If the mined address didn't match getHookPermissions(), the BaseHook
        // constructor would have reverted before reaching here.
        assertEq(address(hook.feeConfig()), address(feeConfig));
        assertEq(hook.feeRecipient(), admin);
    }
}
