// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {Initializable} from "@openzeppelin/contracts/proxy/utils/Initializable.sol";
import {UUPSUpgradeable} from "@openzeppelin/contracts/proxy/utils/UUPSUpgradeable.sol";
import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";
import {PausableUpgradeable} from "@openzeppelin/contracts-upgradeable/utils/PausableUpgradeable.sol";
import {Context} from "@openzeppelin/contracts/utils/Context.sol";
import {ContextUpgradeable} from "@openzeppelin/contracts-upgradeable/utils/ContextUpgradeable.sol";
import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {SafeERC20} from "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import {PulsarStock} from "./PulsarStock.sol";
import {IUniswapV2Router02} from "./interfaces/IUniswapV2Router02.sol";
import {IUniswapV2Factory} from "./interfaces/IUniswapV2Factory.sol";
import {IPoolManager} from "@uniswap/v4-core/src/interfaces/IPoolManager.sol";
import {IUnlockCallback} from "@uniswap/v4-core/src/interfaces/callback/IUnlockCallback.sol";
import {IHooks} from "@uniswap/v4-core/src/interfaces/IHooks.sol";
import {PoolKey} from "@uniswap/v4-core/src/types/PoolKey.sol";
import {PoolId, PoolIdLibrary} from "@uniswap/v4-core/src/types/PoolId.sol";
import {Currency} from "@uniswap/v4-core/src/types/Currency.sol";
import {BalanceDelta} from "@uniswap/v4-core/src/types/BalanceDelta.sol";
import {SwapParams, ModifyLiquidityParams} from "@uniswap/v4-core/src/types/PoolOperation.sol";
import {StateLibrary} from "@uniswap/v4-core/src/libraries/StateLibrary.sol";
import {TickMath} from "@uniswap/v4-core/src/libraries/TickMath.sol";
import {CurrencySettler} from "@openzeppelin/uniswap-hooks/utils/CurrencySettler.sol";
import {LiquidityAmounts} from "@uniswap/v4-periphery/src/libraries/LiquidityAmounts.sol";

