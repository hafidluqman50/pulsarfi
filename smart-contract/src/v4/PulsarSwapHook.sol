// SPDX-License-Identifier: MIT
pragma solidity ^0.8.26;

import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";
import {Pausable} from "@openzeppelin/contracts/utils/Pausable.sol";

import {BaseHook} from "@openzeppelin/uniswap-hooks/base/BaseHook.sol";
import {BaseHookFee} from "@openzeppelin/uniswap-hooks/fee/BaseHookFee.sol";
import {CurrencySettler} from "@openzeppelin/uniswap-hooks/utils/CurrencySettler.sol";

import {IPoolManager} from "@uniswap/v4-core/src/interfaces/IPoolManager.sol";
import {Hooks} from "@uniswap/v4-core/src/libraries/Hooks.sol";
import {SafeCast} from "@uniswap/v4-core/src/libraries/SafeCast.sol";
import {PoolKey} from "@uniswap/v4-core/src/types/PoolKey.sol";
import {PoolId} from "@uniswap/v4-core/src/types/PoolId.sol";
import {Currency, CurrencyLibrary} from "@uniswap/v4-core/src/types/Currency.sol";
import {BalanceDelta} from "@uniswap/v4-core/src/types/BalanceDelta.sol";
import {BeforeSwapDelta, BeforeSwapDeltaLibrary, toBeforeSwapDelta} from
    "@uniswap/v4-core/src/types/BeforeSwapDelta.sol";
import {SwapParams} from "@uniswap/v4-core/src/types/PoolOperation.sol";

/// @notice Single source of truth for the protocol swap fee rate. PulsarProtocol
///         already exposes `swapFeeBps()` (in basis points), so the hook reads it
///         rather than duplicating the setting.
interface IPulsarFeeConfig {
    function swapFeeBps() external view returns (uint256);
}

