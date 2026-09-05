// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {AccessControl} from "@openzeppelin/contracts/access/AccessControl.sol";
import {ReentrancyGuard} from "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import {IERC20} from "@openzeppelin/contracts/token/ERC20/IERC20.sol";
import {SafeERC20} from "@openzeppelin/contracts/token/ERC20/utils/SafeERC20.sol";

interface IPulsarProtocolForAgent {
    function stocks(string calldata ticker) external view returns (address);
    function idrx() external view returns (address);
    function swapV4(string calldata ticker, uint256 amountIn, uint256 minOut, bool buyStock) external;
}

/// @notice Ground-up rebuild — not a migration of the prior sell-only,
/// single-purpose ledger. Task (identity, lifecycle status, readable
/// summary, prompt fingerprint) is fully separate from TradePermission
/// (budget ceiling, expiry — granted later, only once a trade is actually
/// warranted), which is itself separate from a Trade (an actual execution,
/// a real queryable record). Every Task, Sub Task, and Trade is real
/// on-chain storage with a short human-readable summary, not just a hash —
/// see docs/plans/agent-task-manager-rebuild.md §3a/§8 for the full
/// reasoning and gas-cost verification behind that choice.
contract AgentTaskManager is AccessControl, ReentrancyGuard {
    using SafeERC20 for IERC20;

    bytes32 public constant AGENT_ROLE = keccak256("AGENT_ROLE");

    enum TaskAgent {
        Supervisor,
        Analyzer,
        Executor
    }

    enum SubTaskStatus {
        Done,
        Failed,
        NeedsInput
    }

    // A trade's direction is a classification, not a configuration toggle —
    // matches the off-chain stock_transactions.side convention (a
    // categorical value, not a boolean), and the standard practice of real
    // trading systems (e.g. FIX protocol's numeric Side field) of coding
    // direction as an enumerable category rather than a binary flag.
    enum TradeSide {
        Buy,
        Sell
    }

    // A Task's own lifecycle — applies to every Task, informational or
    // actionable. Not a money concept, so it lives on Task itself, not on
    // TradePermission. Only two states are ever actually stored: nothing
    // on-chain automatically transitions a Task when time passes or a
    // budget runs out, so "expired" and "budget exhausted" are derived at
    // read time in isActive(), never written as a status value.
    enum TaskStatus {
        Active,
        Cancelled
    }

    // Every Task, informational or actionable: an identity, a lifecycle
    // status, and a readable fingerprint of the request. Never a
    // money-related number.
    struct Task {
        address owner;
        bool isActionable;
        TaskStatus status;
        string summary; // short, AI-compiled, human-readable — e.g.
            // "Sell 20% of BRPTP if MSCI sentiment turns negative"
        bytes32 promptHash; // keccak256 of the full raw prompt
    }

    // Budget and expiry: always created together, in the same transaction
    // that arms a Task, so they are one struct — not four separate
    // mappings, which would prevent the compiler from packing these fields
    // into shared storage slots for no benefit. Still a *separate* mapping
    // from Task, because the relationship is optional: an informational
    // Task never has one of these.
    struct TradePermission {
        uint256 totalBudget; // IDRX-equivalent value ceiling
        uint256 usedBudget; // IDRX-equivalent value consumed so far
        uint64 expiresAt;
    }

    // Real storage, not events-only — retrievable by anyone, forever, with
    // a plain call, no indexer required. subTaskIdsByTaskId gives on-chain
    // discovery of which Sub Tasks belong to a Task, without needing to
    // scan event logs.
    struct SubTaskRecord {
        uint256 taskId;
        TaskAgent agent;
        string stepName; // e.g. "recognize_request", "gather_evidence", "decide"
        SubTaskStatus status;
        string routingTarget; // e.g. "executor", "" when terminal
        string summary; // short, human-readable conclusion of this step
        bytes32 reasoningHash; // keccak256 of the full off-chain reasoning text
        bytes32 outputHash; // keccak256 of the full off-chain output JSON
        bytes32 decisionHash;
        bytes32 previousDecisionHash;
    }

    // Also real storage — a Trade is what actually happened, the single
    // most consequential record in the whole system, and the one most in
    // need of being trivially findable without special tooling.
    struct TradeRecord {
        uint256 taskId;
        uint256 subTaskId; // the "decide" Sub Task that authorized this fill
        string ticker;
        TradeSide side;
        uint256 amount;
        uint256 receivedAmount;
        string summary; // short, human-readable reason for this fill
        bytes32 reasoningHash;
        uint256 executedAt;
    }

    IPulsarProtocolForAgent public immutable protocol;
    IERC20 public immutable idrx;

    mapping(uint256 taskId => Task) public tasks;
    uint256 public taskCount;

    mapping(uint256 taskId => TradePermission) public tradePermissions;

    mapping(uint256 subTaskId => SubTaskRecord) public subTasks;
    mapping(uint256 taskId => uint256[] subTaskIds) public subTaskIdsByTaskId;
    uint256 public subTaskCount;

    mapping(uint256 tradeId => TradeRecord) public trades;
    mapping(uint256 taskId => uint256[] tradeIds) public tradeIdsByTaskId;
    uint256 public tradeCount;

    event TaskCreated(uint256 indexed taskId, address indexed owner, bool isActionable, string summary);
    event TradePermissionGranted(uint256 indexed taskId, uint256 totalBudget, uint64 expiresAt);
    event SubTaskRecorded(
        uint256 indexed taskId, uint256 subTaskId, TaskAgent indexed agent, string stepName, string summary
    );
    event TradeExecuted(
        uint256 indexed taskId, uint256 indexed tradeId, string ticker, TradeSide side, uint256 amount, string summary
    );
    event TaskCancelled(uint256 indexed taskId, address indexed cancelledBy);

    error ZeroAddress();
    error TaskNotFound(uint256 taskId);
    error TaskNotActive(uint256 taskId);
    error TaskNotActionable(uint256 taskId);
    error TradePermissionAlreadyGranted(uint256 taskId);
    error NoTradePermission(uint256 taskId);
    error TradePermissionExpired(uint256 taskId, uint64 expiresAt);
    error BudgetExceeded(uint256 amount, uint256 remaining);
    error AllowanceExceeded(uint256 amount, uint256 allowance);
    error SubTaskNotFound(uint256 subTaskId);
    error SubTaskTaskMismatch(uint256 subTaskId, uint256 expectedTaskId);
    error NotOwnerOrAgent(uint256 taskId, address caller);

    constructor(address protocol_, address admin) {
        if (protocol_ == address(0) || admin == address(0)) revert ZeroAddress();
        protocol = IPulsarProtocolForAgent(protocol_);
        idrx = IERC20(protocol.idrx());
        _grantRole(DEFAULT_ADMIN_ROLE, admin);
    }

    // Always AGENT_ROLE — Task creation is bookkeeping, never a custody
    // action, so there is no reason to require the owner's own signature
    // here. isActionable is already known by this point: Supervisor's
    // chat-intake flow has already established whether this request has a
    // trigger condition at all before this is ever called.
    function createTask(address owner, bool isActionable, string calldata summary, bytes32 promptHash)
        external
        onlyRole(AGENT_ROLE)
        returns (uint256 taskId)
    {
        if (owner == address(0)) revert ZeroAddress();
        taskId = taskCount++;
        tasks[taskId] = Task({
            owner: owner,
            isActionable: isActionable,
            status: TaskStatus.Active,
            summary: summary,
            promptHash: promptHash
        });
        emit TaskCreated(taskId, owner, isActionable, summary);
    }

    // Called only once Executor concludes a trade is actually warranted for
    // this Task — not reserved speculatively at creation time. Still
    // bookkeeping only: this records what was agreed off-chain, it does
    // not move or reserve any funds itself. The real custody boundary is
    // the owner's own separate ERC20 approve() to this contract's address,
    // checked live in executeTrade below, independent of this record.
    function grantTradePermission(uint256 taskId, uint256 totalBudget, uint256 duration)
        external
        onlyRole(AGENT_ROLE)
    {
        Task storage task = tasks[taskId];
        if (task.owner == address(0)) revert TaskNotFound(taskId);
        if (!task.isActionable) revert TaskNotActionable(taskId);
        if (tradePermissions[taskId].expiresAt != 0) revert TradePermissionAlreadyGranted(taskId);

        uint64 expiresAt = uint64(block.timestamp + duration);
        tradePermissions[taskId] = TradePermission({totalBudget: totalBudget, usedBudget: 0, expiresAt: expiresAt});
        emit TradePermissionGranted(taskId, totalBudget, expiresAt);
    }

    // Batched into one transaction so the chain's base fee is paid once per
    // run instead of once per row. Each record is real storage (queryable
    // by anyone, forever) and also emits an event for real-time listeners.
    // A batch that reverts or never lands leaves its rows unrecorded here;
    // the caller (backend) retries with the exact same rows, same order —
    // see docs/plans/agent-task-manager-code-implementation.md §3.7.
    function recordSubTasks(uint256 taskId, SubTaskRecord[] calldata subTaskRecords)
        external
        onlyRole(AGENT_ROLE)
        returns (uint256[] memory subTaskIds)
    {
        if (tasks[taskId].owner == address(0)) revert TaskNotFound(taskId);

        subTaskIds = new uint256[](subTaskRecords.length);
        for (uint256 i = 0; i < subTaskRecords.length; i++) {
            uint256 subTaskId = subTaskCount++;
            subTasks[subTaskId] = subTaskRecords[i];
            subTaskIdsByTaskId[taskId].push(subTaskId);
            subTaskIds[i] = subTaskId;
            emit SubTaskRecorded(
                taskId, subTaskId, subTaskRecords[i].agent, subTaskRecords[i].stepName, subTaskRecords[i].summary
            );
        }
    }

    // Renamed from executeTask: every parameter here is trade-specific, and
    // an informational Task never reaches this function at all. token is
    // the ERC20 being pulled from the owner (the stock token for a sell,
    // IDRX for a buy) — its live allowance to this contract's own address
    // is the real custody boundary; the recorded TradePermission is the
    // bookkeeping ceiling. Both must agree before any transfer happens.
    // subTaskId must be an already-recorded "decide" row belonging to this
    // same Task — the caller passes it explicitly rather than this
    // function guessing "the last row recorded", since a batch can contain
    // rows from more than one step.
    function executeTrade(
        uint256 taskId,
        uint256 subTaskId,
        address token,
        string calldata ticker,
        TradeSide side,
        uint256 amount,
        uint256 minimumOutputAmount,
        string calldata summary,
        bytes32 reasoningHash
    ) external onlyRole(AGENT_ROLE) nonReentrant returns (uint256 tradeId) {
        Task storage task = tasks[taskId];
        if (task.owner == address(0)) revert TaskNotFound(taskId);
        if (task.status != TaskStatus.Active) revert TaskNotActive(taskId);

        // subTaskId < subTaskCount is the real existence check — taskId
        // equality alone is not enough, since an unrecorded subTaskId's
        // default SubTaskRecord.taskId is 0, which would wrongly match a
        // genuine Task 0.
        if (subTaskId >= subTaskCount) revert SubTaskNotFound(subTaskId);
        SubTaskRecord storage decideRow = subTasks[subTaskId];
        if (decideRow.taskId != taskId) revert SubTaskTaskMismatch(subTaskId, taskId);

        TradePermission storage tradePermission = tradePermissions[taskId];
        if (tradePermission.expiresAt == 0) revert NoTradePermission(taskId);
        if (block.timestamp > tradePermission.expiresAt) {
            revert TradePermissionExpired(taskId, tradePermission.expiresAt);
        }

        uint256 remainingRecordedBudget = tradePermission.totalBudget - tradePermission.usedBudget;
        if (amount == 0 || amount > remainingRecordedBudget) revert BudgetExceeded(amount, remainingRecordedBudget);

        uint256 currentLiveAllowance = IERC20(token).allowance(task.owner, address(this));
        if (amount > currentLiveAllowance) revert AllowanceExceeded(amount, currentLiveAllowance);

        uint256 receivedAmount;
        if (side == TradeSide.Sell) {
            IERC20(token).safeTransferFrom(task.owner, address(this), amount);
            IERC20(token).forceApprove(address(protocol), amount);

            uint256 idrxBefore = idrx.balanceOf(address(this));
            protocol.swapV4(ticker, amount, minimumOutputAmount, false);
            receivedAmount = idrx.balanceOf(address(this)) - idrxBefore;

            // IDRX-denominated budget only known after the swap completes —
            // this update happens after an external call, which is exactly
            // the ordering nonReentrant exists to make safe.
            tradePermission.usedBudget += receivedAmount;
            idrx.safeTransfer(task.owner, receivedAmount);
        } else {
            idrx.safeTransferFrom(task.owner, address(this), amount);
            idrx.forceApprove(address(protocol), amount);

            address stockToken = protocol.stocks(ticker);
            uint256 stockBefore = IERC20(stockToken).balanceOf(address(this));
            protocol.swapV4(ticker, amount, minimumOutputAmount, true);
            receivedAmount = IERC20(stockToken).balanceOf(address(this)) - stockBefore;

            tradePermission.usedBudget += amount; // IDRX amount known immediately for a buy
            IERC20(stockToken).safeTransfer(task.owner, receivedAmount);
        }

        tradeId = tradeCount++;
        trades[tradeId] = TradeRecord({
            taskId: taskId,
            subTaskId: subTaskId,
            ticker: ticker,
            side: side,
            amount: amount,
            receivedAmount: receivedAmount,
            summary: summary,
            reasoningHash: reasoningHash,
            executedAt: block.timestamp
        });
        tradeIdsByTaskId[taskId].push(tradeId);

        emit TradeExecuted(taskId, tradeId, ticker, side, amount, summary);
    }

    // Cancel is risk-reducing — it only ever removes a future possibility,
    // never moves funds — so, unlike executeTrade, it's safe to let either
    // the owner or AGENT_ROLE call it. Sets Task.status to Cancelled;
    // isActive() then returns false for this Task regardless of any
    // TradePermission headroom remaining.
    function cancelTask(uint256 taskId) external {
        Task storage task = tasks[taskId];
        if (task.owner == address(0)) revert TaskNotFound(taskId);
        if (task.status != TaskStatus.Active) revert TaskNotActive(taskId);
        if (msg.sender != task.owner && !hasRole(AGENT_ROLE, msg.sender)) {
            revert NotOwnerOrAgent(taskId, msg.sender);
        }
        task.status = TaskStatus.Cancelled;
        emit TaskCancelled(taskId, msg.sender);
    }

    function isActive(uint256 taskId) external view returns (bool) {
        Task storage task = tasks[taskId];
        if (task.owner == address(0)) return false;
        if (task.status != TaskStatus.Active) return false;
        if (!task.isActionable) return true;

        TradePermission storage tradePermission = tradePermissions[taskId];
        if (tradePermission.expiresAt == 0) return false; // actionable but no permission granted yet
        return block.timestamp <= tradePermission.expiresAt && tradePermission.usedBudget < tradePermission.totalBudget;
    }

    function subTaskIdsForTask(uint256 taskId) external view returns (uint256[] memory) {
        return subTaskIdsByTaskId[taskId];
    }

    function tradeIdsForTask(uint256 taskId) external view returns (uint256[] memory) {
        return tradeIdsByTaskId[taskId];
    }
}
