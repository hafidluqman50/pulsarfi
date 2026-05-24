// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {Initializable} from "@openzeppelin/contracts/proxy/utils/Initializable.sol";
import {UUPSUpgradeable} from "@openzeppelin/contracts/proxy/utils/UUPSUpgradeable.sol";
import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";
import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {PulsarStock} from "./PulsarStock.sol";
import {IUniswapV2Router02} from "./interfaces/IUniswapV2Router02.sol";
import {IUniswapV2Factory} from "./interfaces/IUniswapV2Factory.sol";

interface IMintable {
    function mint(address to, uint256 amount) external;
}

error StockNotFound(string ticker);
error KYCRequired(address wallet);
error ProposalNotFound(uint256 proposalId);
error AlreadyApproved(uint256 proposalId, address custodian);
error ProposalAlreadyExecuted(uint256 proposalId);
error MintRequestPending(string ticker);
error NotRequester(uint256 proposalId, address caller);
error ThresholdNotMet(uint256 proposalId, uint8 current, uint8 required);

/// @notice Single entry point for all PulsarFi protocol operations.
///         UUPS upgradeable to support future Uniswap V4 migration.
contract PulsarProtocol is Initializable, UUPSUpgradeable, AccessControl {
    bytes32 public constant CUSTODIAN_ROLE = keccak256("CUSTODIAN_ROLE");
    uint8   public constant THRESHOLD      = 3;

    enum MintDestination { OperatorWallet, LiquidityPool }

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
    }

    IUniswapV2Router02 public router;
    address public idrx;

    mapping(string => address) public stocks;
    string[] private _tickers;
    mapping(address => bool) public kycApproved;

    mapping(uint256 => MintProposal) public proposals;
    mapping(uint256 => mapping(address => bool)) public hasApproved;
    mapping(string => bool) public hasPendingRequest;
    uint256 public proposalCount;

    event StockDeployed(string indexed ticker, address contractAddress);
    event TokensMinted(string indexed ticker, address indexed to, uint256 amount, bytes32 attestationHash);
    event PoolCreated(string indexed ticker, uint256 tokenAmount, uint256 idrxAmount, uint256 liquidity);
    event LiquidityAdded(string indexed ticker, uint256 tokenAmount, uint256 idrxAmount, uint256 liquidity);
    event TokensRedeemed(string indexed ticker, address indexed from, uint256 amount, bytes32 attestationHash);
    event KYCApproved(address indexed wallet);
    event KYCRevoked(address indexed wallet);
    event MintRequested(uint256 indexed proposalId, address indexed requester, string ticker);
    event MintApproved(uint256 indexed proposalId, address indexed approver, uint8 approvalCount);
    event MintExecuted(uint256 indexed proposalId);
    event TokensSwapped(string indexed ticker, address indexed user, bool buyStock, uint256 amountIn, uint256 amountOut);

    /// @custom:oz-upgrades-unsafe-allow constructor
    constructor() {
        _disableInitializers();
    }

    function initialize(
        address admin,
        address router_,
        address idrx_,
        address[] calldata custodians
    ) external initializer {
        _grantRole(DEFAULT_ADMIN_ROLE, admin);
        for (uint256 i = 0; i < custodians.length; i++) {
            _grantRole(CUSTODIAN_ROLE, custodians[i]);
        }
        router = IUniswapV2Router02(router_);
        idrx   = idrx_;
    }

    // ─── Multisig Proposal ────────────────────────────────────────────────────

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
        proposal.ticker          = ticker;
        proposal.stockName       = stockName;
        proposal.idxTicker       = idxTicker;
        proposal.tokenAmount     = tokenAmount;
        proposal.idrxAmount      = idrxAmount;
        proposal.attestationHash = attestationHash;
        proposal.destination     = destination;
        proposal.requester       = msg.sender;
        proposal.approvalCount   = 1;

        hasApproved[proposalId][msg.sender] = true;

        emit MintRequested(proposalId, msg.sender, ticker);
        emit MintApproved(proposalId, msg.sender, 1);
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

    function executeMint(uint256 proposalId) external onlyRole(CUSTODIAN_ROLE) {
        MintProposal storage proposal = proposals[proposalId];

        if (proposal.approvalCount == 0 && !proposal.executed) revert ProposalNotFound(proposalId);
        if (proposal.executed) revert ProposalAlreadyExecuted(proposalId);
        if (proposal.requester != msg.sender) revert NotRequester(proposalId, msg.sender);
        if (proposal.approvalCount < THRESHOLD) revert ThresholdNotMet(proposalId, proposal.approvalCount, THRESHOLD);

        proposal.executed = true;
        hasPendingRequest[proposal.ticker] = false;
        emit MintExecuted(proposalId);

        address stockAddress = _ensureStock(proposal.ticker, proposal.stockName, proposal.idxTicker);
        address mintTarget   = proposal.destination == MintDestination.LiquidityPool ? address(this) : msg.sender;

        _mint(stockAddress, proposal.ticker, mintTarget, proposal.tokenAmount, proposal.attestationHash);

        if (proposal.destination == MintDestination.LiquidityPool) {
            _provideToPool(stockAddress, proposal.ticker, proposal.tokenAmount, proposal.idrxAmount);
        }
    }

    // ─── Redemption ───────────────────────────────────────────────────────────

    function redeem(
        string calldata ticker,
        address user,
        uint256 amount,
        bytes32 attestationHash
    ) external onlyRole(CUSTODIAN_ROLE) {
        if (!kycApproved[user]) revert KYCRequired(user);
        address stockAddress = _requireStock(ticker);
        PulsarStock(stockAddress).burn(user, amount, attestationHash);
        emit TokensRedeemed(ticker, user, amount, attestationHash);
    }

    // ─── Swap ─────────────────────────────────────────────────────────────────

    /// @notice Permissionless swap. buyStock=true: IDRX → PulsarStock, false: PulsarStock → IDRX.
    function swap(
        string calldata ticker,
        uint256 amountIn,
        uint256 amountOutMin,
        bool buyStock
    ) external {
        address stockAddress = _requireStock(ticker);

        address tokenIn  = buyStock ? idrx : stockAddress;
        address tokenOut = buyStock ? stockAddress : idrx;

        IERC20(tokenIn).transferFrom(msg.sender, address(this), amountIn);
        IERC20(tokenIn).approve(address(router), amountIn);

        address[] memory path = new address[](2);
        path[0] = tokenIn;
        path[1] = tokenOut;

        uint256[] memory amounts = router.swapExactTokensForTokens(
            amountIn,
            amountOutMin,
            path,
            msg.sender,
            block.timestamp + 15 minutes
        );

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

    // ─── View ─────────────────────────────────────────────────────────────────

    function getTickers() external view returns (string[] memory) {
        return _tickers;
    }

    // ─── Internal ─────────────────────────────────────────────────────────────


    function _ensureStock(
        string memory ticker,
        string memory stockName,
        string memory idxTicker
    ) internal returns (address) {
        address stockAddress = stocks[ticker];
        if (stockAddress != address(0)) return stockAddress;

        PulsarStock token = new PulsarStock(stockName, ticker, idxTicker, address(this));
        stocks[ticker] = address(token);
        _tickers.push(ticker);
        emit StockDeployed(ticker, address(token));
        return address(token);
    }

    function _mint(
        address stockAddress,
        string memory ticker,
        address to,
        uint256 amount,
        bytes32 attestationHash
    ) internal {
        PulsarStock(stockAddress).mint(to, amount, attestationHash);
        emit TokensMinted(ticker, to, amount, attestationHash);
    }

    function _provideToPool(
        address stockAddress,
        string memory ticker,
        uint256 tokenAmount,
        uint256 idrxAmount
    ) internal {
        bool poolExists = IUniswapV2Factory(router.factory()).getPair(stockAddress, idrx) != address(0);

        IMintable(idrx).mint(address(this), idrxAmount);

        IERC20(stockAddress).approve(address(router), tokenAmount);
        IERC20(idrx).approve(address(router), idrxAmount);

        (uint256 actualToken, uint256 actualIdrx, uint256 liquidity) = router.addLiquidity(
            stockAddress,
            idrx,
            tokenAmount,
            idrxAmount,
            0,
            0,
            address(this),
            block.timestamp + 15 minutes
        );

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