interface IPulsarSwapHookControl {
    function pause() external;
    function registerPool(PoolKey calldata key, string calldata ticker) external;
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

/// @notice Single entry point for all PulsarFi protocol operations.
///         UUPS upgradeable to support future Uniswap V4 migration.
contract PulsarProtocol is Initializable, UUPSUpgradeable, AccessControl, PausableUpgradeable, IUnlockCallback {
    using SafeERC20 for IERC20;
    using StateLibrary for IPoolManager;
    using CurrencySettler for Currency;
    using PoolIdLibrary for PoolKey;

    bytes32 public constant CUSTODIAN_ROLE = keccak256("CUSTODIAN_ROLE");
    uint8 public constant THRESHOLD = 3;

    struct MintProposal {
        string ticker;
        string stockName;
        string idxTicker;
        uint256 tokenAmount;
        uint256 idrxAmount;
        bytes32 attestationHash;
        uint8 __deprecatedDestination; // preserve slot — was MintDestination enum, removed in v3
        address requester;
        uint8 approvalCount;
        bool executed;
        address rejectInitiator;
        uint8 rejectCount;
    }

    struct RedeemRequest {
        string ticker;
        address user;
        uint256 tokenAmount;
        uint256 feeIdrx;
        bool processed;
        bool approved;
        address approveInitiator;
        address rejectInitiator;
        uint8 approvalCount;
        uint8 rejectCount;
    }

    IUniswapV2Router02 public router;
    address public idrx;
    address public treasury;
    uint256 public redeemFeeBps;

    mapping(string => address) public stocks;
    string[] private _tickers;
    mapping(address => bool) public kycApproved;

    mapping(uint256 => MintProposal) public proposals;
    mapping(uint256 => mapping(address => bool)) public hasApproved;
    mapping(string => bool) public hasPendingRequest;
    uint256 public proposalCount;

    mapping(uint256 => RedeemRequest) public redeemRequests;
    uint256 public redeemRequestCount;

    mapping(uint256 => mapping(address => bool)) public hasRejectedMint;
    mapping(uint256 => mapping(address => bool)) public hasApprovedRedeem;
    mapping(uint256 => mapping(address => bool)) public hasRejectedRedeem;

    // Storage slot preserved for UUPS layout compatibility — used only for legacy
    // proposals created before this upgrade. New proposals never write to this mapping.
    mapping(uint256 => uint256) public mintLiquidityFunding;

    // New state below this line — appended, never inserted, to preserve UUPS storage layout.
    uint256 public swapFeeBps;
    uint256 public minimumDistributionThreshold;
    uint256 public accumulatedFees;
    mapping(address => bool) public isActiveCustodian;
    address[] private _activeCustodians;

    // ─── V4 migration state (appended) ──────────────────────────────────────
    IPoolManager public poolManager;
    address public swapHook;
    mapping(string => PoolKey) public poolKeys; // ticker => V4 pool key
    mapping(string => bool) public isV4Migrated; // ticker => swaps route to V4

    event StockDeployed(string indexed ticker, address contractAddress);
    event TokensMinted(string indexed ticker, address indexed to, uint256 amount, bytes32 attestationHash);
    event PoolCreated(string indexed ticker, uint256 tokenAmount, uint256 idrxAmount, uint256 liquidity);
    event LiquidityAdded(string indexed ticker, uint256 tokenAmount, uint256 idrxAmount, uint256 liquidity);
    event TokensRedeemed(string indexed ticker, address indexed from, uint256 amount);
    event KYCApproved(address indexed wallet);
    event KYCRevoked(address indexed wallet);
    event MintRequested(uint256 indexed proposalId, address indexed requester, string ticker);
    event MintApproved(uint256 indexed proposalId, address indexed approver, uint8 approvalCount);
    event MintExecuted(uint256 indexed proposalId);
    event MintRejectionVoted(uint256 indexed proposalId, address indexed custodian, uint8 rejectCount);
    event MintRejected(uint256 indexed proposalId, address indexed rejectInitiator);
    event TokensSwapped(
        string indexed ticker, address indexed user, bool buyStock, uint256 amountIn, uint256 amountOut
    );
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
    event SwapFeeCollected(string indexed ticker, address indexed user, uint256 feeIdrx);
    event FeesDistributed(uint256 treasuryAmount, uint256 custodianAmount, uint256 recipientCount);
    event RouterUpdated(address indexed router);
    event IDRXUpdated(address indexed idrx);

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

    /// @notice Deprecated: retained for storage layout compatibility only.
    ///         Do not call — IDRX is now pulled from the requester at executeMint.
    function fundMintLiquidity(uint256 proposalId, uint256 amount) external onlyRole(CUSTODIAN_ROLE) {
        MintProposal storage proposal = proposals[proposalId];
        if (proposal.approvalCount == 0 && !proposal.executed) revert ProposalNotFound(proposalId);
        if (proposal.executed) revert ProposalAlreadyExecuted(proposalId);
        if (amount == 0) revert InvalidAmount();

        IERC20(idrx).safeTransferFrom(msg.sender, address(this), amount);
        mintLiquidityFunding[proposalId] += amount;
    }

    function approveMint(uint256 proposalId) external onlyRole(CUSTODIAN_ROLE) {
        MintProposal storage proposal = proposals[proposalId];

        if (proposal.approvalCount == 0 && !proposal.executed) revert ProposalNotFound(proposalId);
        if (proposal.executed) revert ProposalAlreadyExecuted(proposalId);
        if (hasApproved[proposalId][msg.sender]) revert AlreadyApproved(proposalId, msg.sender);

        hasApproved[proposalId][msg.sender] = true;
        proposal.approvalCount++;
        _markActiveCustodian(msg.sender);

        emit MintApproved(proposalId, msg.sender, proposal.approvalCount);
    }

    /// @notice Requester executes after 3/5 approvals.
    ///         Pulls idrxAmount from msg.sender at this point.
    ///         Caller must have approved this contract for idrxAmount IDRX beforehand.
    /// @notice Requester executes after 3/5 approvals.
    ///         Pulls idrxAmount from msg.sender at this point.
    ///         Caller must have approved this contract for idrxAmount IDRX beforehand.
    function executeMint(uint256 proposalId) external onlyRole(CUSTODIAN_ROLE) whenNotPaused {
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
        _provideToPool(proposalId, stockAddress, proposal.ticker, proposal.tokenAmount, proposal.idrxAmount);
        emit MintExecuted(proposalId);
    }

    /// @notice Custodian votes to reject a mint proposal.
    function rejectMint(uint256 proposalId) external onlyRole(CUSTODIAN_ROLE) {
        MintProposal storage proposal = proposals[proposalId];

        if (proposal.approvalCount == 0 && !proposal.executed) revert ProposalNotFound(proposalId);
        if (proposal.executed) revert ProposalAlreadyExecuted(proposalId);
        if (hasRejectedMint[proposalId][msg.sender]) revert AlreadyRejectedMint(proposalId, msg.sender);

        hasRejectedMint[proposalId][msg.sender] = true;
        proposal.rejectCount++;

        if (proposal.rejectInitiator == address(0)) {
            proposal.rejectInitiator = msg.sender;
        }

        emit MintRejectionVoted(proposalId, msg.sender, proposal.rejectCount);
    }

    /// @notice rejectInitiator closes the proposal after 3/5 rejection votes.
    ///         Refunds any IDRX already locked via fundMintLiquidity (legacy pre-upgrade proposals).
    function executeRejectMint(uint256 proposalId) external onlyRole(CUSTODIAN_ROLE) {
        MintProposal storage proposal = proposals[proposalId];

        if (proposal.approvalCount == 0 && !proposal.executed) revert ProposalNotFound(proposalId);
        if (proposal.executed) revert ProposalAlreadyExecuted(proposalId);
        if (proposal.rejectInitiator != msg.sender) revert NotMintRejectInitiator(proposalId, msg.sender);
        if (proposal.rejectCount < THRESHOLD) {
            revert ThresholdNotMet(proposalId, proposal.rejectCount, THRESHOLD);
        }

        proposal.executed = true;
        hasPendingRequest[proposal.ticker] = false;

        // Refund any IDRX pre-locked for this proposal (handles legacy pre-upgrade proposals).
        uint256 funded = mintLiquidityFunding[proposalId];
        if (funded > 0) {
            mintLiquidityFunding[proposalId] = 0;
            IERC20(idrx).safeTransfer(proposal.requester, funded);
        }

        emit MintRejected(proposalId, msg.sender);
    }

    // ─── Redeem Request ───────────────────────────────────────────────────────

    function requestRedeem(string calldata ticker, uint256 tokenAmount) external {
        address user = msg.sender;
        if (!kycApproved[user]) revert KYCRequired(user);
        address stockAddress = _requireStock(ticker);

        uint256 feeIdrx = 0;
        if (redeemFeeBps > 0) {
            address[] memory path = new address[](2);
            path[0] = stockAddress;
            path[1] = idrx;
            uint256[] memory amounts = router.getAmountsOut(tokenAmount, path);
            feeIdrx = (amounts[1] * redeemFeeBps) / 10_000;
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

    function approveRedeem(uint256 requestId) external onlyRole(CUSTODIAN_ROLE) {
        RedeemRequest storage req = redeemRequests[requestId];
        if (req.user == address(0)) revert RedeemRequestNotFound(requestId);
        if (req.processed) revert RedeemRequestAlreadyProcessed(requestId);
        if (hasApprovedRedeem[requestId][msg.sender]) revert RedeemAlreadyApproved(requestId, msg.sender);

        hasApprovedRedeem[requestId][msg.sender] = true;
        req.approvalCount++;

        if (req.approveInitiator == address(0)) {
            req.approveInitiator = msg.sender;
        }

        emit RedeemApproved(requestId, msg.sender, req.approvalCount);
    }

    function executeRedeem(uint256 requestId) external onlyRole(CUSTODIAN_ROLE) {
        RedeemRequest storage req = redeemRequests[requestId];
        if (req.user == address(0)) revert RedeemRequestNotFound(requestId);
        if (req.processed) revert RedeemRequestAlreadyProcessed(requestId);
        if (req.approveInitiator != msg.sender) revert NotRedeemInitiator(requestId, msg.sender);
        if (req.approvalCount < THRESHOLD) {
            revert RedeemThresholdNotMet(requestId, req.approvalCount, THRESHOLD);
        }

        req.processed = true;
        req.approved = true;

        address stockAddress = stocks[req.ticker];
        PulsarStock(stockAddress).burn(address(this), req.tokenAmount, bytes32(0));

        if (req.feeIdrx > 0) {
            accumulatedFees += req.feeIdrx;
        }

        emit RedeemExecuted(requestId, msg.sender);
        emit TokensRedeemed(req.ticker, req.user, req.tokenAmount);
    }

    function rejectRedeem(uint256 requestId) external onlyRole(CUSTODIAN_ROLE) {
        RedeemRequest storage req = redeemRequests[requestId];
        if (req.user == address(0)) revert RedeemRequestNotFound(requestId);
        if (req.processed) revert RedeemRequestAlreadyProcessed(requestId);
        if (hasRejectedRedeem[requestId][msg.sender]) revert RedeemAlreadyRejected(requestId, msg.sender);

        hasRejectedRedeem[requestId][msg.sender] = true;
        req.rejectCount++;

        if (req.rejectInitiator == address(0)) req.rejectInitiator = msg.sender;

        emit RedeemRejectionVoted(requestId, msg.sender, req.rejectCount);
    }

    function executeReject(uint256 requestId) external onlyRole(CUSTODIAN_ROLE) {
        RedeemRequest storage req = redeemRequests[requestId];
        if (req.user == address(0)) revert RedeemRequestNotFound(requestId);
        if (req.processed) revert RedeemRequestAlreadyProcessed(requestId);
        if (req.rejectInitiator != msg.sender) revert NotRedeemInitiator(requestId, msg.sender);
        if (req.rejectCount < THRESHOLD) {
            revert RedeemThresholdNotMet(requestId, req.rejectCount, THRESHOLD);
        }

        req.processed = true;
        req.approved = false;

        address stockAddress = stocks[req.ticker];
        IERC20(stockAddress).safeTransfer(req.user, req.tokenAmount);

        if (req.feeIdrx > 0) {
            IERC20(idrx).safeTransfer(req.user, req.feeIdrx);
        }

        emit RedeemRejected(requestId, msg.sender);
    }

    // ─── Swap ─────────────────────────────────────────────────────────────────

    /// @notice Permissionless swap. Protocol fee (`swapFeeBps`) is always denominated
    ///         in IDRX: taken from `amountIn` when buying stock, from the IDRX output
    ///         when selling stock. Fee stays in this contract's own balance and is
    ///         tracked in `accumulatedFees` until `distributeFees` is called.
    function swap(string calldata ticker, uint256 amountIn, uint256 amountOutMin, bool buyStock) external whenNotPaused {
        address stockAddress = _requireStock(ticker);

        address tokenIn  = buyStock ? idrx : stockAddress;
        address tokenOut = buyStock ? stockAddress : idrx;

        IERC20(tokenIn).safeTransferFrom(msg.sender, address(this), amountIn);

        uint256 feeAmount = buyStock ? _swapFeeAmount(amountIn) : 0;
        uint256 swapAmountIn = amountIn - feeAmount;

        IERC20(tokenIn).approve(address(router), swapAmountIn);

        address[] memory path = new address[](2);
        path[0] = tokenIn;
        path[1] = tokenOut;

        address outputRecipient = (!buyStock && swapFeeBps > 0) ? address(this) : msg.sender;

        uint256[] memory amounts = router.swapExactTokensForTokens(
            swapAmountIn, amountOutMin, path, outputRecipient, block.timestamp + 15 minutes
        );

        uint256 amountOut = amounts[amounts.length - 1];

        if (!buyStock && swapFeeBps > 0) {
            feeAmount = _swapFeeAmount(amountOut);
            amountOut -= feeAmount;
            IERC20(idrx).safeTransfer(msg.sender, amountOut);
        }

        _recordSwapFee(ticker, feeAmount);

        emit TokensSwapped(ticker, msg.sender, buyStock, amounts[0], amountOut);
    }

    /// @notice Computes the protocol fee portion of `amount` given `swapFeeBps`.
    function _swapFeeAmount(uint256 amount) internal view returns (uint256) {
        return swapFeeBps == 0 ? 0 : (amount * swapFeeBps) / 10_000;
    }

    /// @notice Books a confirmed swap fee into accumulatedFees and emits it. No-op if zero.
    function _recordSwapFee(string calldata ticker, uint256 feeAmount) internal {
        if (feeAmount == 0) return;
        accumulatedFees += feeAmount;
        emit SwapFeeCollected(ticker, msg.sender, feeAmount);
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
        REMOVE_ALL_LIQUIDITY
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
    event V2ToV4Migrated(string indexed ticker, uint256 idrxRecovered, uint256 stockRecovered);
    event V4EmergencyWithdrawn(string indexed ticker, uint256 idrxRecovered, uint256 stockRecovered);

    function configureV4(address poolManager_, address swapHook_) external onlyRole(DEFAULT_ADMIN_ROLE) {
        if (poolManager_ == address(0) || swapHook_ == address(0)) revert InvalidAddress();
        poolManager = IPoolManager(poolManager_);
        swapHook = swapHook_;
        emit V4Configured(poolManager_, swapHook_);
    }

    /// @dev Initializes the V4 pool for `ticker` and registers it with the hook.
    ///      Stock must already be deployed (via an earlier executeMint). Internal:
    ///      the only path to a V4 pool today is migrateV2ToV4, which enforces
    ///      CUSTODIAN_ROLE. Add a tested external entrypoint if/when a greenfield
    ///      V4 listing (a stock with no V2 pool) is ever required.
    function _createV4Pool(string memory ticker, uint160 sqrtPriceX96, int24 tickSpacing, uint24 lpFee) internal {
        if (address(poolManager) == address(0) || swapHook == address(0)) revert V4NotConfigured();
        address stockAddress = stocks[ticker];
        if (stockAddress == address(0)) revert StockNotFound(ticker);
        if (address(poolKeys[ticker].hooks) != address(0)) revert V4PoolExists(ticker);

        (Currency c0, Currency c1) = _sortCurrencies(idrx, stockAddress);
        PoolKey memory key =
            PoolKey({currency0: c0, currency1: c1, fee: lpFee, tickSpacing: tickSpacing, hooks: IHooks(swapHook)});
        poolManager.initialize(key, sqrtPriceX96);
        poolKeys[ticker] = key;
        IPulsarSwapHookControl(swapHook).registerPool(key, ticker);
        emit V4PoolCreated(ticker, sqrtPriceX96);
    }

    /// @notice Adds full-range liquidity to the V4 pool from the caller's funds.
    ///         Once seeded, swaps for this ticker route to V4.
    function addV4Liquidity(string calldata ticker, uint256 idrxAmount, uint256 stockAmount)
        external
        onlyRole(CUSTODIAN_ROLE)
        whenNotPaused
    {
        if (idrxAmount == 0 || stockAmount == 0) revert InvalidAmount();
        IERC20(idrx).safeTransferFrom(msg.sender, address(this), idrxAmount);
        IERC20(stocks[ticker]).safeTransferFrom(msg.sender, address(this), stockAmount);
        _provideV4Liquidity(ticker, idrxAmount, stockAmount);
    }

    /// @dev Seeds V4 liquidity from tokens the protocol already holds (no pull).
    ///      Used by addV4Liquidity (after pulling) and migrateV2ToV4 (from
    ///      recovered V2 liquidity).
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
    ) external onlyRole(CUSTODIAN_ROLE) {
        address stockAddress = stocks[ticker];
        if (stockAddress == address(0)) revert StockNotFound(ticker);

        // 1. Pull all protocol-owned V2 liquidity back into the protocol.
        address pair = IUniswapV2Factory(router.factory()).getPair(stockAddress, idrx);
        uint256 lp = pair == address(0) ? 0 : IERC20(pair).balanceOf(address(this));
        if (lp > 0) {
            IERC20(pair).approve(address(router), lp);
            router.removeLiquidity(
                stockAddress, idrx, lp, minStockOut, minIdrxOut, address(this), block.timestamp + 15 minutes
            );
        }

        // 2. Ensure the V4 pool exists.
        if (address(poolKeys[ticker].hooks) == address(0)) {
            _createV4Pool(ticker, sqrtPriceX96, tickSpacing, lpFee);
        }

        // 3. Re-seed V4 with everything recovered.
        uint256 idrxBal = IERC20(idrx).balanceOf(address(this));
        uint256 stockBal = IERC20(stockAddress).balanceOf(address(this));
        if (idrxBal == 0 || stockBal == 0) revert InvalidAmount();
        _provideV4Liquidity(ticker, idrxBal, stockBal);

        emit V2ToV4Migrated(ticker, idrxBal, stockBal);
    }

    /// @notice Escape hatch: pause the hook (halts all swaps on its pools) and
    ///         pull the protocol's entire V4 liquidity for `ticker` back into the
    ///         protocol. Callable during an incident; not blocked by the
    ///         protocol pause. Requires the protocol to hold PAUSER_ROLE on the hook.
    function emergencyWithdrawV4(string calldata ticker) external onlyRole(CUSTODIAN_ROLE) {
        _requireV4Pool(ticker);
        IPulsarSwapHookControl(swapHook).pause();

        uint256 idrxBefore = IERC20(idrx).balanceOf(address(this));
        uint256 stockBefore = IERC20(stocks[ticker]).balanceOf(address(this));

        poolManager.unlock(
            abi.encode(
                V4CallbackData({
                    action: V4Action.REMOVE_ALL_LIQUIDITY,
                    ticker: ticker,
                    amountA: 0,
                    amountB: 0,
                    buyStock: false,
                    user: address(this)
                })
            )
        );

        isV4Migrated[ticker] = false;
        emit V4EmergencyWithdrawn(
            ticker,
            IERC20(idrx).balanceOf(address(this)) - idrxBefore,
            IERC20(stocks[ticker]).balanceOf(address(this)) - stockBefore
        );
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

    function unlockCallback(bytes calldata data) external returns (bytes memory) {
        if (msg.sender != address(poolManager)) revert NotPoolManager();
        V4CallbackData memory d = abi.decode(data, (V4CallbackData));
        PoolKey memory key = poolKeys[d.ticker];
        if (d.action == V4Action.ADD_LIQUIDITY) {
            _v4AddLiquidity(key, d);
        } else if (d.action == V4Action.SWAP) {
            _v4Swap(key, d);
        } else {
            _v4RemoveAllLiquidity(key);
        }
        return "";
    }

    /// @dev Removes the protocol's entire full-range position and takes both
    ///      tokens back to the protocol. Used by emergencyWithdrawV4.
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

    // ─── KYC Management ──────────────────────────────────────────────────────

    function approveKYC(address wallet) external onlyRole(CUSTODIAN_ROLE) {
        kycApproved[wallet] = true;
        emit KYCApproved(wallet);
    }

    function revokeKYC(address wallet) external onlyRole(CUSTODIAN_ROLE) {
        kycApproved[wallet] = false;
        emit KYCRevoked(wallet);
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

    // ─── Admin Config ─────────────────────────────────────────────────────────

    function setTreasury(address treasury_) external onlyRole(DEFAULT_ADMIN_ROLE) {
        treasury = treasury_;
        emit TreasuryUpdated(treasury_);
    }

    function setRedeemFeeBps(uint256 feeBps) external onlyRole(DEFAULT_ADMIN_ROLE) {
        require(feeBps <= 1_000, "max 10%");
        redeemFeeBps = feeBps;
        emit RedeemFeeBpsUpdated(feeBps);
    }

    function setSwapFeeBps(uint256 feeBps) external onlyRole(DEFAULT_ADMIN_ROLE) {
        require(feeBps <= 1_000, "max 10%");
        swapFeeBps = feeBps;
        emit SwapFeeBpsUpdated(feeBps);
    }

    function setMinimumDistributionThreshold(uint256 threshold) external onlyRole(DEFAULT_ADMIN_ROLE) {
        minimumDistributionThreshold = threshold;
        emit MinimumDistributionThresholdUpdated(threshold);
    }

    function setRouter(address router_) external onlyRole(DEFAULT_ADMIN_ROLE) {
        if (router_ == address(0)) revert InvalidAddress();
        router = IUniswapV2Router02(router_);
        emit RouterUpdated(router_);
    }

    function setIDRX(address idrx_) external onlyRole(DEFAULT_ADMIN_ROLE) {
        if (idrx_ == address(0)) revert InvalidAddress();
        idrx = idrx_;
        emit IDRXUpdated(idrx_);
    }

    // ─── Fee Distribution ─────────────────────────────────────────────────────

    /// @notice Permissionless: anyone can trigger distribution once accumulatedFees
    ///         reaches minimumDistributionThreshold. 30% to treasury, 70% split
    ///         equally among custodians that have ever approved/requested a mint.
    function distributeFees() external {
        uint256 balance = accumulatedFees;
        if (balance < minimumDistributionThreshold) {
            revert BelowDistributionThreshold(balance, minimumDistributionThreshold);
        }

        uint256 count = _activeCustodians.length;
        if (count == 0) revert NoActiveCustodians();

        accumulatedFees = 0;

        uint256 treasuryShare = (balance * 30) / 100;
        uint256 custodianPool = balance - treasuryShare;
        uint256 perCustodian = custodianPool / count;
        uint256 remainder = custodianPool - (perCustodian * count);

        if (treasury != address(0)) {
            IERC20(idrx).safeTransfer(treasury, treasuryShare + remainder);
        }

        for (uint256 i = 0; i < count; i++) {
            IERC20(idrx).safeTransfer(_activeCustodians[i], perCustodian);
        }

        emit FeesDistributed(treasuryShare + remainder, perCustodian * count, count);
    }

    // ─── View ─────────────────────────────────────────────────────────────────

    function getTickers() external view returns (string[] memory) {
        return _tickers;
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

    /// @notice Provides liquidity to the Uniswap V2 pool for a mint proposal.
    ///         IDRX is pulled from msg.sender (= proposal.requester, enforced by executeMint).
    ///         Any IDRX already funded via the legacy fundMintLiquidity path is used first.
    function _provideToPool(
        uint256 proposalId,
        address stockAddress,
        string memory ticker,
        uint256 tokenAmount,
        uint256 idrxAmount
    ) internal {
        bool poolExists = IUniswapV2Factory(router.factory()).getPair(stockAddress, idrx) != address(0);

        // Use any pre-funded IDRX (legacy proposals), pull the remainder from msg.sender.
        uint256 alreadyFunded = mintLiquidityFunding[proposalId];
        if (alreadyFunded < idrxAmount) {
            IERC20(idrx).safeTransferFrom(msg.sender, address(this), idrxAmount - alreadyFunded);
        }
        mintLiquidityFunding[proposalId] = 0;

        IERC20(stockAddress).approve(address(router), tokenAmount);
        IERC20(idrx).approve(address(router), idrxAmount);

        (uint256 actualToken, uint256 actualIdrx, uint256 liquidity) = router.addLiquidity(
            stockAddress, idrx, tokenAmount, idrxAmount, 0, 0, address(this), block.timestamp + 15 minutes
        );

        // Refund any IDRX not consumed by addLiquidity (pool ratio may not need the full amount).
        uint256 idrxExcess = idrxAmount - actualIdrx;
        if (idrxExcess > 0) {
            IERC20(idrx).safeTransfer(msg.sender, idrxExcess);
        }

        if (poolExists) {
            emit LiquidityAdded(ticker, actualToken, actualIdrx, liquidity);
        } else {
            emit PoolCreated(ticker, actualToken, actualIdrx, liquidity);
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

    // ─── Context resolution ─────────────────────────────────────────────────
    // AccessControl (Context) and PausableUpgradeable (ContextUpgradeable) both
    // define these. Both are stateless and identical, so we resolve explicitly.
    // AccessControl stays non-upgradeable to preserve _roles at slot 0 on the
    // already-deployed proxy (AccessControlUpgradeable would move it to a
    // namespaced slot and orphan existing roles).

    function _msgSender() internal view override(Context, ContextUpgradeable) returns (address) {
        return msg.sender;
    }

    function _msgData() internal view override(Context, ContextUpgradeable) returns (bytes calldata) {
        return msg.data;
    }

    function _contextSuffixLength() internal view override(Context, ContextUpgradeable) returns (uint256) {
        return 0;
    }
}
