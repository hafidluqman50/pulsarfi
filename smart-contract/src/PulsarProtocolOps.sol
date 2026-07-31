// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {SafeERC20} from "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";
import {PulsarStock} from "./PulsarStock.sol";
import {PulsarProtocolStorage} from "./PulsarProtocolStorage.sol";
import {IUniswapV2Router02} from "./interfaces/IUniswapV2Router02.sol";
import {IUniswapV2Factory} from "./interfaces/IUniswapV2Factory.sol";
import {IPoolManager} from "@uniswap/v4-core/src/interfaces/IPoolManager.sol";
import {IHooks} from "@uniswap/v4-core/src/interfaces/IHooks.sol";
import {PoolKey} from "@uniswap/v4-core/src/types/PoolKey.sol";
import {Currency} from "@uniswap/v4-core/src/types/Currency.sol";
import {
    PulsarProtocol,
    IPulsarSwapHookControl,
    StockNotFound,
    ProposalNotFound,
    AlreadyApproved,
    AlreadyRejectedMint,
    ProposalAlreadyExecuted,
    NotMintRejectInitiator,
    ThresholdNotMet,
    RedeemRequestNotFound,
    RedeemRequestAlreadyProcessed,
    RedeemAlreadyApproved,
    RedeemAlreadyRejected,
    RedeemThresholdNotMet,
    NotRedeemInitiator,
    InvalidAddress,
    InvalidAmount,
    BelowDistributionThreshold,
    NoActiveCustodians,
    V4NotConfigured,
    V4PoolExists,
    V4PoolNotFound
} from "./PulsarProtocol.sol";

