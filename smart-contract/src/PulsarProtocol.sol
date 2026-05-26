// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {Initializable} from "@openzeppelin/contracts/proxy/utils/Initializable.sol";
import {UUPSUpgradeable} from "@openzeppelin/contracts/proxy/utils/UUPSUpgradeable.sol";
import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";
import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {SafeERC20} from "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import {PulsarStock} from "./PulsarStock.sol";
import {IUniswapV2Router02} from "./interfaces/IUniswapV2Router02.sol";
import {IUniswapV2Factory} from "./interfaces/IUniswapV2Factory.sol";

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

/// @notice Single entry point for all PulsarFi protocol operations.
///         UUPS upgradeable to support future Uniswap V4 migration.
contract PulsarProtocol is Initializable, UUPSUpgradeable, AccessControl {
    using SafeERC20 for IERC20;

    bytes32 public constant CUSTODIAN_ROLE = keccak256("CUSTODIAN_ROLE");
    uint8 public constant THRESHOLD = 3;

    enum MintDestination {
        OperatorWallet,
        LiquidityPool
    }

    struct MintProposal {
        string ticker;
        string stockName;
        string idxTicker;
        uint256 tokenAmount;
        uint256 idrxAmount;
        bytes32 attestationHash;
        MintDestination destination;
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

    /// @notice Custodian submits a mint proposal. IDRX for LP is no longer locked here —
    ///         it is pulled from the requester's wallet at executeMint time instead.
    function requestMint(
        string calldata ticker,
        string calldata stockName,
        string calldata idxTicker,
        uint256 tokenAmount,
        uint256 idrxAmount,
        bytes32 attestationHash,
        MintDestination destination
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
        proposal.destination = destination;
        proposal.requester = msg.sender;
        proposal.approvalCount = 1;

        hasApproved[proposalId][msg.sender] = true;

        emit MintRequested(proposalId, msg.sender, ticker);
        emit MintApproved(proposalId, msg.sender, 1);
    }

    /// @notice Deprecated: retained for storage layout compatibility only.
    ///         Do not call — IDRX is now pulled from the requester at executeMint.
    function fundMintLiquidity(uint256 proposalId, uint256 amount) external onlyRole(CUSTODIAN_ROLE) {
        MintProposal storage proposal = proposals[proposalId];
        if (proposal.approvalCount == 0 && !proposal.executed) revert ProposalNotFound(proposalId);
        if (proposal.executed) revert ProposalAlreadyExecuted(proposalId);
        if (proposal.destination != MintDestination.LiquidityPool) revert InvalidAmount();
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

        emit MintApproved(proposalId, msg.sender, proposal.approvalCount);
    }

    /// @notice Requester executes after 3/5 approvals.
    ///         For LiquidityPool destination: pulls idrxAmount from msg.sender at this point.
    ///         Caller must have approved this contract for idrxAmount IDRX beforehand.
    function executeMint(uint256 proposalId) external onlyRole(CUSTODIAN_ROLE) {
        MintProposal storage proposal = proposals[proposalId];

        if (proposal.approvalCount == 0 && !proposal.executed) revert ProposalNotFound(proposalId);
        if (proposal.executed) revert ProposalAlreadyExecuted(proposalId);
        if (proposal.requester != msg.sender) revert NotRequester(proposalId, msg.sender);
        if (proposal.approvalCount < THRESHOLD) {
            revert ThresholdNotMet(proposalId, proposal.approvalCount, THRESHOLD);
        }

        proposal.executed = true;
        hasPendingRequest[proposal.ticker] = false;
        emit MintExecuted(proposalId);

        address stockAddress = _ensureStock(proposal.ticker, proposal.stockName, proposal.idxTicker);
        address mintTarget = proposal.destination == MintDestination.LiquidityPool ? address(this) : msg.sender;

        _mint(stockAddress, proposal.ticker, mintTarget, proposal.tokenAmount, proposal.attestationHash);

        if (proposal.destination == MintDestination.LiquidityPool) {
            _provideToPool(proposalId, stockAddress, proposal.ticker, proposal.tokenAmount, proposal.idrxAmount);
        }
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
        if (!kycApproved[msg.sender]) revert KYCRequired(msg.sender);
        address stockAddress = _requireStock(ticker);

        uint256 feeIdrx = 0;
        if (redeemFeeBps > 0) {
            address[] memory path = new address[](2);
            path[0] = stockAddress;
            path[1] = idrx;
            uint256[] memory amounts = router.getAmountsOut(tokenAmount, path);
            feeIdrx = (amounts[1] * redeemFeeBps) / 10_000;
        }

        IERC20(stockAddress).safeTransferFrom(msg.sender, address(this), tokenAmount);

        if (feeIdrx > 0) {
            IERC20(idrx).safeTransferFrom(msg.sender, address(this), feeIdrx);
        }

        uint256 requestId = redeemRequestCount++;
        RedeemRequest storage req = redeemRequests[requestId];
        req.ticker = ticker;
        req.user = msg.sender;
        req.tokenAmount = tokenAmount;
        req.feeIdrx = feeIdrx;

        emit RedeemRequested(requestId, msg.sender, ticker, tokenAmount, feeIdrx);
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

        if (req.feeIdrx > 0 && treasury != address(0)) {
            IERC20(idrx).safeTransfer(treasury, req.feeIdrx);
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

    function swap(string calldata ticker, uint256 amountIn, uint256 amountOutMin, bool buyStock) external {
        address stockAddress = _requireStock(ticker);

        address tokenIn  = buyStock ? idrx : stockAddress;
        address tokenOut = buyStock ? stockAddress : idrx;

        IERC20(tokenIn).safeTransferFrom(msg.sender, address(this), amountIn);
        IERC20(tokenIn).approve(address(router), amountIn);

        address[] memory path = new address[](2);
        path[0] = tokenIn;
        path[1] = tokenOut;

        uint256[] memory amounts =
            router.swapExactTokensForTokens(amountIn, amountOutMin, path, msg.sender, block.timestamp + 15 minutes);

        emit TokensSwapped(ticker, msg.sender, buyStock, amounts[0], amounts[amounts.length - 1]);
    }

    // ─── KYC Management ──────────────────────────────────────────────────────

    function approveKYC(address wallet) external onlyRole(DEFAULT_ADMIN_ROLE) {
        kycApproved[wallet] = true;
        emit KYCApproved(wallet);
    }

    function revokeKYC(address wallet) external onlyRole(DEFAULT_ADMIN_ROLE) {
        kycApproved[wallet] = false;
        emit KYCRevoked(wallet);
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

    function _requireStock(string calldata ticker) internal view returns (address stockAddress) {
        stockAddress = stocks[ticker];
        if (stockAddress == address(0)) revert StockNotFound(ticker);
    }

    function _authorizeUpgrade(address) internal override onlyRole(DEFAULT_ADMIN_ROLE) {}
}
