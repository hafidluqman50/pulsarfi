// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {SafeERC20} from "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import {PulsarStock} from "./PulsarStock.sol";
import {IUniswapV2Router02} from "./interfaces/IUniswapV2Router02.sol";
import {PulsarProtocolStorage} from "./PulsarProtocolStorage.sol";
import {IPoolManager} from "@uniswap/v4-core/src/interfaces/IPoolManager.sol";
import {IUnlockCallback} from "@uniswap/v4-core/src/interfaces/callback/IUnlockCallback.sol";
import {IHooks} from "@uniswap/v4-core/src/interfaces/IHooks.sol";
import {PoolKey} from "@uniswap/v4-core/src/types/PoolKey.sol";
import {PoolIdLibrary} from "@uniswap/v4-core/src/types/PoolId.sol";
import {Currency, CurrencyLibrary} from "@uniswap/v4-core/src/types/Currency.sol";
import {BalanceDelta} from "@uniswap/v4-core/src/types/BalanceDelta.sol";
import {SwapParams, ModifyLiquidityParams} from "@uniswap/v4-core/src/types/PoolOperation.sol";
import {StateLibrary} from "@uniswap/v4-core/src/libraries/StateLibrary.sol";
import {TickMath} from "@uniswap/v4-core/src/libraries/TickMath.sol";
import {FullMath} from "@uniswap/v4-core/src/libraries/FullMath.sol";
import {FixedPoint96} from "@uniswap/v4-core/src/libraries/FixedPoint96.sol";
import {CurrencySettler} from "@openzeppelin/uniswap-hooks/utils/CurrencySettler.sol";
import {LiquidityAmounts} from "@uniswap/v4-periphery/src/libraries/LiquidityAmounts.sol";

interface IPulsarSwapHookControl {
    function pause() external;
    function unpause() external;
    function registerPool(PoolKey calldata key, string calldata ticker) external;
    function handleHookFees(Currency[] calldata currencies) external;
}

error StockNotFound(string ticker);
error KYCRequired(address wallet);
error ProposalNotFound(uint256 proposalId);
error AlreadyApproved(uint256 proposalId, address custodian);
error AlreadyRejectedMint(uint256 proposalId, address custodian);
error ProposalAlreadyExecuted(uint256 proposalId);
error MintRequestPending(string ticker);
error NotRequester(uint256 proposalId, address caller);
error NotMintRejectInitiator(uint256 proposalId, address caller);
error ThresholdNotMet(uint256 proposalId, uint8 current, uint8 required);
error RedeemRequestNotFound(uint256 requestId);
error RedeemRequestAlreadyProcessed(uint256 requestId);
error RedeemAlreadyApproved(uint256 requestId, address custodian);
error RedeemAlreadyRejected(uint256 requestId, address custodian);
error RedeemThresholdNotMet(uint256 requestId, uint8 current, uint8 required);
error NotRedeemInitiator(uint256 requestId, address caller);
error InvalidAddress();
error InvalidAmount();
error BelowDistributionThreshold(uint256 balance, uint256 threshold);
error NoActiveCustodians();
error V4NotConfigured();
error V4PoolExists(string ticker);
error V4PoolNotFound(string ticker);
error NotPoolManager();
error SlippageExceeded(uint256 amountOut, uint256 minOut);
error OpsNotConfigured();
error OpsDelegateFailed();

