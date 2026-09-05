// SPDX-License-Identifier: MIT
pragma solidity ^0.8.25;

import {Test, console} from "forge-std/Test.sol";
import {AgentTaskManager} from "../src/AgentTaskManager.sol";
import {PulsarProtocol} from "../src/PulsarProtocol.sol";

/// @notice Fork test against the REAL live Arbitrum Sepolia proxy — proves
///         the full Task -> TradePermission -> recordSubTasks ->
///         executeTrade lifecycle against the exact pools seeded in this
///         session (BBCAP), not a local mock. Rewritten from scratch for
///         the ground-up rebuild (docs/plans/agent-task-manager-rebuild.md
///         §3a) — not a patch of the prior sell-only, single-Trade design.
contract AgentTaskManagerForkTest is Test {
    address constant PROXY = 0x204488318C0E75978B3c851382Aa83f3065a8f5A;
    address constant ADMIN = 0x5617007a51331f4e7241DE852cDc4Bbbe723CAE4;

    AgentTaskManager manager;
    PulsarProtocol protocol;
    address idrxAddr;
    address agent;
    bool forked;

    function setUp() public {
        string memory rpc = vm.envOr("RPC_URL", string(""));
        if (bytes(rpc).length == 0) return;
        vm.createSelectFork(rpc);
        forked = true;

        protocol = PulsarProtocol(PROXY);
        idrxAddr = protocol.idrx();
        manager = new AgentTaskManager(PROXY, ADMIN);

        agent = makeAddr("agentWallet");
        bytes32 agentRole = manager.AGENT_ROLE();
        vm.prank(ADMIN);
        manager.grantRole(agentRole, agent);
    }

    function _mintIdrx(address to, uint256 amount) internal {
        vm.prank(ADMIN);
        (bool ok,) = idrxAddr.call(abi.encodeWithSignature("mint(address,uint256)", to, amount));
        require(ok, "idrx mint failed");
    }

    function _buyRealBBCAP(address user, uint256 idrxIn) internal returns (uint256 stockBalance, address stockAddr) {
        stockAddr = protocol.stocks("BBCAP");
        assertTrue(stockAddr != address(0), "BBCAP must already have a deployed stock");

        _mintIdrx(user, idrxIn);
        vm.prank(user);
        (bool ok,) = idrxAddr.call(abi.encodeWithSignature("approve(address,uint256)", PROXY, idrxIn));
        require(ok, "idrx approve failed");
        vm.prank(user);
        protocol.swapV4("BBCAP", idrxIn, 0, true);

        (bool ok2, bytes memory ret) = stockAddr.call(abi.encodeWithSignature("balanceOf(address)", user));
        require(ok2, "balanceOf failed");
        stockBalance = abi.decode(ret, (uint256));
    }

    // ── createTask ──────────────────────────────────────────────────────

    function test_createTask_actionableAndInformational_bothGetNoTradePermission() public {
        if (!forked) return;
        address owner = makeAddr("taskOwner1");

        vm.prank(agent);
        uint256 actionableId = manager.createTask(owner, true, "Sell 20% BRPTP if MSCI turns negative", keccak256("prompt1"));
        vm.prank(agent);
        uint256 infoId = manager.createTask(owner, false, "Why did BRPT drop yesterday", keccak256("prompt2"));

        (address ownerA, bool isActionableA, AgentTaskManager.TaskStatus statusA,,) = manager.tasks(actionableId);
        assertEq(ownerA, owner);
        assertTrue(isActionableA);
        assertEq(uint256(statusA), uint256(AgentTaskManager.TaskStatus.Active));

        (,, AgentTaskManager.TaskStatus statusI,,) = manager.tasks(infoId);
        assertEq(uint256(statusI), uint256(AgentTaskManager.TaskStatus.Active));

        (,, uint64 expiresAtA) = manager.tradePermissions(actionableId);
        (,, uint64 expiresAtI) = manager.tradePermissions(infoId);
        assertEq(expiresAtA, 0, "no TradePermission until explicitly granted, even for an actionable Task");
        assertEq(expiresAtI, 0, "informational Task never gets a TradePermission");

        assertTrue(manager.isActive(infoId), "informational Task is active with no permission at all");
    }

    function test_createTask_onlyAgentRole() public {
        if (!forked) return;
        vm.expectRevert();
        manager.createTask(makeAddr("someone"), true, "x", bytes32(0));
    }

    // ── grantTradePermission ────────────────────────────────────────────

    function test_grantTradePermission_revertsForUnknownTask() public {
        if (!forked) return;
        vm.prank(agent);
        vm.expectRevert(abi.encodeWithSelector(AgentTaskManager.TaskNotFound.selector, 999));
        manager.grantTradePermission(999, 1_000_000, 7 days);
    }

    function test_grantTradePermission_revertsForInformationalTask() public {
        if (!forked) return;
        address owner = makeAddr("taskOwner2");
        vm.prank(agent);
        uint256 taskId = manager.createTask(owner, false, "info only", bytes32(0));

        vm.prank(agent);
        vm.expectRevert(abi.encodeWithSelector(AgentTaskManager.TaskNotActionable.selector, taskId));
        manager.grantTradePermission(taskId, 1_000_000, 7 days);
    }

    function test_grantTradePermission_revertsIfAlreadyGranted() public {
        if (!forked) return;
        address owner = makeAddr("taskOwner3");
        vm.prank(agent);
        uint256 taskId = manager.createTask(owner, true, "sell some", bytes32(0));

        vm.prank(agent);
        manager.grantTradePermission(taskId, 1_000_000, 7 days);

        vm.prank(agent);
        vm.expectRevert(abi.encodeWithSelector(AgentTaskManager.TradePermissionAlreadyGranted.selector, taskId));
        manager.grantTradePermission(taskId, 2_000_000, 7 days);
    }

    // ── recordSubTasks ───────────────────────────────────────────────────

    function test_recordSubTasks_writesRowsAndAppendsIds() public {
        if (!forked) return;
        address owner = makeAddr("taskOwner4");
        vm.prank(agent);
        uint256 taskId = manager.createTask(owner, false, "info", bytes32(0));

        AgentTaskManager.SubTaskRecord[] memory rows = new AgentTaskManager.SubTaskRecord[](2);
        rows[0] = AgentTaskManager.SubTaskRecord({
            taskId: taskId,
            agent: AgentTaskManager.TaskAgent.Supervisor,
            stepName: "recognize_request",
            status: AgentTaskManager.SubTaskStatus.Done,
            routingTarget: "analyzer",
            summary: "Recognized as informational",
            reasoningHash: keccak256("r1"),
            outputHash: keccak256("o1"),
            decisionHash: keccak256("d1"),
            previousDecisionHash: bytes32(0)
        });
        rows[1] = AgentTaskManager.SubTaskRecord({
            taskId: taskId,
            agent: AgentTaskManager.TaskAgent.Analyzer,
            stepName: "gather_evidence",
            status: AgentTaskManager.SubTaskStatus.Done,
            routingTarget: "",
            summary: "Gathered and concluded",
            reasoningHash: keccak256("r2"),
            outputHash: keccak256("o2"),
            decisionHash: keccak256("d2"),
            previousDecisionHash: keccak256("d1")
        });

        vm.prank(agent);
        uint256[] memory ids = manager.recordSubTasks(taskId, rows);
        assertEq(ids.length, 2);

        uint256[] memory idsForTask = manager.subTaskIdsForTask(taskId);
        assertEq(idsForTask.length, 2);
        assertEq(idsForTask[0], ids[0]);
        assertEq(idsForTask[1], ids[1]);

        (uint256 storedTaskId,,, AgentTaskManager.SubTaskStatus storedStatus,, string memory storedSummary,,,,) =
            manager.subTasks(ids[1]);
        assertEq(storedTaskId, taskId);
        assertEq(uint256(storedStatus), uint256(AgentTaskManager.SubTaskStatus.Done));
        assertEq(storedSummary, "Gathered and concluded");
    }

    function test_recordSubTasks_revertsForUnknownTask() public {
        if (!forked) return;
        AgentTaskManager.SubTaskRecord[] memory rows = new AgentTaskManager.SubTaskRecord[](0);
        vm.prank(agent);
        vm.expectRevert(abi.encodeWithSelector(AgentTaskManager.TaskNotFound.selector, 999));
        manager.recordSubTasks(999, rows);
    }

    // ── executeTrade ─────────────────────────────────────────────────────

    function _armAndRecordDecide(address owner, uint256 totalBudget)
        internal
        returns (uint256 taskId, uint256 decideSubTaskId)
    {
        vm.prank(agent);
        taskId = manager.createTask(owner, true, "sell some BBCAP on trigger", bytes32(0));
        vm.prank(agent);
        manager.grantTradePermission(taskId, totalBudget, 7 days);

        AgentTaskManager.SubTaskRecord[] memory rows = new AgentTaskManager.SubTaskRecord[](1);
        rows[0] = AgentTaskManager.SubTaskRecord({
            taskId: taskId,
            agent: AgentTaskManager.TaskAgent.Executor,
            stepName: "decide",
            status: AgentTaskManager.SubTaskStatus.Done,
            routingTarget: "",
            summary: "Confirmed negative sentiment, sizing full position",
            reasoningHash: keccak256("decide-reasoning"),
            outputHash: keccak256("decide-output"),
            decisionHash: keccak256("decide-hash"),
            previousDecisionHash: bytes32(0)
        });
        vm.prank(agent);
        uint256[] memory ids = manager.recordSubTasks(taskId, rows);
        decideSubTaskId = ids[0];
    }

    function test_executeTrade_sellFlow_succeedsAndUpdatesBudget() public {
        if (!forked) return;
        address owner = makeAddr("sellOwner1");
        (uint256 stockBalance, address stockAddr) = _buyRealBBCAP(owner, 50_000);
        assertGt(stockBalance, 0);

        (uint256 taskId, uint256 decideSubTaskId) = _armAndRecordDecide(owner, type(uint128).max);

        vm.prank(owner);
        (bool ok,) = stockAddr.call(abi.encodeWithSignature("approve(address,uint256)", address(manager), stockBalance));
        require(ok, "stock approve failed");

        uint256 ownerIdrxBefore = _balanceOf(idrxAddr, owner);

        vm.prank(agent);
        uint256 tradeId = manager.executeTrade(
            taskId,
            decideSubTaskId,
            stockAddr,
            "BBCAP",
            AgentTaskManager.TradeSide.Sell,
            stockBalance,
            0,
            "Sold full position on confirmed negative sentiment",
            keccak256("exec-reasoning")
        );

        (, uint256 usedBudget,) = manager.tradePermissions(taskId);
        assertGt(usedBudget, 0, "usedBudget must increase by the realized IDRX proceeds");

        uint256 ownerIdrxAfter = _balanceOf(idrxAddr, owner);
        assertEq(ownerIdrxAfter - ownerIdrxBefore, usedBudget, "owner must receive exactly the realized IDRX proceeds");

        uint256[] memory tradeIds = manager.tradeIdsForTask(taskId);
        assertEq(tradeIds.length, 1);
        assertEq(tradeIds[0], tradeId);

        console.log("SUCCESS: sell executeTrade realized IDRX proceeds and updated usedBudget", usedBudget);
    }

    function test_executeTrade_buyFlow_succeeds() public {
        if (!forked) return;
        address owner = makeAddr("buyOwner1");
        uint256 idrxAmount = 20_000;
        _mintIdrx(owner, idrxAmount);

        (uint256 taskId, uint256 decideSubTaskId) = _armAndRecordDecide(owner, idrxAmount);

        vm.prank(owner);
        (bool ok,) = idrxAddr.call(abi.encodeWithSignature("approve(address,uint256)", address(manager), idrxAmount));
        require(ok, "idrx approve failed");

        address stockAddr = protocol.stocks("BBCAP");
        uint256 ownerStockBefore = _balanceOf(stockAddr, owner);

        vm.prank(agent);
        manager.executeTrade(
            taskId,
            decideSubTaskId,
            idrxAddr,
            "BBCAP",
            AgentTaskManager.TradeSide.Buy,
            idrxAmount,
            0,
            "Bought full budget on confirmed positive sentiment",
            keccak256("exec-reasoning-buy")
        );

        uint256 ownerStockAfter = _balanceOf(stockAddr, owner);
        assertGt(ownerStockAfter, ownerStockBefore, "owner must receive stock tokens from the buy");

        (, uint256 usedBudget,) = manager.tradePermissions(taskId);
        assertEq(usedBudget, idrxAmount, "buy path: usedBudget equals the IDRX amount spent, known immediately");
    }

    function test_executeTrade_revertsWhenExceedingTradePermission() public {
        if (!forked) return;
        address owner = makeAddr("sellOwner2");
        (uint256 stockBalance, address stockAddr) = _buyRealBBCAP(owner, 50_000);

        // Budget of 1 wei of IDRX — any realistic sell proceeds exceed it.
        (uint256 taskId, uint256 decideSubTaskId) = _armAndRecordDecide(owner, 1);

        vm.prank(owner);
        (bool ok,) = stockAddr.call(abi.encodeWithSignature("approve(address,uint256)", address(manager), stockBalance));
        require(ok, "stock approve failed");

        vm.prank(agent);
        vm.expectRevert(); // BudgetExceeded — exact remaining depends on live price, checked structurally not by value
        manager.executeTrade(
            taskId, decideSubTaskId, stockAddr, "BBCAP", AgentTaskManager.TradeSide.Sell, stockBalance, 0, "x", bytes32(0)
        );
    }

    function test_executeTrade_revertsWhenExceedingLiveAllowance() public {
        if (!forked) return;
        address owner = makeAddr("sellOwner3");
        (uint256 stockBalance, address stockAddr) = _buyRealBBCAP(owner, 50_000);
        (uint256 taskId, uint256 decideSubTaskId) = _armAndRecordDecide(owner, type(uint128).max);

        // Deliberately no approve() at all — live allowance is 0.
        vm.prank(agent);
        vm.expectRevert(abi.encodeWithSelector(AgentTaskManager.AllowanceExceeded.selector, stockBalance, 0));
        manager.executeTrade(
            taskId, decideSubTaskId, stockAddr, "BBCAP", AgentTaskManager.TradeSide.Sell, stockBalance, 0, "x", bytes32(0)
        );
    }

    function test_executeTrade_revertsWhenNotArmed() public {
        if (!forked) return;
        address owner = makeAddr("taskOwner5");
        vm.prank(agent);
        uint256 taskId = manager.createTask(owner, true, "not armed yet", bytes32(0));

        // A real subTaskId is required to isolate "no TradePermission" from
        // "unknown subTaskId" — recordSubTasks does not itself require a
        // TradePermission to exist, so this is a valid state to construct.
        AgentTaskManager.SubTaskRecord[] memory rows = new AgentTaskManager.SubTaskRecord[](1);
        rows[0] = AgentTaskManager.SubTaskRecord({
            taskId: taskId,
            agent: AgentTaskManager.TaskAgent.Executor,
            stepName: "decide",
            status: AgentTaskManager.SubTaskStatus.Done,
            routingTarget: "",
            summary: "decide",
            reasoningHash: bytes32(0),
            outputHash: bytes32(0),
            decisionHash: bytes32(0),
            previousDecisionHash: bytes32(0)
        });
        vm.prank(agent);
        uint256[] memory ids = manager.recordSubTasks(taskId, rows);

        vm.prank(agent);
        vm.expectRevert(abi.encodeWithSelector(AgentTaskManager.NoTradePermission.selector, taskId));
        manager.executeTrade(taskId, ids[0], address(0), "BBCAP", AgentTaskManager.TradeSide.Sell, 1, 0, "x", bytes32(0));
    }

    function test_executeTrade_revertsOnSubTaskTaskMismatch() public {
        if (!forked) return;
        address owner = makeAddr("sellOwner4");
        (, address stockAddr) = _buyRealBBCAP(owner, 50_000);
        (uint256 taskId,) = _armAndRecordDecide(owner, type(uint128).max);

        // decideSubTaskId from a DIFFERENT task.
        (, uint256 otherDecideSubTaskId) = _armAndRecordDecide(makeAddr("sellOwner4b"), type(uint128).max);

        vm.prank(agent);
        vm.expectRevert(
            abi.encodeWithSelector(AgentTaskManager.SubTaskTaskMismatch.selector, otherDecideSubTaskId, taskId)
        );
        manager.executeTrade(
            taskId, otherDecideSubTaskId, stockAddr, "BBCAP", AgentTaskManager.TradeSide.Sell, 1, 0, "x", bytes32(0)
        );
    }

    function test_executeTrade_revertsAfterExpiry() public {
        if (!forked) return;
        address owner = makeAddr("sellOwner5");
        (uint256 stockBalance, address stockAddr) = _buyRealBBCAP(owner, 50_000);

        vm.prank(agent);
        uint256 taskId = manager.createTask(owner, true, "short window", bytes32(0));
        vm.prank(agent);
        manager.grantTradePermission(taskId, type(uint128).max, 1 days);

        AgentTaskManager.SubTaskRecord[] memory rows = new AgentTaskManager.SubTaskRecord[](1);
        rows[0] = AgentTaskManager.SubTaskRecord({
            taskId: taskId,
            agent: AgentTaskManager.TaskAgent.Executor,
            stepName: "decide",
            status: AgentTaskManager.SubTaskStatus.Done,
            routingTarget: "",
            summary: "decide",
            reasoningHash: bytes32(0),
            outputHash: bytes32(0),
            decisionHash: bytes32(0),
            previousDecisionHash: bytes32(0)
        });
        vm.prank(agent);
        uint256[] memory ids = manager.recordSubTasks(taskId, rows);

        vm.prank(owner);
        (bool ok,) = stockAddr.call(abi.encodeWithSignature("approve(address,uint256)", address(manager), stockBalance));
        require(ok);

        vm.warp(block.timestamp + 2 days);
        assertFalse(manager.isActive(taskId));

        vm.prank(agent);
        vm.expectRevert(abi.encodeWithSelector(AgentTaskManager.TradePermissionExpired.selector, taskId, uint64(block.timestamp - 1 days)));
        manager.executeTrade(
            taskId, ids[0], stockAddr, "BBCAP", AgentTaskManager.TradeSide.Sell, stockBalance, 0, "x", bytes32(0)
        );
    }

    // ── cancelTask ───────────────────────────────────────────────────────

    function test_cancelTask_byOwner_thenIsActiveFalse() public {
        if (!forked) return;
        address owner = makeAddr("taskOwner6");
        vm.prank(agent);
        uint256 taskId = manager.createTask(owner, false, "cancel me", bytes32(0));
        assertTrue(manager.isActive(taskId));

        vm.prank(owner);
        manager.cancelTask(taskId);
        assertFalse(manager.isActive(taskId));
    }

    function test_cancelTask_byAgent_alsoWorks() public {
        if (!forked) return;
        address owner = makeAddr("taskOwner7");
        vm.prank(agent);
        uint256 taskId = manager.createTask(owner, false, "cancel me too", bytes32(0));

        vm.prank(agent);
        manager.cancelTask(taskId);
        assertFalse(manager.isActive(taskId));
    }

    function test_cancelTask_revertsForThirdParty() public {
        if (!forked) return;
        address owner = makeAddr("taskOwner8");
        vm.prank(agent);
        uint256 taskId = manager.createTask(owner, false, "cannot be cancelled by a stranger", bytes32(0));

        address stranger = makeAddr("stranger");
        vm.prank(stranger);
        vm.expectRevert(abi.encodeWithSelector(AgentTaskManager.NotOwnerOrAgent.selector, taskId, stranger));
        manager.cancelTask(taskId);
    }

    function _balanceOf(address token, address account) internal returns (uint256) {
        (bool ok, bytes memory ret) = token.call(abi.encodeWithSignature("balanceOf(address)", account));
        require(ok, "balanceOf failed");
        return abi.decode(ret, (uint256));
    }
}