/// @title PulsarSwapHook
/// @notice Enforces PulsarFi's protocol swap fee at the Uniswap V4 POOL level, so
///         every swap against a registered pStock/IDRX pool pays it — regardless
///         of entry point (PulsarProtocol, a raw PoolManager.unlock, an aggregator,
///         or Uniswap's own UI). This closes the V2 bypass hole where fees only
///         applied to swaps routed through PulsarProtocol.swap().
///
///         Fee is ALWAYS denominated in IDRX, matching the V2 design and keeping
///         the fee treasury single-currency (strict accounting):
///           - SELL (stock in -> IDRX out): IDRX is the swap OUTPUT (unspecified
///             currency). Handled by the audited BaseHookFee.afterSwap path.
///           - BUY  (IDRX in -> stock out): IDRX is the swap INPUT (specified
///             currency). Handled by the custom beforeSwap below, taking the fee
///             off the input before the core swap math runs.
///
///         Both sides accrue the fee as ERC-6909 IDRX claims held by this hook,
///         moved to `feeRecipient` in batch via handleHookFees (never per-swap).
///
///         DRAFT — compiles and unit/integration tests pending; do NOT deploy
///         before fork integration tests + invariant tests + external audit.
contract PulsarSwapHook is BaseHookFee, AccessControl, Pausable {
    using CurrencySettler for Currency;
    using CurrencyLibrary for Currency;
    using SafeCast for uint256;

    bytes32 public constant PAUSER_ROLE = keccak256("PAUSER_ROLE");

    /// @dev The IDRX currency; the fee is always taken in this token.
    Currency public immutable idrx;

    /// @dev Reads the live `swapFeeBps` so the hook and protocol never drift.
    IPulsarFeeConfig public immutable feeConfig;

    /// @dev Where accrued IDRX fee claims are swept by handleHookFees.
    address public feeRecipient;

    /// @dev Ticker attribution per pool, for fee bookkeeping/events.
    mapping(PoolId => string) public poolTicker;

    /// @dev Cumulative IDRX fee taken (observability only; not authoritative).
    uint256 public totalFeeIdrxTaken;

    event FeeRecipientUpdated(address indexed feeRecipient);
    event PoolRegistered(PoolId indexed poolId, string ticker);
    event BuySideFeeTaken(PoolId indexed poolId, address indexed sender, uint256 feeIdrx);
    event FeesSwept(address indexed to, uint256 amount);

    error ZeroAddress();

    constructor(IPoolManager poolManager_, address feeConfig_, address idrx_, address admin, address pauser)
        BaseHook(poolManager_)
    {
        if (feeConfig_ == address(0) || idrx_ == address(0) || admin == address(0)) revert ZeroAddress();
        feeConfig = IPulsarFeeConfig(feeConfig_);
        idrx = Currency.wrap(idrx_);
        feeRecipient = admin;
        _grantRole(DEFAULT_ADMIN_ROLE, admin);
        _grantRole(PAUSER_ROLE, pauser);
    }

    // ─── Admin ──────────────────────────────────────────────────────────────

    function setFeeRecipient(address feeRecipient_) external onlyRole(DEFAULT_ADMIN_ROLE) {
        if (feeRecipient_ == address(0)) revert ZeroAddress();
        feeRecipient = feeRecipient_;
        emit FeeRecipientUpdated(feeRecipient_);
    }

    function pause() external onlyRole(PAUSER_ROLE) {
        _pause();
    }

    function unpause() external onlyRole(DEFAULT_ADMIN_ROLE) {
        _unpause();
    }

    /// @notice Registers a pool's ticker. Called by the protocol after it
    ///         initializes the V4 pool for that ticker.
    function registerPool(PoolKey calldata key, string calldata ticker) external onlyRole(DEFAULT_ADMIN_ROLE) {
        PoolId poolId = key.toId();
        poolTicker[poolId] = ticker;
        emit PoolRegistered(poolId, ticker);
    }

    // ─── Sell side (IDRX is output) — via audited BaseHookFee.afterSwap ──────

    /// @dev Returns the fee (in hundredths of a bip) ONLY when IDRX is the
    ///      swap OUTPUT (sell side). On buys the output is a stock token, so we
    ///      return 0 here and take the fee in beforeSwap instead — this prevents
    ///      double-charging.
    function _getHookFee(address, PoolKey calldata key, SwapParams calldata params, BalanceDelta, bytes calldata)
        internal
        view
        override
        returns (uint24)
    {
        Currency unspecified = (params.amountSpecified < 0 == params.zeroForOne) ? key.currency1 : key.currency0;
        if (Currency.unwrap(unspecified) != Currency.unwrap(idrx)) return 0; // buy side handled in beforeSwap
        // basis points -> hundredths of a bip (BaseHookFee unit): bps * 100
        return uint24(feeConfig.swapFeeBps() * 100);
    }

    // ─── Buy side (IDRX is input) — custom beforeSwap ────────────────────────

    /// @dev On an exact-input swap whose INPUT currency is IDRX (a buy), take the
    ///      protocol fee off the input in IDRX before the core swap runs. Mirrors
    ///      the OZ BaseAsyncSwap take-on-input pattern, but only skims the fee
    ///      portion (not the whole input). Non-IDRX-input swaps fall through to the
    ///      normal pool swap; the sell side is charged in afterSwap.
    function _beforeSwap(address sender, PoolKey calldata key, SwapParams calldata params, bytes calldata)
        internal
        override
        whenNotPaused
        returns (bytes4, BeforeSwapDelta, uint24)
    {
        // Only exact-input swaps are fee-skimmed on the input side.
        if (params.amountSpecified >= 0) {
            return (this.beforeSwap.selector, BeforeSwapDeltaLibrary.ZERO_DELTA, 0);
        }

        Currency specified = params.zeroForOne ? key.currency0 : key.currency1; // input currency
        if (Currency.unwrap(specified) != Currency.unwrap(idrx)) {
            // sell side (stock in) — fee taken later in afterSwap
            return (this.beforeSwap.selector, BeforeSwapDeltaLibrary.ZERO_DELTA, 0);
        }

        uint256 feeBps = feeConfig.swapFeeBps();
        if (feeBps == 0) {
            return (this.beforeSwap.selector, BeforeSwapDeltaLibrary.ZERO_DELTA, 0);
        }

        uint256 amountIn = uint256(-params.amountSpecified);
        uint256 feeAmount = (amountIn * feeBps) / 10_000;
        if (feeAmount == 0) {
            return (this.beforeSwap.selector, BeforeSwapDeltaLibrary.ZERO_DELTA, 0);
        }

        // Take the fee in IDRX as an ERC-6909 claim held by the hook. Positive
        // specified delta => hook consumed `feeAmount` of the input; the core
        // swap proceeds on (amountIn - feeAmount).
        idrx.take(poolManager, address(this), feeAmount, true);
        totalFeeIdrxTaken += feeAmount;

        emit BuySideFeeTaken(key.toId(), sender, feeAmount);

        return (this.beforeSwap.selector, toBeforeSwapDelta(feeAmount.toInt128(), 0), 0);
    }

    /// @dev Pause guard on the sell-side path too; delegates fee logic to BaseHookFee.
    function _afterSwap(address sender, PoolKey calldata key, SwapParams calldata params, BalanceDelta delta, bytes calldata hookData)
        internal
        override
        whenNotPaused
        returns (bytes4, int128)
    {
        return super._afterSwap(sender, key, params, delta, hookData);
    }

    // ─── Fee sweep ───────────────────────────────────────────────────────────

    /// @notice Moves accrued IDRX fee claims (ERC-6909) to `feeRecipient`. The
    ///         recipient (e.g. PulsarProtocol) redeems and runs the 30/70
    ///         treasury/custodian split. Batch, not per-swap.
    /// @dev    Intentionally NOT gated by whenNotPaused: this only moves already
    ///         collected fees to the trusted `feeRecipient` (never an attacker
    ///         path), so it must stay callable during an emergency pause to
    ///         rescue accrued fees while swaps are halted.
    function handleHookFees(Currency[] memory currencies) public override {
        for (uint256 i = 0; i < currencies.length; i++) {
            Currency currency = currencies[i];
            uint256 balance = poolManager.balanceOf(address(this), currency.toId());
            if (balance == 0) continue;
            poolManager.transfer(feeRecipient, currency.toId(), balance);
            emit FeesSwept(feeRecipient, balance);
        }
    }

    // ─── Permissions ──────────────────────────────────────────────────────────

    /// @dev Enable both beforeSwap (buy-side fee) and afterSwap (sell-side fee),
    ///      each with return-delta so the hook can adjust settlement amounts.
    function getHookPermissions() public pure override returns (Hooks.Permissions memory) {
        return Hooks.Permissions({
            beforeInitialize: false,
            afterInitialize: false,
            beforeAddLiquidity: false,
            afterAddLiquidity: false,
            beforeRemoveLiquidity: false,
            afterRemoveLiquidity: false,
            beforeSwap: true,
            afterSwap: true,
            beforeDonate: false,
            afterDonate: false,
            beforeSwapReturnDelta: true,
            afterSwapReturnDelta: true,
            afterAddLiquidityReturnDelta: false,
            afterRemoveLiquidityReturnDelta: false
        });
    }
}