/// @notice Delegatecall target for the 20 PulsarProtocol functions that moved
///         out to keep the main contract's own bytecode under the EIP-170
///         24,576-byte deploy limit (mint/redeem approve+reject+execute flows,
///         V2->V4 migration, emergency withdraw, fee distribution, KYC, admin
///         setters). Every function here is called via `_delegateToOps()` on
///         the proxy, so it runs with the PROXY's own storage/balances/
///         msg.sender — this contract is NEVER meant to hold state or be used
///         standalone; a direct call to it (bypassing the proxy) only ever sees
///         its own permanently-empty storage, so every real check here
///         (onlyRole, proposal/request lookups, pool-exists checks) harmlessly
///         fails closed.
///
///         Inherits PulsarProtocolStorage (NOT PulsarProtocol) — same storage
///         layout guarantee, without pulling PulsarProtocol's own logic (and
///         bytecode) back in. A few small internal helpers duplicated below
///         (pool creation, tiny existence checks, active-custodian bookkeeping)
///         exist ONLY in PulsarProtocol as `internal` functions, so they can't
///         be called cross-contract — cheap to duplicate here since this
///         contract's own size doesn't count against PulsarProtocol's limit.
contract PulsarProtocolOps is PulsarProtocolStorage {
    using SafeERC20 for IERC20;

    event MintApproved(uint256 indexed proposalId, address indexed approver, uint8 approvalCount);
    event MintRejectionVoted(uint256 indexed proposalId, address indexed custodian, uint8 rejectCount);
    event MintRejected(uint256 indexed proposalId, address indexed rejectInitiator);
    event RedeemApproved(uint256 indexed requestId, address indexed custodian, uint8 approvalCount);
    event RedeemRejectionVoted(uint256 indexed requestId, address indexed custodian, uint8 rejectCount);
    event RedeemExecuted(uint256 indexed requestId, address indexed initiator);
    event RedeemRejected(uint256 indexed requestId, address indexed initiator);
    event TokensRedeemed(string indexed ticker, address indexed from, uint256 amount);
    event TreasuryUpdated(address indexed treasury);
    event RedeemFeeBpsUpdated(uint256 feeBps);
    event SwapFeeBpsUpdated(uint256 feeBps);
    event MinimumDistributionThresholdUpdated(uint256 threshold);
    event FeesDistributed(uint256 treasuryAmount, uint256 custodianAmount, uint256 recipientCount);
    event RouterUpdated(address indexed router);
    event IDRXUpdated(address indexed idrx);
    event KYCApproved(address indexed wallet);
    event KYCRevoked(address indexed wallet);
    event V4PoolCreated(string indexed ticker, uint160 sqrtPriceX96);
    event V4LiquidityAdded(string indexed ticker, uint256 idrxAmount, uint256 stockAmount);
    event V4EmergencyWithdrawn(string indexed ticker, uint256 idrxRecovered, uint256 stockRecovered);

    // Never meant to be upgraded independently — this contract is a static
    // delegatecall target, redeployed (new address, re-wired via
    // setOpsContract) rather than upgraded in place.
    function _authorizeUpgrade(address) internal pure override {
        revert("PulsarProtocolOps: not upgradeable");
    }

    // ─── Mint approve/reject flow ───────────────────────────────────────────

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

        uint256 funded = mintLiquidityFunding[proposalId];
        if (funded > 0) {
            mintLiquidityFunding[proposalId] = 0;
            IERC20(idrx).safeTransfer(proposal.requester, funded);
        }

        emit MintRejected(proposalId, msg.sender);
    }

    // ─── Redeem approve/reject/execute flow ─────────────────────────────────

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

    // ─── V4 liquidity management (rare custodian ops) ───────────────────────

    /// @dev Duplicated from PulsarProtocol._createV4Pool (internal there, so it
    ///      can't be called cross-contract). poolManager.initialize() is a
    ///      synchronous call with no callback routing back to the proxy, unlike
    ///      poolManager.unlock() below, which IS reused for free.
    function _createV4Pool(string memory ticker, uint160 sqrtPriceX96, int24 tickSpacing, uint24 lpFee) internal {
        if (address(poolManager) == address(0) || swapHook == address(0)) revert V4NotConfigured();
        address stockAddress = stocks[ticker];
        if (stockAddress == address(0)) revert StockNotFound(ticker);
        if (address(poolKeys[ticker].hooks) != address(0)) revert V4PoolExists(ticker);

        (Currency c0, Currency c1) =
            idrx < stockAddress ? (Currency.wrap(idrx), Currency.wrap(stockAddress)) : (Currency.wrap(stockAddress), Currency.wrap(idrx));
        PoolKey memory key =
            PoolKey({currency0: c0, currency1: c1, fee: lpFee, tickSpacing: tickSpacing, hooks: IHooks(swapHook)});
        poolKeys[ticker] = key;
        emit V4PoolCreated(ticker, sqrtPriceX96);
        poolManager.initialize(key, sqrtPriceX96);
        IPulsarSwapHookControl(swapHook).registerPool(key, ticker);
    }

    /// @dev poolManager.unlock() routes its callback to `address(this)` (this
    ///      proxy) regardless of whether the caller executed directly or via
    ///      this delegatecalled code — so PulsarProtocol's own unlockCallback /
    ///      _v4AddLiquidity dispatch is reused for free here, no duplication.
    function _provideV4Liquidity(string memory ticker, uint256 idrxAmount, uint256 stockAmount) internal {
        if (address(poolKeys[ticker].hooks) == address(0)) revert V4PoolNotFound(ticker);
        poolManager.unlock(
            abi.encode(
                PulsarProtocol.V4CallbackData({
                    action: PulsarProtocol.V4Action.ADD_LIQUIDITY,
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
    ///         protocol. Requires this protocol to hold PAUSER_ROLE on the hook.
    function emergencyWithdrawV4(string calldata ticker) external onlyRole(CUSTODIAN_ROLE) {
        if (address(poolKeys[ticker].hooks) == address(0)) revert V4PoolNotFound(ticker);
        IPulsarSwapHookControl(swapHook).pause();

        uint256 idrxBefore = IERC20(idrx).balanceOf(address(this));
        uint256 stockBefore = IERC20(stocks[ticker]).balanceOf(address(this));

        poolManager.unlock(
            abi.encode(
                PulsarProtocol.V4CallbackData({
                    action: PulsarProtocol.V4Action.REMOVE_ALL_LIQUIDITY,
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

    // ─── Fee distribution ────────────────────────────────────────────────────

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

    // ─── KYC management ──────────────────────────────────────────────────────

    function approveKYC(address wallet) external onlyRole(CUSTODIAN_ROLE) {
        kycApproved[wallet] = true;
        emit KYCApproved(wallet);
    }

    function revokeKYC(address wallet) external onlyRole(CUSTODIAN_ROLE) {
        kycApproved[wallet] = false;
        emit KYCRevoked(wallet);
    }

    // ─── Admin config ────────────────────────────────────────────────────────

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

    function _markActiveCustodian(address custodian) internal {
        if (!isActiveCustodian[custodian]) {
            isActiveCustodian[custodian] = true;
            _activeCustodians.push(custodian);
        }
    }
}
