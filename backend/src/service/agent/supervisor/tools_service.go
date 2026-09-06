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

func newCreateTaskTool(tasks *repository.AgentTaskRepository, subTasks *repository.AgentSubTaskRepository) (tool.InvokableTool, error) {
	return utils.InferTool(
		"create_task",
		"Opens a new Task for a request you've just recognized as genuinely distinct — call this once, the first time you conclude this prompt isn't a continuation of an already-open Task in this chat, and isn't pure conversation with nothing to fetch.",
		func(ctx context.Context, req createTaskRequest) (createTaskResponse, error) {
			rc, ok := agent.RunContextFrom(ctx)
			if !ok {
				return createTaskResponse{}, fmt.Errorf("create_task: no run context bound to this call")
			}
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
			recorder.OnRecord = rc.OnSubTask
			rc.TaskID = task.ID
			rc.Recorder = recorder

			if _, err := rc.Recorder.Record(ctx, "supervisor", "recognize_request", "done", req.Summary, map[string]any{"is_actionable": req.IsActionable}); err != nil {
				return createTaskResponse{}, fmt.Errorf("create_task: record recognize_request: %w", err)
			}
			return createTaskResponse{TaskID: task.ID}, nil
		},
	)
}

func ensureRecorder(ctx context.Context, rc *agent.RunContext, subTasks *repository.AgentSubTaskRepository) error {
	if rc.Recorder != nil {
		return nil
	}
	if rc.LastKnownTaskID == nil {
		return fmt.Errorf("no Task open yet, call create_task first")
	}
	recorder, err := agent.NewSubTaskRecorder(ctx, subTasks, *rc.LastKnownTaskID, rc.TriggerDescription, rc.Wallet)
	if err != nil {
		return fmt.Errorf("resume Task %d: %w", *rc.LastKnownTaskID, err)
	}
	recorder.OnRecord = rc.OnSubTask
	rc.TaskID = *rc.LastKnownTaskID
	rc.OnChainTaskID = rc.LastKnownOnChainTaskID
	rc.Recorder = recorder
	return nil
}

func newAnalyzerTool(analyzerAgent adk.Agent, subTasks *repository.AgentSubTaskRepository) (tool.InvokableTool, error) {
	return utils.InferTool(
		"analyzer_agent",
		"Gathers news and/or technical evidence for the current Task's trigger condition and concludes whether it is satisfied. Also handles portfolio/chart questions.",
		func(ctx context.Context, req routeRequest) (string, error) {
			rc, ok := agent.RunContextFrom(ctx)
			if !ok {
				return "", fmt.Errorf("analyzer_agent: no run context bound to this call")
			}
			if err := ensureRecorder(ctx, rc, subTasks); err != nil {
				return "", fmt.Errorf("analyzer_agent: %w", err)
			}
			if _, err := rc.Recorder.Record(ctx, "supervisor", "route_to_analyzer", "done", "Supervisor routed this prompt to Analyzer for evaluation.", nil); err != nil {
				return "", fmt.Errorf("analyzer_agent: record route_to_analyzer: %w", err)
			}

			conclusion, nestedToolCalls, err := agent.RunAgentWithTrace(ctx, analyzerAgent, req.Request, nil)
			if err != nil {
				return "", fmt.Errorf("analyzer_agent: run: %w", err)
			}
			rc.NestedToolCalls = append(rc.NestedToolCalls, nestedToolCalls...)

			if _, err := rc.Recorder.Record(ctx, "analyzer", "gather_evidence", "done", conclusion, nil); err != nil {
				return "", fmt.Errorf("analyzer_agent: record gather_evidence: %w", err)
			}
			return conclusion, nil
		},
	)
}

func newExecutorTool(executorAgent adk.Agent, subTasks *repository.AgentSubTaskRepository) (tool.InvokableTool, error) {
	return utils.InferTool(
		"executor_agent",
		"Decides the concrete action for the current Task (sell, buy, or hold) and submits it on-chain when it decides to act.",
		func(ctx context.Context, req routeRequest) (string, error) {
			rc, ok := agent.RunContextFrom(ctx)
			if !ok {
				return "", fmt.Errorf("executor_agent: no run context bound to this call")
			}
			if err := ensureRecorder(ctx, rc, subTasks); err != nil {
				return "", fmt.Errorf("executor_agent: %w", err)
			}
			if _, err := rc.Recorder.Record(ctx, "supervisor", "route_to_executor", "done", "Supervisor forwarded this to Executor.", nil); err != nil {
				return "", fmt.Errorf("executor_agent: record route_to_executor: %w", err)
			}

			tipBeforeExecutor := rc.Recorder.TerminalHash()
			reply, nestedToolCalls, err := agent.RunAgentWithTrace(ctx, executorAgent, req.Request, nil)
			if err != nil {
				return "", fmt.Errorf("executor_agent: run: %w", err)
			}
			rc.NestedToolCalls = append(rc.NestedToolCalls, nestedToolCalls...)

			if rc.Recorder.TerminalHash() == tipBeforeExecutor {
				if _, err := rc.Recorder.Record(ctx, "executor", "decide", "done", reply, map[string]any{"action": "hold"}); err != nil {
					return "", fmt.Errorf("executor_agent: record hold decision: %w", err)
				}
			}
			return reply, nil
		},
	)
}
