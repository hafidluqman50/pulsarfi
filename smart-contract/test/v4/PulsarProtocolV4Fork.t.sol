// SPDX-License-Identifier: MIT
pragma solidity ^0.8.26;

import {Test} from "forge-std/Test.sol";
import {ERC1967Proxy} from "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";
import {PulsarProtocol} from "../../src/PulsarProtocol.sol";
import {PulsarProtocolOps} from "../../src/PulsarProtocolOps.sol";
import {PulsarStock} from "../../src/PulsarStock.sol";
import {PulsarSwapHook} from "../../src/v4/PulsarSwapHook.sol";
import {IDRX} from "../../src/mocks/IDRX.sol";

import {IPoolManager} from "@uniswap/v4-core/src/interfaces/IPoolManager.sol";
import {Hooks} from "@uniswap/v4-core/src/libraries/Hooks.sol";
import {FullMath} from "@uniswap/v4-core/src/libraries/FullMath.sol";
import {HookMiner} from "@uniswap/v4-periphery/src/utils/HookMiner.sol";

/// @notice End-to-end fork test of the V4-native live path against the REAL
///         Uniswap V4 PoolManager on Arbitrum Sepolia: multisig mint that creates
///         the V4 pool with the fee hook and seeds full-range liquidity -> retail
///         buy (fee charged in IDRX by the hook) -> collectV4Fees (redeem the
///         hook's ERC-6909 claims into accumulatedFees) -> emergency withdraw
///         (pause the hook + pull liquidity back). Runs only when RPC_URL is set.
///
///         migrateV2ToV4 is exercised only against pre-existing V2 pools (the live
///         proxy's BUMIP/ENRGP); it can't be reconstructed on a fresh protocol
///         because stocks only ever come from executeMint, which is now V4-native.
contract PulsarProtocolV4ForkTest is Test {
    address constant POOL_MANAGER = 0xFB3e0C6F74eB1a21CC1Da29aeC80D2Dfe6C9a317;

    uint256 constant TOKEN_AMOUNT = 1_000 * 1e18;
    uint256 constant IDRX_AMOUNT = 2_500_000; // 25 000.00 IDRX
    uint256 constant SWAP_FEE_BPS = 20; // 0.2%

    PulsarProtocol protocol;
    IDRX idrxToken;
    PulsarSwapHook hook;

    address admin = makeAddr("admin");
    address cust1 = makeAddr("cust1");
    address cust2 = makeAddr("cust2");
    address cust3 = makeAddr("cust3");
    address cust4 = makeAddr("cust4");
    address cust5 = makeAddr("cust5");
    address retail = makeAddr("retail");

    bool forked;
    uint256 idrxId;

    function _sqrt(uint256 x) internal pure returns (uint256 y) {
        if (x == 0) return 0;
        uint256 z = (x + 1) / 2;
        y = x;
        while (z < y) {
            y = z;
            z = (x / z + z) / 2;
        }
    }

    /// Canonical initial price for executeMint: sqrt(IDRX_raw / stock_raw) * 2^96.
    function _canonicalSqrtPriceX96(uint256 idrxAmt, uint256 stockAmt) internal pure returns (uint160) {
        return uint160(_sqrt(FullMath.mulDiv(idrxAmt, uint256(1) << 192, stockAmt)));
    }

    function setUp() public {
        string memory rpc = vm.envOr("RPC_URL", string(""));
        if (bytes(rpc).length == 0) return;
        vm.createSelectFork(rpc);
        forked = true;

        vm.prank(admin);
        idrxToken = new IDRX(admin);
        idrxId = uint256(uint160(address(idrxToken)));

        address[] memory custodians = new address[](5);
        custodians[0] = cust1;
        custodians[1] = cust2;
        custodians[2] = cust3;
        custodians[3] = cust4;
        custodians[4] = cust5;

        // Router is unused on the V4-native path — placeholder address is fine.
        PulsarProtocol impl = new PulsarProtocol();
        ERC1967Proxy proxy = new ERC1967Proxy(
            address(impl),
            abi.encodeCall(PulsarProtocol.initialize, (admin, makeAddr("router"), address(idrxToken), custodians, admin))
        );
        protocol = PulsarProtocol(address(proxy));

        vm.startPrank(admin);
        idrxToken.mint(cust1, 100_000_000);
        idrxToken.mint(retail, 50_000_000);
        vm.stopPrank();

        // Deploy the hook at a mined address; feeConfig = protocol (reads swapFeeBps),
        // pauser = protocol (so emergencyWithdrawV4 can pause the hook).
        uint160 flags = uint160(
            Hooks.BEFORE_SWAP_FLAG | Hooks.AFTER_SWAP_FLAG | Hooks.BEFORE_SWAP_RETURNS_DELTA_FLAG
                | Hooks.AFTER_SWAP_RETURNS_DELTA_FLAG
        );
        bytes memory args =
            abi.encode(POOL_MANAGER, address(protocol), address(idrxToken), admin, address(protocol));
        (address hookAddr, bytes32 salt) =
            HookMiner.find(address(this), flags, type(PulsarSwapHook).creationCode, args);
        hook = new PulsarSwapHook{salt: salt}(
            IPoolManager(POOL_MANAGER), address(protocol), address(idrxToken), admin, address(protocol)
        );
        assertEq(address(hook), hookAddr);

        vm.startPrank(admin);
        protocol.configureV4(POOL_MANAGER, address(hook));
        protocol.setOpsContract(address(new PulsarProtocolOps()));
        protocol.setSwapFeeBps(SWAP_FEE_BPS);
        hook.setFeeRecipient(address(protocol));
        vm.stopPrank();
    }

    /// Full multisig mint that creates the V4 pool and seeds full-range liquidity.
    function _mintV4() internal returns (address stock) {
        vm.prank(cust1);
        uint256 pid =
            protocol.requestMint("BUMIP", "Pulsar Bumi Resources", "BUMI", TOKEN_AMOUNT, IDRX_AMOUNT, keccak256("a"));
        vm.prank(cust2);
        protocol.approveMint(pid);
        vm.prank(cust3);
        protocol.approveMint(pid);

        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        protocol.executeMint(pid, _canonicalSqrtPriceX96(IDRX_AMOUNT, TOKEN_AMOUNT));
        vm.stopPrank();

        stock = protocol.stocks("BUMIP");
    }

    function test_mint_swap_collect_emergency_endToEnd() public {
        if (!forked) return;

        // ── V4-native mint (custodian) creates + seeds the pool ──
        address stock = _mintV4();
        assertTrue(protocol.isV4Migrated("BUMIP"), "ticker must be V4-native after mint");

        // ── Retail buy on V4 (permissionless); hook charges 0.2% in IDRX ──
        uint256 amountIn = 1_000_000; // 10 000.00 IDRX
        uint256 stockBefore = PulsarStock(stock).balanceOf(retail);

        vm.startPrank(retail);
        idrxToken.approve(address(protocol), amountIn);
        protocol.swapV4("BUMIP", amountIn, 0, true);
        vm.stopPrank();

        assertGt(PulsarStock(stock).balanceOf(retail) - stockBefore, 0, "retail must receive pStock");

        uint256 expectedFee = (amountIn * SWAP_FEE_BPS) / 10_000;
        assertEq(
            IPoolManager(POOL_MANAGER).balanceOf(address(hook), idrxId),
            expectedFee,
            "hook must hold exactly the 0.2% IDRX fee claim"
        );

        // ── Collect fees: sweep the hook's claims, redeem to real IDRX, book them ──
        protocol.collectV4Fees();
        assertEq(protocol.accumulatedFees(), expectedFee, "collectV4Fees must book the exact buy-side fee");
        assertEq(IPoolManager(POOL_MANAGER).balanceOf(address(hook), idrxId), 0, "hook claim swept");

        // ── Emergency withdraw (custodian): pause hook + pull liquidity ──
        uint256 protoStockBefore = PulsarStock(stock).balanceOf(address(protocol));
        uint256 protoIdrxBefore = idrxToken.balanceOf(address(protocol));

        vm.prank(cust1);
        protocol.emergencyWithdrawV4("BUMIP");

        assertTrue(hook.paused(), "hook must be paused after emergency");
        assertFalse(protocol.isV4Migrated("BUMIP"), "migration flag cleared");
        assertGt(PulsarStock(stock).balanceOf(address(protocol)) - protoStockBefore, 0, "protocol recovers pStock");
        assertGt(idrxToken.balanceOf(address(protocol)) - protoIdrxBefore, 0, "protocol recovers IDRX");

        // ── Swaps must be halted while paused ──
        vm.startPrank(retail);
        idrxToken.approve(address(protocol), 100_000);
        vm.expectRevert();
        protocol.swapV4("BUMIP", 100_000, 0, true);
        vm.stopPrank();
    }
}
