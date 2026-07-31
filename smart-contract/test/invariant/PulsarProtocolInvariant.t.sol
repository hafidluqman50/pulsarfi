// SPDX-License-Identifier: MIT
pragma solidity ^0.8.26;

import {Test, console} from "forge-std/Test.sol";
import {ERC1967Proxy} from "@openzeppelin/contracts/proxy/ERC1967/ERC1967Proxy.sol";
import {PulsarProtocol} from "../../src/PulsarProtocol.sol";
import {PulsarProtocolOps} from "../../src/PulsarProtocolOps.sol";
import {PulsarSwapHook} from "../../src/v4/PulsarSwapHook.sol";
import {IDRX} from "../../src/mocks/IDRX.sol";
import {PulsarProtocolHandler} from "./PulsarProtocolHandler.sol";

import {PoolManager} from "@uniswap/v4-core/src/PoolManager.sol";
import {IPoolManager} from "@uniswap/v4-core/src/interfaces/IPoolManager.sol";
import {Hooks} from "@uniswap/v4-core/src/libraries/Hooks.sol";
import {FullMath} from "@uniswap/v4-core/src/libraries/FullMath.sol";
import {HookMiner} from "@uniswap/v4-periphery/src/utils/HookMiner.sol";

/// @notice Stateful fuzzing over the V4 fee pipeline this cutover introduced:
///         hook accrual (ERC-6909 claim) -> collectV4Fees (redeem to real IDRX)
///         -> accumulatedFees -> distributeFees (30/70 payout). Random sequences
///         of buy/sell/setFee/collect/distribute must never let the protocol's
///         IDRX solvency or the fee-conservation identity break.
contract PulsarProtocolInvariantTest is Test {
    PulsarProtocol protocol;
    IDRX idrxToken;
    PoolManager poolManager;
    PulsarSwapHook hook;
    PulsarProtocolHandler handler;

    address admin = makeAddr("invAdmin");
    address cust1 = makeAddr("invCust1");
    address cust2 = makeAddr("invCust2");
    address cust3 = makeAddr("invCust3");
    address cust4 = makeAddr("invCust4");
    address cust5 = makeAddr("invCust5");
    address trader = makeAddr("invTrader");

    uint256 constant TOKEN_AMOUNT = 1_000 * 1e18;
    uint256 constant IDRX_AMOUNT = 2_500_000;

    function _sqrt(uint256 x) internal pure returns (uint256 y) {
        if (x == 0) return 0;
        uint256 z = (x + 1) / 2;
        y = x;
        while (z < y) {
            y = z;
            z = (x / z + z) / 2;
        }
    }

    function _canonicalSqrtPriceX96(uint256 idrxAmt, uint256 stockAmt) internal pure returns (uint160) {
        return uint160(_sqrt(FullMath.mulDiv(idrxAmt, uint256(1) << 192, stockAmt)));
    }

    function setUp() public {
        vm.prank(admin);
        idrxToken = new IDRX(admin);

        poolManager = new PoolManager(admin);

        address[] memory custodians = new address[](5);
        custodians[0] = cust1;
        custodians[1] = cust2;
        custodians[2] = cust3;
        custodians[3] = cust4;
        custodians[4] = cust5;

        PulsarProtocol impl = new PulsarProtocol();
        ERC1967Proxy proxy = new ERC1967Proxy(
            address(impl),
            abi.encodeCall(
                PulsarProtocol.initialize, (admin, makeAddr("invRouter"), address(idrxToken), custodians, admin)
            )
        );
        protocol = PulsarProtocol(address(proxy));

        uint160 flags = uint160(
            Hooks.BEFORE_SWAP_FLAG | Hooks.AFTER_SWAP_FLAG | Hooks.BEFORE_SWAP_RETURNS_DELTA_FLAG
                | Hooks.AFTER_SWAP_RETURNS_DELTA_FLAG
        );
        bytes memory args =
            abi.encode(IPoolManager(address(poolManager)), address(protocol), address(idrxToken), admin, address(protocol));
        (address hookAddr, bytes32 salt) =
            HookMiner.find(address(this), flags, type(PulsarSwapHook).creationCode, args);
        hook = new PulsarSwapHook{salt: salt}(
            IPoolManager(address(poolManager)), address(protocol), address(idrxToken), admin, address(protocol)
        );
        require(address(hook) == hookAddr, "hook mine mismatch");

        vm.startPrank(admin);
        protocol.configureV4(address(poolManager), address(hook));
        hook.setFeeRecipient(address(protocol));
        protocol.setOpsContract(address(new PulsarProtocolOps()));
        idrxToken.mint(cust1, 100_000_000);
        vm.stopPrank();

        // Seed BUMIP via the multisig mint flow so a V4 pool exists before fuzzing.
        vm.prank(cust1);
        uint256 pid = protocol.requestMint("BUMIP", "Pulsar Bumi Resources", "BUMI", TOKEN_AMOUNT, IDRX_AMOUNT, keccak256("inv"));
        vm.prank(cust2);
        protocol.approveMint(pid);
        vm.prank(cust3);
        protocol.approveMint(pid);
        vm.startPrank(cust1);
        idrxToken.approve(address(protocol), IDRX_AMOUNT);
        protocol.executeMint(pid, _canonicalSqrtPriceX96(IDRX_AMOUNT, TOKEN_AMOUNT));
        vm.stopPrank();

        vm.prank(admin);
        protocol.setSwapFeeBps(20); // 0.2%, non-zero so fees actually accrue under fuzzing

        handler = new PulsarProtocolHandler(protocol, idrxToken, IPoolManager(address(poolManager)), address(hook), admin, trader, "BUMIP");

        bytes4[] memory selectors = new bytes4[](5);
        selectors[0] = PulsarProtocolHandler.buy.selector;
        selectors[1] = PulsarProtocolHandler.sell.selector;
        selectors[2] = PulsarProtocolHandler.setFeeBps.selector;
        selectors[3] = PulsarProtocolHandler.collectFees.selector;
        selectors[4] = PulsarProtocolHandler.distributeFees.selector;
        targetSelector(FuzzSelector({addr: address(handler), selectors: selectors}));
        targetContract(address(handler));
    }

    /// The protocol must always hold enough IDRX to cover its own fee accounting.
    /// A break here would mean distributeFees or collectV4Fees could ever revert
    /// insolvently, or that fee bookkeeping drifted ahead of real balances.
    function invariant_solvency() public view {
        assertGe(
            idrxToken.balanceOf(address(protocol)),
            protocol.accumulatedFees(),
            "protocol IDRX balance must always cover accumulatedFees"
        );
    }

    /// Every raw IDRX unit the hook has ever taken as a fee must be accounted for
    /// exactly once: still uncollected in the hook (ERC-6909 claim), collected but
    /// undistributed (accumulatedFees), or already paid out (distributeFees).
    function invariant_feeConservation() public view {
        assertEq(
            handler.ghost_feeAccruedFromEvents(),
            protocol.accumulatedFees() + handler.ghost_totalDistributedOut() + handler.hookIdrxClaim(),
            "every hook fee unit must be uncollected, in accumulatedFees, or already distributed"
        );
    }

    function invariant_callSummary() public view {
        console.log("buy calls:", handler.callsBuy());
        console.log("sell calls:", handler.callsSell());
        console.log("setFee calls:", handler.callsSetFee());
        console.log("collect calls:", handler.callsCollect());
        console.log("distribute calls:", handler.callsDistribute());
    }
}