/// @notice Single entry point for all PulsarFi protocol operations.
///         UUPS upgradeable to support future Uniswap V4 migration.
///
///         Size-cutover split (this session): several less-hot-path functions
///         (see the "Delegated to PulsarProtocolOps" section) are thin
///         dispatchers that forward their entire calldata, unmodified, via
///         delegatecall to a separately deployed PulsarProtocolOps contract —
///         same storage (shared base, see PulsarProtocolStorage), same
///         msg.sender, same access control, just physically relocated bytecode.
///         This exists solely because PulsarProtocol's own compiled bytecode hit
///         the EIP-170 24,576-byte deploy limit. No capability was removed; the
///         only real-world change is one extra DELEGATECALL of gas overhead on
///         each delegated function call.
contract PulsarProtocol is PulsarProtocolStorage, IUnlockCallback {
    using SafeERC20 for IERC20;
    using StateLibrary for IPoolManager;
    using CurrencySettler for Currency;
    using CurrencyLibrary for Currency;
    using PoolIdLibrary for PoolKey;

    event StockDeployed(string indexed ticker, address contractAddress);
    event TokensMinted(string indexed ticker, address indexed to, uint256 amount, bytes32 attestationHash);
    event TokensRedeemed(string indexed ticker, address indexed from, uint256 amount);
    event KYCApproved(address indexed wallet);
    event KYCRevoked(address indexed wallet);
    event MintRequested(uint256 indexed proposalId, address indexed requester, string ticker);
    event MintApproved(uint256 indexed proposalId, address indexed approver, uint8 approvalCount);
    event MintExecuted(uint256 indexed proposalId);
    event MintRejectionVoted(uint256 indexed proposalId, address indexed custodian, uint8 rejectCount);
    event MintRejected(uint256 indexed proposalId, address indexed rejectInitiator);
    event RedeemRequested(
        uint256 indexed requestId, address indexed user, string ticker, uint256 tokenAmount, uint256 feeIdrx
    );
    event RedeemApproved(uint256 indexed requestId, address indexed custodian, uint8 approvalCount);
    event RedeemRejectionVoted(uint256 indexed requestId, address indexed custodian, uint8 rejectCount);
    event RedeemExecuted(uint256 indexed requestId, address indexed initiator);
    event RedeemRejected(uint256 indexed requestId, address indexed initiator);
    event TreasuryUpdated(address indexed treasury);
    event RedeemFeeBpsUpdated(uint256 feeBps);
    event SwapFeeBpsUpdated(uint256 feeBps);
    event MinimumDistributionThresholdUpdated(uint256 threshold);
    event FeesDistributed(uint256 treasuryAmount, uint256 custodianAmount, uint256 recipientCount);
    event RouterUpdated(address indexed router);
    event IDRXUpdated(address indexed idrx);
    event OpsContractUpdated(address indexed opsContract);

    /// @custom:oz-upgrades-unsafe-allow constructor
    constructor() {
        _disableInitializers();
    }

    function initialize(address admin, address router_, address idrx_, address[] calldata custodians, address treasury_)
        external
        initializer
    {
        _grantRole(DEFAULT_ADMIN_ROLE, admin);
        for (uint256 i = 0; i < custodians.length; i++) {
            _grantRole(CUSTODIAN_ROLE, custodians[i]);
        }
        router = IUniswapV2Router02(router_);
        idrx = idrx_;
        treasury = treasury_;
    }

    // ─── Delegated to PulsarProtocolOps ─────────────────────────────────────
    // Each forwards msg.data as-is (no re-encoding) via delegatecall, so the ops
    // contract's identical function signature + real access-control checks are
    // what actually runs — against this proxy's own storage/balances. See the
    // contract-level docs above for why this split exists.

    /// @notice Deprecated: retained for storage layout compatibility only.
    ///         Do not call — IDRX is now pulled from the requester at executeMint.
    function fundMintLiquidity(uint256 proposalId, uint256 amount) external {
        _delegateToOps();
    }

    function approveMint(uint256 proposalId) external {
        _delegateToOps();
    }

    /// @notice Custodian votes to reject a mint proposal.
    function rejectMint(uint256 proposalId) external {
        _delegateToOps();
    }

    /// @notice rejectInitiator closes the proposal after 3/5 rejection votes.
    ///         Refunds any IDRX already locked via fundMintLiquidity (legacy pre-upgrade proposals).
    function executeRejectMint(uint256 proposalId) external {
        _delegateToOps();
    }

    function approveRedeem(uint256 requestId) external {
        _delegateToOps();
    }

    function rejectRedeem(uint256 requestId) external {
        _delegateToOps();
    }

    function executeReject(uint256 requestId) external {
        _delegateToOps();
    }

    /// @notice Adds full-range liquidity to the V4 pool from the caller's funds.
    ///         Once seeded, swaps for this ticker route to V4.
    function addV4Liquidity(string calldata ticker, uint256 idrxAmount, uint256 stockAmount) external {
        _delegateToOps();
    }

    /// @notice Migrates a ticker's liquidity from its V2 pool to a fresh V4 pool
    ///         carrying the fee hook. Removes all protocol-owned V2 liquidity,
    ///         creates the V4 pool at `sqrtPriceX96` (pass the current V2 price to
    ///         avoid an arb gap), and re-seeds it with the recovered tokens.
    ///         `sqrtPriceX96`/`tickSpacing`/`lpFee` are used only if the V4 pool
    ///         does not exist yet.
    function migrateV2ToV4(
        string calldata ticker,
        uint160 sqrtPriceX96,
        int24 tickSpacing,
        uint24 lpFee,
        uint256 minIdrxOut,
        uint256 minStockOut
    ) external {
        _delegateToOps();
    }

    /// @notice Escape hatch: pause the hook (halts all swaps on its pools) and
    ///         pull the protocol's entire V4 liquidity for `ticker` back into the
    ///         protocol. Callable during an incident; not blocked by the
    ///         protocol pause. Requires the protocol to hold PAUSER_ROLE on the hook.
    function emergencyWithdrawV4(string calldata ticker) external {
        _delegateToOps();
    }

    /// @notice Permissionless: anyone can trigger distribution once accumulatedFees
    ///         reaches minimumDistributionThreshold. 30% to treasury, 70% split
    ///         equally among custodians that have ever approved/requested a mint.
    function distributeFees() external {
        _delegateToOps();
    }

    /// @dev Forwards the entire original calldata to opsContract via delegatecall
    ///      (same storage, same msg.sender, same balances) and relays its return
    ///      data or revert reason verbatim.
    function _delegateToOps() internal {
        address ops = opsContract;
        if (ops == address(0)) revert OpsNotConfigured();
        (bool ok, bytes memory ret) = ops.delegatecall(msg.data);
        if (!ok) {
            assembly {
                revert(add(ret, 32), mload(ret))
            }
        }
        assembly {
            return(add(ret, 32), mload(ret))
        }
    }

    function setOpsContract(address opsContract_) external onlyRole(DEFAULT_ADMIN_ROLE) {
        if (opsContract_ == address(0)) revert InvalidAddress();
        opsContract = opsContract_;
        emit OpsContractUpdated(opsContract_);
    }

    // ─── Multisig Mint ────────────────────────────────────────────────────────

    /// @notice Custodian submits a mint proposal. All mints go to the liquidity pool.
    ///         IDRX is pulled from the requester's wallet at executeMint time.
    function requestMint(
        string calldata ticker,
        string calldata stockName,
        string calldata idxTicker,
        uint256 tokenAmount,
        uint256 idrxAmount,
        bytes32 attestationHash
    ) external onlyRole(CUSTODIAN_ROLE) returns (uint256 proposalId) {
        if (hasPendingRequest[ticker]) revert MintRequestPending(ticker);

        proposalId = proposalCount++;
        hasPendingRequest[ticker] = true;

        MintProposal storage proposal = proposals[proposalId];
        proposal.ticker = ticker;
        proposal.stockName = stockName;
        proposal.idxTicker = idxTicker;
        proposal.tokenAmount = tokenAmount;
        proposal.idrxAmount = idrxAmount;
        proposal.attestationHash = attestationHash;
        proposal.requester = msg.sender;
        proposal.approvalCount = 1;

        hasApproved[proposalId][msg.sender] = true;
        _markActiveCustodian(msg.sender);

        emit MintRequested(proposalId, msg.sender, ticker);
        emit MintApproved(proposalId, msg.sender, 1);
    }

    /// @notice Requester executes after 3/5 approvals. Mints the stock to the
    ///         protocol, pulls idrxAmount from msg.sender, and provides both as
    ///         full-range Uniswap V4 liquidity (creating the pool on the first mint
    ///         for a ticker). Caller must have approved this contract for idrxAmount
    ///         IDRX beforehand.
    /// @param sqrtPriceX96 Canonical initial price for a first mint, encoded as
    ///        `sqrt(IDRX_raw per 1 stock_raw) * 2^96` — orientation-independent, so
    ///        the caller does not need the (lazily deployed) stock address. The
    ///        contract flips it to the pool's currency ordering on-chain. Ignored
    ///        once the ticker's V4 pool already exists.
    function executeMint(uint256 proposalId, uint160 sqrtPriceX96)
        external
        onlyRole(CUSTODIAN_ROLE)
        whenNotPaused
    {
        MintProposal storage proposal = proposals[proposalId];

        if (proposal.approvalCount == 0 && !proposal.executed) revert ProposalNotFound(proposalId);
        if (proposal.executed) revert ProposalAlreadyExecuted(proposalId);
        if (proposal.requester != msg.sender) revert NotRequester(proposalId, msg.sender);
        if (proposal.approvalCount < THRESHOLD) {
            revert ThresholdNotMet(proposalId, proposal.approvalCount, THRESHOLD);
        }

        proposal.executed = true;
        hasPendingRequest[proposal.ticker] = false;

        address stockAddress = _ensureStock(proposal.ticker, proposal.stockName, proposal.idxTicker);
        _mint(stockAddress, proposal.ticker, address(this), proposal.tokenAmount, proposal.attestationHash);

        // Pull IDRX from the requester. Any IDRX already funded via the legacy
        // fundMintLiquidity path (pre-upgrade proposals only) is used first.
        uint256 alreadyFunded = mintLiquidityFunding[proposalId];
        if (alreadyFunded < proposal.idrxAmount) {
            IERC20(idrx).safeTransferFrom(msg.sender, address(this), proposal.idrxAmount - alreadyFunded);
        }
        mintLiquidityFunding[proposalId] = 0;

        _provideMintToV4(proposal.ticker, proposal.tokenAmount, proposal.idrxAmount, sqrtPriceX96);
        emit MintExecuted(proposalId);
    }

    // ─── Redeem Request ───────────────────────────────────────────────────────

    function requestRedeem(string calldata ticker, uint256 tokenAmount) external {
        address user = msg.sender;
        if (!kycApproved[user]) revert KYCRequired(user);
        address stockAddress = _requireStock(ticker);

        uint256 feeIdrx = 0;
        if (redeemFeeBps > 0) {
            uint256 idrxValue = _quoteStockToIdrxV4(ticker, tokenAmount);
            feeIdrx = (idrxValue * redeemFeeBps) / 10_000;
        }

        IERC20(stockAddress).safeTransferFrom(user, address(this), tokenAmount);

        if (feeIdrx > 0) {
            IERC20(idrx).safeTransferFrom(user, address(this), feeIdrx);
        }

        uint256 requestId = redeemRequestCount++;
        RedeemRequest storage req = redeemRequests[requestId];
        req.ticker = ticker;
        req.user = user;
        req.tokenAmount = tokenAmount;
        req.feeIdrx = feeIdrx;

        emit RedeemRequested(requestId, user, ticker, tokenAmount, feeIdrx);
    }

    /// @dev Delegated to PulsarProtocolOps — used once per completed redeem,
    ///      cold enough relative to swapV4/executeMint to relocate.
    function executeRedeem(uint256 requestId) external {
        _delegateToOps();
    }

    // ─── Uniswap V4 ─────────────────────────────────────────────────────────
    //
    // V4 pools carry PulsarSwapHook, which enforces the protocol swap fee at the
    // pool level (in IDRX both directions). The protocol is the sole LP so it can
    // unilaterally withdraw liquidity for the emergency escape plan. Liquidity and
    // swaps go through PoolManager.unlock -> unlockCallback (flash accounting).

    enum V4Action {
        ADD_LIQUIDITY,
        SWAP,
        REMOVE_ALL_LIQUIDITY,
        COLLECT_FEES
    }

    struct V4CallbackData {
        V4Action action;
        string ticker;
        uint256 amountA; // ADD: idrx amount   | SWAP: amountIn
        uint256 amountB; // ADD: stock amount  | SWAP: minOut
        bool buyStock; // SWAP only
        address user; // SWAP output recipient
    }

    event V4Configured(address indexed poolManager, address indexed swapHook);
    event V4PoolCreated(string indexed ticker, uint160 sqrtPriceX96);
    event V4LiquidityAdded(string indexed ticker, uint256 idrxAmount, uint256 stockAmount);
    event V4Swapped(string indexed ticker, address indexed user, bool buyStock, uint256 amountIn, uint256 amountOut);
    event V4EmergencyWithdrawn(string indexed ticker, uint256 idrxRecovered, uint256 stockRecovered);
    event V4FeesCollected(uint256 idrxAmount);

    function configureV4(address poolManager_, address swapHook_) external onlyRole(DEFAULT_ADMIN_ROLE) {
        if (poolManager_ == address(0) || swapHook_ == address(0)) revert InvalidAddress();
        poolManager = IPoolManager(poolManager_);
        swapHook = swapHook_;
        emit V4Configured(poolManager_, swapHook_);
    }

    /// @dev Initializes the V4 pool for `ticker` and registers it with the hook.
    ///      Stock must already be deployed (via an earlier executeMint).
    function _createV4Pool(string memory ticker, uint160 sqrtPriceX96, int24 tickSpacing, uint24 lpFee) internal {
        if (address(poolManager) == address(0) || swapHook == address(0)) revert V4NotConfigured();
        address stockAddress = stocks[ticker];
        if (stockAddress == address(0)) revert StockNotFound(ticker);
        if (address(poolKeys[ticker].hooks) != address(0)) revert V4PoolExists(ticker);

        (Currency c0, Currency c1) = _sortCurrencies(idrx, stockAddress);
        PoolKey memory key =
            PoolKey({currency0: c0, currency1: c1, fee: lpFee, tickSpacing: tickSpacing, hooks: IHooks(swapHook)});
        // CEI: write state and emit before the external calls.
        poolKeys[ticker] = key;
        emit V4PoolCreated(ticker, sqrtPriceX96);
        poolManager.initialize(key, sqrtPriceX96);
        IPulsarSwapHookControl(swapHook).registerPool(key, ticker);
    }

    /// @dev Seeds V4 liquidity from tokens the protocol already holds (no pull).
    ///      Used by executeMint's first-mint path, and by PulsarProtocolOps
    ///      (addV4Liquidity / migrateV2ToV4) via delegatecall.
    function _provideV4Liquidity(string memory ticker, uint256 idrxAmount, uint256 stockAmount) internal {
        _requireV4PoolMem(ticker);
        poolManager.unlock(
            abi.encode(
                V4CallbackData({
                    action: V4Action.ADD_LIQUIDITY,
                    ticker: ticker,
                    amountA: idrxAmount,
                    amountB: stockAmount,
                    buyStock: false,
                    user: address(this)
                })
            )
        );
        isV4Migrated[ticker] = true;
        emit V4LiquidityAdded(ticker, idrxAmount, stockAmount);
    }

    /// @dev Mint-time V4 provisioning. Creates the pool on the first mint for a
    ///      ticker (at the caller's canonical price, oriented to the pool's currency
    ///      ordering) and seeds full-range liquidity from tokens the protocol just
    ///      minted/pulled. `canonicalSqrtPriceX96` is `sqrt(IDRX_raw per stock_raw)
    ///      * 2^96`; it is ignored once the pool exists.
    function _provideMintToV4(
        string memory ticker,
        uint256 tokenAmount,
        uint256 idrxAmount,
        uint160 canonicalSqrtPriceX96
    ) internal {
        if (address(poolManager) == address(0) || swapHook == address(0)) revert V4NotConfigured();
        if (address(poolKeys[ticker].hooks) == address(0)) {
            // The pool price is currency1/currency0 in raw units. Canonical price is
            // IDRX/stock, so it matches directly when stock is currency0 and must be
            // inverted (2^192 / s) when IDRX is currency0. No on-chain sqrt.
            bool idrxIs0 = idrx < stocks[ticker];
            uint160 poolSqrtPriceX96 = idrxIs0
                ? uint160(FullMath.mulDiv(FixedPoint96.Q96, FixedPoint96.Q96, canonicalSqrtPriceX96))
                : canonicalSqrtPriceX96;
            _createV4Pool(ticker, poolSqrtPriceX96, MINT_TICK_SPACING, MINT_LP_FEE);
        }
        _provideV4Liquidity(ticker, idrxAmount, tokenAmount);
    }

    /// @notice Permissionless swap through the V4 pool. The hook charges the
    ///         protocol fee automatically; no fee logic here.
    function swapV4(string calldata ticker, uint256 amountIn, uint256 minOut, bool buyStock) external whenNotPaused {
        _requireV4Pool(ticker);
        if (amountIn == 0) revert InvalidAmount();

        address tokenIn = buyStock ? idrx : stocks[ticker];
        IERC20(tokenIn).safeTransferFrom(msg.sender, address(this), amountIn);

        poolManager.unlock(
            abi.encode(
                V4CallbackData({
                    action: V4Action.SWAP,
                    ticker: ticker,
                    amountA: amountIn,
                    amountB: minOut,
                    buyStock: buyStock,
                    user: msg.sender
                })
            )
        );
    }

    /// @notice Permissionless. Sweeps the hook's accrued IDRX swap-fee claims
    ///         (ERC-6909) to this protocol, redeems them for real IDRX, and books
    ///         them into `accumulatedFees` so `distributeFees` can split 30/70.
    ///         Callable by anyone (fees only ever move to the protocol/treasury).
    function collectV4Fees() external {
        Currency[] memory currencies = new Currency[](1);
        currencies[0] = Currency.wrap(idrx);
        IPulsarSwapHookControl(swapHook).handleHookFees(currencies);

        uint256 claim = poolManager.balanceOf(address(this), Currency.wrap(idrx).toId());
        if (claim == 0) return;

        uint256 balanceBefore = IERC20(idrx).balanceOf(address(this));
        poolManager.unlock(
            abi.encode(
                V4CallbackData({
                    action: V4Action.COLLECT_FEES,
                    ticker: "",
                    amountA: 0,
                    amountB: 0,
                    buyStock: false,
                    user: address(this)
                })
            )
        );
        uint256 collected = IERC20(idrx).balanceOf(address(this)) - balanceBefore;
        if (collected > 0) {
            accumulatedFees += collected;
            emit V4FeesCollected(collected);
        }
    }

    /// @dev Callback target for poolManager.unlock(). PoolManager routes this to
    ///      `address(this)` (this proxy) regardless of whether the code that
    ///      called unlock() was executing directly here or via a delegatecall
    ///      from PulsarProtocolOps — so this dispatch is shared/reused for free
    ///      by the ops contract's addV4Liquidity/migrateV2ToV4/emergencyWithdrawV4.
    function unlockCallback(bytes calldata data) external returns (bytes memory) {
        if (msg.sender != address(poolManager)) revert NotPoolManager();
        V4CallbackData memory d = abi.decode(data, (V4CallbackData));
        PoolKey memory key = poolKeys[d.ticker];
        if (d.action == V4Action.ADD_LIQUIDITY) {
            _v4AddLiquidity(key, d);
        } else if (d.action == V4Action.SWAP) {
            _v4Swap(key, d);
        } else if (d.action == V4Action.REMOVE_ALL_LIQUIDITY) {
            _v4RemoveAllLiquidity(key);
        } else {
            _v4CollectFees();
        }
        return "";
    }

    /// @dev Removes the protocol's entire full-range position and takes both
    ///      tokens back to the protocol. Used by emergencyWithdrawV4 (via ops).
    function _v4RemoveAllLiquidity(PoolKey memory key) internal {
        int24 tickLower = (TickMath.MIN_TICK / key.tickSpacing) * key.tickSpacing;
        int24 tickUpper = (TickMath.MAX_TICK / key.tickSpacing) * key.tickSpacing;

        bytes32 positionId =
            keccak256(abi.encodePacked(address(this), tickLower, tickUpper, bytes32(0)));
        uint128 liquidity = poolManager.getPositionLiquidity(key.toId(), positionId);
        if (liquidity == 0) return;

        (BalanceDelta delta,) = poolManager.modifyLiquidity(
            key,
            ModifyLiquidityParams({
                tickLower: tickLower,
                tickUpper: tickUpper,
                liquidityDelta: -int256(uint256(liquidity)),
                salt: bytes32(0)
            }),
            ""
        );
        // Removing liquidity yields positive deltas (owed to us) — take to protocol.
        _settleDelta(key, delta.amount0(), delta.amount1(), address(this));
    }

    /// @dev Redeems the protocol's IDRX ERC-6909 claims for real IDRX. `take`
    ///      pulls the real tokens (negative delta); burning the claims via
    ///      `settle(burn=true)` pays that debt, netting the unlock to zero.
    function _v4CollectFees() internal {
        Currency idrxCurrency = Currency.wrap(idrx);
        uint256 claim = poolManager.balanceOf(address(this), idrxCurrency.toId());
        if (claim == 0) return;
        idrxCurrency.take(poolManager, address(this), claim, false);
        idrxCurrency.settle(poolManager, address(this), claim, true);
    }

    function _v4AddLiquidity(PoolKey memory key, V4CallbackData memory d) internal {
        (uint160 sqrtP,,,) = poolManager.getSlot0(key.toId());
        int24 tickLower = (TickMath.MIN_TICK / key.tickSpacing) * key.tickSpacing;
        int24 tickUpper = (TickMath.MAX_TICK / key.tickSpacing) * key.tickSpacing;

        (uint256 amount0, uint256 amount1) =
            Currency.unwrap(key.currency0) == idrx ? (d.amountA, d.amountB) : (d.amountB, d.amountA);

        uint128 liquidity = LiquidityAmounts.getLiquidityForAmounts(
            sqrtP, TickMath.getSqrtPriceAtTick(tickLower), TickMath.getSqrtPriceAtTick(tickUpper), amount0, amount1
        );

        (BalanceDelta delta,) = poolManager.modifyLiquidity(
            key,
            ModifyLiquidityParams({
                tickLower: tickLower,
                tickUpper: tickUpper,
                liquidityDelta: int256(uint256(liquidity)),
                salt: bytes32(0)
            }),
            ""
        );

        _settleDelta(key, delta.amount0(), delta.amount1(), address(this));
    }

    function _v4Swap(PoolKey memory key, V4CallbackData memory d) internal {
        bool idrxIs0 = Currency.unwrap(key.currency0) == idrx;
        bool zeroForOne = d.buyStock ? idrxIs0 : !idrxIs0;

        BalanceDelta delta = poolManager.swap(
            key,
            SwapParams({
                zeroForOne: zeroForOne,
                amountSpecified: -int256(d.amountA),
                sqrtPriceLimitX96: zeroForOne ? TickMath.MIN_SQRT_PRICE + 1 : TickMath.MAX_SQRT_PRICE - 1
            }),
            ""
        );

        int128 d0 = delta.amount0();
        int128 d1 = delta.amount1();

        // Settle inputs we owe (negative), send outputs owed to us (positive) to the user.
        if (d0 < 0) key.currency0.settle(poolManager, address(this), uint256(uint128(-d0)), false);
        if (d1 < 0) key.currency1.settle(poolManager, address(this), uint256(uint128(-d1)), false);

        uint256 amountOut;
        if (d0 > 0) {
            amountOut = uint256(uint128(d0));
            key.currency0.take(poolManager, d.user, amountOut, false);
        }
        if (d1 > 0) {
            amountOut = uint256(uint128(d1));
            key.currency1.take(poolManager, d.user, amountOut, false);
        }

        if (amountOut < d.amountB) revert SlippageExceeded(amountOut, d.amountB);
        emit V4Swapped(d.ticker, d.user, d.buyStock, d.amountA, amountOut);
    }

    /// @dev Settles negative deltas (owed by us) and takes positive deltas (owed to us) to `to`.
    function _settleDelta(PoolKey memory key, int128 d0, int128 d1, address to) internal {
        if (d0 < 0) key.currency0.settle(poolManager, address(this), uint256(uint128(-d0)), false);
        if (d1 < 0) key.currency1.settle(poolManager, address(this), uint256(uint128(-d1)), false);
        if (d0 > 0) key.currency0.take(poolManager, to, uint256(uint128(d0)), false);
        if (d1 > 0) key.currency1.take(poolManager, to, uint256(uint128(d1)), false);
    }

    function _sortCurrencies(address a, address b) internal pure returns (Currency, Currency) {
        return a < b ? (Currency.wrap(a), Currency.wrap(b)) : (Currency.wrap(b), Currency.wrap(a));
    }

    function _requireV4Pool(string calldata ticker) internal view returns (PoolKey memory key) {
        key = poolKeys[ticker];
        if (address(key.hooks) == address(0)) revert V4PoolNotFound(ticker);
    }

    function _requireV4PoolMem(string memory ticker) internal view returns (PoolKey memory key) {
        key = poolKeys[ticker];
        if (address(key.hooks) == address(0)) revert V4PoolNotFound(ticker);
    }

    // ─── KYC Management (delegated to PulsarProtocolOps) ────────────────────

    function approveKYC(address wallet) external {
        _delegateToOps();
    }

    function revokeKYC(address wallet) external {
        _delegateToOps();
    }

    // ─── Circuit breaker ────────────────────────────────────────────────────

    /// @notice Emergency stop. Any custodian can pause (fast incident response);
    ///         only admin can unpause (deliberate all-clear). Refund/reject and
    ///         withdrawal paths stay open while paused so funds are never trapped.
    function pause() external onlyRole(CUSTODIAN_ROLE) {
        _pause();
    }

    function unpause() external onlyRole(DEFAULT_ADMIN_ROLE) {
        _unpause();
    }

    /// @notice Pool-level circuit breaker: halts ALL V4 swaps for every pool
    ///         (any entry point — this protocol, an aggregator, or Uniswap's own
    ///         UI), without touching liquidity. Narrower than emergencyWithdrawV4,
    ///         which pauses the hook AND pulls the ticker's liquidity out.
    ///         Requires this protocol to hold PAUSER_ROLE on the hook.
    function pauseHook() external onlyRole(CUSTODIAN_ROLE) {
        IPulsarSwapHookControl(swapHook).pause();
    }

    /// @notice Deliberate all-clear for the pool-level breaker. Admin-only,
    ///         mirroring the asymmetric pause/unpause split above.
    function unpauseHook() external onlyRole(DEFAULT_ADMIN_ROLE) {
        IPulsarSwapHookControl(swapHook).unpause();
    }

    // ─── Admin Config (delegated to PulsarProtocolOps) ──────────────────────
    // setOpsContract itself stays on the main contract (see above) — everything
    // else here is cold/rare enough to relocate.

    function setTreasury(address treasury_) external {
        _delegateToOps();
    }

    function setRedeemFeeBps(uint256 feeBps) external {
        _delegateToOps();
    }

    function setSwapFeeBps(uint256 feeBps) external {
        _delegateToOps();
    }

    function setMinimumDistributionThreshold(uint256 threshold) external {
        _delegateToOps();
    }

    function setRouter(address router_) external {
        _delegateToOps();
    }

    function setIDRX(address idrx_) external {
        _delegateToOps();
    }

    // ─── View ─────────────────────────────────────────────────────────────────

    function getTickers() external view returns (string[] memory) {
        return _tickers;
    }

    /// @notice Spot value of `tokenAmount` raw stock in raw IDRX from the ticker's
    ///         V4 pool price. Reverts if the ticker has no V4 pool. Intended for
    ///         off-chain price feeds and UI quotes — identical math to the redeem
    ///         fee quote, so on-chain and off-chain never drift.
    function quoteStockToIdrx(string calldata ticker, uint256 tokenAmount) external view returns (uint256) {
        return _quoteStockToIdrxV4(ticker, tokenAmount);
    }

    // ─── Internal ─────────────────────────────────────────────────────────────

    function _ensureStock(string memory ticker, string memory stockName, string memory idxTicker)
        internal
        returns (address)
    {
        address stockAddress = stocks[ticker];
        if (stockAddress != address(0)) return stockAddress;

        PulsarStock token = new PulsarStock(stockName, ticker, idxTicker, address(this));
        stocks[ticker] = address(token);
        _tickers.push(ticker);
        emit StockDeployed(ticker, address(token));
        return address(token);
    }

    function _mint(address stockAddress, string memory ticker, address to, uint256 amount, bytes32 attestationHash)
        internal
    {
        PulsarStock(stockAddress).mint(to, amount, attestationHash);
        emit TokensMinted(ticker, to, amount, attestationHash);
    }

    /// @notice Values `tokenAmount` of stock in IDRX using the V4 pool spot price
    ///         (`getSlot0`). The pool holds raw balances, so its price already
    ///         encodes the IDRX(2)/stock(18) decimal gap — the result is in raw
    ///         IDRX units with no extra scaling. Uses `FullMath.mulDiv` so the
    ///         `sqrtP^2` term never overflows.
    function _quoteStockToIdrxV4(string calldata ticker, uint256 tokenAmount) internal view returns (uint256) {
        PoolKey memory key = poolKeys[ticker];
        if (address(key.hooks) == address(0)) revert V4PoolNotFound(ticker);
        (uint160 sqrtPriceX96,,,) = poolManager.getSlot0(key.toId());

        // pool price P = (sqrtP/2^96)^2 = currency1 per currency0 (raw units).
        if (Currency.unwrap(key.currency0) == idrx) {
            // idrx = currency0, stock = currency1: P = stock per idrx.
            // idrxOut = tokenAmount / P = tokenAmount * 2^192 / sqrtP^2.
            uint256 half = FullMath.mulDiv(tokenAmount, FixedPoint96.Q96, sqrtPriceX96);
            return FullMath.mulDiv(half, FixedPoint96.Q96, sqrtPriceX96);
        } else {
            // stock = currency0, idrx = currency1: P = idrx per stock.
            // idrxOut = tokenAmount * P = tokenAmount * sqrtP^2 / 2^192.
            uint256 half = FullMath.mulDiv(tokenAmount, sqrtPriceX96, FixedPoint96.Q96);
            return FullMath.mulDiv(half, sqrtPriceX96, FixedPoint96.Q96);
        }
    }

    function _markActiveCustodian(address custodian) internal {
        if (!isActiveCustodian[custodian]) {
            isActiveCustodian[custodian] = true;
            _activeCustodians.push(custodian);
        }
    }

    function _requireStock(string calldata ticker) internal view returns (address stockAddress) {
        stockAddress = stocks[ticker];
        if (stockAddress == address(0)) revert StockNotFound(ticker);
    }

    function _authorizeUpgrade(address) internal override onlyRole(DEFAULT_ADMIN_ROLE) {}

    // _msgSender/_msgData/_contextSuffixLength (Context vs ContextUpgradeable
    // resolution) live in PulsarProtocolStorage — inherited from there, not
    // redeclared here.
}
