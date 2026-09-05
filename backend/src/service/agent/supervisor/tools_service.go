package supervisor

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent"
)

type routeRequest struct {
	Request string `json:"request" jsonschema_description:"The instruction or context to hand to this node — Analyzer's trigger condition to evaluate, or Executor's confirmed conclusion/instruction to act on."`
}

type createTaskRequest struct {
	IsActionable bool   `json:"is_actionable" jsonschema_description:"true if this request has a real trigger condition to act on; false if purely informational — including a portfolio/chart lookup."`
	Summary      string `json:"summary" jsonschema_description:"A short, human-readable one-line summary of what's being asked, suitable for on-chain storage later — e.g. 'Sell 20% BRPT if MSCI sentiment turns negative.'"`
}

type createTaskResponse struct {
	TaskID int64 `json:"task_id"`
}

// newCreateTaskTool lets Supervisor itself recognize a new distinct
// request and open a Task for it — the off-chain agent_tasks row only,
// never the on-chain Task (that happens later, at Arm time, see
// docs/plans/agent-task-manager-code-implementation.md §7.C). Task = any
// request that needs data fetched or searched on the user's behalf
// (trading, portfolio/chart lookups, news search); NOT a Task = pure
// conversation with nothing to fetch (a greeting, small talk).
//
// This is the one tool call with no existing RunContext.Recorder to write
// through yet — it creates one, then binds it onto the shared RunContext
// pointer so every tool called afterward in this same run (analyzer_agent,
// executor_agent) picks up the same recorder and continues the same hash
// chain.
func newCreateTaskTool(tasks *repository.AgentTaskRepository, subTasks *repository.AgentSubTaskRepository) (tool.InvokableTool, error) {
	return utils.InferTool(
		"create_task",
		"Opens a new Task for a request you've just recognized as genuinely distinct — call this once, the first time you conclude this prompt isn't a continuation of an already-open Task in this chat, and isn't pure conversation with nothing to fetch.",
		func(ctx context.Context, req createTaskRequest) (createTaskResponse, error) {
			rc, ok := agent.RunContextFrom(ctx)
			if !ok {
				return createTaskResponse{}, fmt.Errorf("create_task: no run context bound to this call")
			}
			// Graceful no-op, not a hard error: a NodeRunError here would
			// abort the entire turn (observed live — the model sometimes
			// calls create_task again on a follow-up message in an
			// already-open Task despite the instruction above), wasting
			// every tool call already made this run for a mistake that is
			// actually harmless to recover from — the Task already exists,
			// so just keep using it.
			if rc.TaskID != 0 {
				return createTaskResponse{TaskID: rc.TaskID}, nil
			}

			task, err := tasks.Create(ctx, repository.AgentTaskCreateInput{
				WalletAddress:   rc.Wallet,
				SourceMessageID: rc.SourceMessageID,
				IsActionable:    req.IsActionable,
				RawPrompt:       &rc.TriggerDescription,
				Summary:         req.Summary,
			})
			if err != nil {
				return createTaskResponse{}, fmt.Errorf("create_task: persist row: %w", err)
			}

			recorder, err := agent.NewSubTaskRecorder(ctx, subTasks, task.ID, req.Summary, rc.Wallet)
			if err != nil {
				return createTaskResponse{}, fmt.Errorf("create_task: build recorder: %w", err)
			}
			rc.TaskID = task.ID
			rc.Recorder = recorder

			if _, err := rc.Recorder.Record(ctx, "supervisor", "recognize_request", "done", req.Summary, map[string]any{"is_actionable": req.IsActionable}); err != nil {
				return createTaskResponse{}, fmt.Errorf("create_task: record recognize_request: %w", err)
			}
			return createTaskResponse{TaskID: task.ID}, nil
		},
	)
}

// newAnalyzerTool wraps Analyzer as a callable tool for Supervisor. Unlike
// a bare adk.NewAgentTool, this records the route_to_analyzer and
// gather_evidence agent_sub_tasks rows itself, at the exact moment
// Supervisor's own reasoning decides to call it — the only way to keep the
// hash chain in true chronological order, since Supervisor's routing
// decision and Analyzer's call happen inside the same continuous run and
// a bare AgentTool gives no hook to record anything in between.
func newAnalyzerTool(analyzerAgent adk.Agent) (tool.InvokableTool, error) {
	return utils.InferTool(
		"analyzer_agent",
		"Gathers news and/or technical evidence for the current Task's trigger condition and concludes whether it is satisfied. Also handles portfolio/chart questions.",
		func(ctx context.Context, req routeRequest) (string, error) {
			rc, ok := agent.RunContextFrom(ctx)
			if !ok {
				return "", fmt.Errorf("analyzer_agent: no run context bound to this call")
			}
			if _, err := rc.Recorder.Record(ctx, "supervisor", "route_to_analyzer", "done", "Supervisor routed this prompt to Analyzer for evaluation.", nil); err != nil {
				return "", fmt.Errorf("analyzer_agent: record route_to_analyzer: %w", err)
			}

			conclusion, _, err := agent.RunAgentWithTrace(ctx, analyzerAgent, req.Request)
			if err != nil {
				return "", fmt.Errorf("analyzer_agent: run: %w", err)
			}

			if _, err := rc.Recorder.Record(ctx, "analyzer", "gather_evidence", "done", conclusion, nil); err != nil {
				return "", fmt.Errorf("analyzer_agent: record gather_evidence: %w", err)
			}
			return conclusion, nil
		},
	)
}

// newExecutorTool wraps Executor as a callable tool for Supervisor. Records
// route_to_executor before running Executor, so a subsequent submit_trade
// call (executor/tools_service.go), if Executor decides to act, chains
// correctly after this row. If Executor runs but never calls submit_trade,
// this records the hold itself — a hold needs no on-chain call at all, the
// "decide" SubTaskRecord already written here is the entire accountability
// trail (a dedicated logDecision/AgentDecisionLogged would only duplicate
// that same anchor a second time — removed from the contract entirely).
func newExecutorTool(executorAgent adk.Agent) (tool.InvokableTool, error) {
	return utils.InferTool(
		"executor_agent",
		"Decides the concrete action for the current Task (sell, buy, or hold) and submits it on-chain when it decides to act.",
		func(ctx context.Context, req routeRequest) (string, error) {
			rc, ok := agent.RunContextFrom(ctx)
			if !ok {
				return "", fmt.Errorf("executor_agent: no run context bound to this call")
			}
			if _, err := rc.Recorder.Record(ctx, "supervisor", "route_to_executor", "done", "Supervisor forwarded this to Executor.", nil); err != nil {
				return "", fmt.Errorf("executor_agent: record route_to_executor: %w", err)
			}

			tipBeforeExecutor := rc.Recorder.TerminalHash()
			reply, _, err := agent.RunAgentWithTrace(ctx, executorAgent, req.Request)
			if err != nil {
				return "", fmt.Errorf("executor_agent: run: %w", err)
			}

			if rc.Recorder.TerminalHash() == tipBeforeExecutor {
				if _, err := rc.Recorder.Record(ctx, "executor", "decide", "done", reply, map[string]any{"action": "hold"}); err != nil {
					return "", fmt.Errorf("executor_agent: record hold decision: %w", err)
				}
			}
			return reply, nil
		},
	)
}
