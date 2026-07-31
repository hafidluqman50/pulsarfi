// SPDX-License-Identifier: MIT
pragma solidity ^0.8.26;

import {Test} from "forge-std/Test.sol";
import {Vm} from "forge-std/Vm.sol";
import {PulsarProtocol} from "../../src/PulsarProtocol.sol";
import {IDRX} from "../../src/mocks/IDRX.sol";
import {IPoolManager} from "@uniswap/v4-core/src/interfaces/IPoolManager.sol";
import {Currency} from "@uniswap/v4-core/src/types/Currency.sol";

/// @notice Bounded random actions over an already-seeded V4 pool (BUMIP), used by
///         the invariant suite to fuzz sequences of buy/sell/fee-admin/collect/
///         distribute calls. Tracks ghost totals so the invariant contract can
///         assert exact fee conservation across the whole hook -> accumulatedFees
///         -> distributeFees pipeline, not just per-call correctness.
contract PulsarProtocolHandler is Test {
    bytes32 constant BUY_SIDE_FEE_TAKEN_TOPIC = keccak256("BuySideFeeTaken(bytes32,address,uint256)");
    bytes32 constant HOOK_FEE_TOPIC = keccak256("HookFee(bytes32,address,uint128,uint128)");

    PulsarProtocol public protocol;
    IDRX public idrxToken;
    IPoolManager public poolManager;
    address public hook;
    address public admin;
    address public trader;
    string public ticker;

    uint256 public idrxId;

    /// Every raw IDRX unit the hook has ever taken as a protocol fee (buy or sell
    /// side), read directly from its events. Must always equal:
    ///   protocol.accumulatedFees() + ghost_totalDistributedOut + hook's uncollected 6909 claim
    uint256 public ghost_feeAccruedFromEvents;
    /// Total raw IDRX ever paid out by a successful distributeFees() call.
    uint256 public ghost_totalDistributedOut;

    uint256 public callsBuy;
    uint256 public callsSell;
    uint256 public callsSetFee;
    uint256 public callsCollect;
    uint256 public callsDistribute;

    constructor(
        PulsarProtocol protocol_,
        IDRX idrxToken_,
        IPoolManager poolManager_,
        address hook_,
        address admin_,
        address trader_,
        string memory ticker_
    ) {
        protocol = protocol_;
        idrxToken = idrxToken_;
        poolManager = poolManager_;
        hook = hook_;
        admin = admin_;
        trader = trader_;
        ticker = ticker_;
        idrxId = uint256(uint160(address(idrxToken)));
    }

    /// Retail-style buy: bounded to a range that keeps the pool's price from
    /// blowing past V4's sqrt-price limits over many repeated calls.
    function buy(uint256 amountInSeed) external {
        callsBuy++;
        uint256 amountIn = bound(amountInSeed, 10_000, 2_000_000); // 100.00 - 20,000.00 IDRX

        vm.prank(admin);
        idrxToken.mint(trader, amountIn);

        vm.startPrank(trader);
        idrxToken.approve(address(protocol), amountIn);
        vm.recordLogs();
        try protocol.swapV4(ticker, amountIn, 0, true) {
            _accrueFeeFromLogs();
        } catch {}
        vm.stopPrank();
    }

    /// Sell a bounded percentage of the trader's current stock balance.
    function sell(uint256 pctSeed) external {
        callsSell++;
        address stockAddress = protocol.stocks(ticker);
        uint256 balance = _stockBalanceOf(stockAddress, trader);
        if (balance == 0) return;

        uint256 pct = bound(pctSeed, 1, 10_000); // 0.01% - 100% of holdings
        uint256 amountIn = (balance * pct) / 10_000;
        if (amountIn == 0) return;

        vm.startPrank(trader);
        _approveStock(stockAddress, amountIn);
        vm.recordLogs();
        try protocol.swapV4(ticker, amountIn, 0, false) {
            _accrueFeeFromLogs();
        } catch {}
        vm.stopPrank();
    }

    /// Admin adjusts the live protocol swap fee (hook reads it every swap).
    function setFeeBps(uint256 bpsSeed) external {
        callsSetFee++;
        uint256 bps = bound(bpsSeed, 0, 1000); // contract-enforced cap
        vm.prank(admin);
        protocol.setSwapFeeBps(bps);
    }

    /// Permissionless: sweep the hook's accrued IDRX claims into accumulatedFees.
    function collectFees() external {
        callsCollect++;
        try protocol.collectV4Fees() {} catch {}
    }

    /// Permissionless: distribute accumulatedFees 30/70. The full pre-call balance
    /// is always what gets sent out (treasury share + remainder + custodian pool),
    /// so the pre-call read is exactly ghost_totalDistributedOut's increment.
    function distributeFees() external {
        callsDistribute++;
        uint256 before = protocol.accumulatedFees();
        try protocol.distributeFees() {
            ghost_totalDistributedOut += before;
        } catch {}
    }

    function _accrueFeeFromLogs() internal {
        Vm.Log[] memory logs = vm.getRecordedLogs();
        for (uint256 i = 0; i < logs.length; i++) {
            if (logs[i].emitter != hook) continue;
            if (logs[i].topics.length == 0) continue;

            if (logs[i].topics[0] == BUY_SIDE_FEE_TAKEN_TOPIC) {
                uint256 feeIdrx = abi.decode(logs[i].data, (uint256));
                ghost_feeAccruedFromEvents += feeIdrx;
            } else if (logs[i].topics[0] == HOOK_FEE_TOPIC) {
                (uint128 fee0, uint128 fee1) = abi.decode(logs[i].data, (uint128, uint128));
                ghost_feeAccruedFromEvents += uint256(fee0) + uint256(fee1);
            }
        }
    }

    function _stockBalanceOf(address stockAddress, address who) internal view returns (uint256 balance) {
        (bool ok, bytes memory data) = stockAddress.staticcall(abi.encodeWithSignature("balanceOf(address)", who));
        require(ok, "balanceOf failed");
        balance = abi.decode(data, (uint256));
    }

    function _approveStock(address stockAddress, uint256 amount) internal {
        (bool ok,) = stockAddress.call(abi.encodeWithSignature("approve(address,uint256)", address(protocol), amount));
        require(ok, "approve failed");
    }

    function hookIdrxClaim() external view returns (uint256) {
        return poolManager.balanceOf(hook, idrxId);
    }
}
