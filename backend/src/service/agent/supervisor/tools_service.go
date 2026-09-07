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
	Label   string `json:"label" jsonschema_description:"A short, human-readable description of this specific action, written in the same language you are replying to the user in — e.g. 'Mengecek tren VKTR lewat Analyzer' or 'Checking VKTR's trend with Analyzer'. Shown to the user as this step's title, never a technical term like 'route_to_analyzer'."`
	// Depth only affects Analyzer's own reasoning effort (analyzer_agent) —
	// see reasoningEffortFor below. executor_agent ignores it and always
	// runs at max effort: any call to Executor means an action might
	// actually be taken, which is inherently the highest-stakes case
	// regardless of what got it there.
	Depth string `json:"depth" jsonschema_description:"'quick' for a chart/portfolio lookup or a plain informational sentiment read — the common case. 'deep' only when the user explicitly asked for deep/quantitative analysis, or when this evaluates an actionable trigger condition whose conclusion could authorize a real trade (docs/plans/dynamic-model-tier-routing.md) — never 'deep' by default."`
}

// reasoningEffortFor maps Depth to DeepSeek's own reasoning-effort value.
// "quick" leaves the model's own default in place (empty string, no
// override) — DeepSeek V4 Flash at its default effort already benchmarks
// close to Pro on general reasoning (MMLU-Pro, Codeforces); the real gap
// vs Pro is hallucination resistance (SimpleQA-Verified, roughly 2x), which
// only matters for a genuinely hallucination-sensitive judgment — "deep"
// pushes Flash to max effort for exactly that case, since the vendor's own
// benchmark shows Flash@max roughly matching Pro@high, at a fraction of the
// cost, rather than switching to a whole different, pricier model tier.
func reasoningEffortFor(depth string) string {
	if depth == "deep" {
		return "max"
	}
	return ""
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
		"Call this as your very first action for every user prompt that isn't pure conversation with nothing to fetch — with no exception, even if the prompt looks like a continuation of something discussed earlier in this chat. Every distinct user message opens its own new Task; there is no such thing as silently attaching a new message to an old Task. If this exact turn already opened a Task (e.g. you are calling this a second time within the same reply), it safely returns that same Task's id instead of creating a duplicate — so calling it again within one turn costs nothing.",
		func(ctx context.Context, req createTaskRequest) (createTaskResponse, error) {
			rc, ok := agent.RunContextFrom(ctx)
			if !ok {
				return createTaskResponse{}, fmt.Errorf("create_task: no run context bound to this call")
			}
			if rc.TaskID != 0 {
				return createTaskResponse{TaskID: rc.TaskID}, nil
			}
			if rc.OnSubTaskStarted != nil {
				rc.OnSubTaskStarted("supervisor", "recognize_request", req.Summary)
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

			if _, err := rc.Recorder.Record(ctx, "supervisor", "recognize_request", "done", req.Summary, req.Summary, map[string]any{"is_actionable": req.IsActionable}); err != nil {
				return createTaskResponse{}, fmt.Errorf("create_task: record recognize_request: %w", err)
			}
			return createTaskResponse{TaskID: task.ID}, nil
		},
	)
}

// ensureRecorder is a pure precondition check, never a fallback — a Task
// only ever gets opened through create_task (this same turn) or the
// needs_input rebind in runForMessage (task_service.go), both of which set
// rc.Recorder before analyzer_agent/executor_agent can run. There is
// deliberately no path here that resumes a Task purely because one happens
// to already exist for this chat: that was tried once (LastKnownTaskID,
// docs/plans/agent-task-manager-code-implementation.md §7.AD) and caused
// exactly the bug it was meant to prevent a different way — a genuinely
// new user message silently attaching to an old, already-resolved Task
// whenever Supervisor's own model judged it "looked like" a continuation.
// A nil Recorder here means Supervisor skipped create_task; the caller
// gets a plain error, turned into a recoverable tool_error by
// WrapToolGraceful, so Supervisor can correct itself by calling create_task
// instead of silently reusing someone else's Task.
func ensureRecorder(rc *agent.RunContext) error {
	if rc.Recorder == nil {
		return fmt.Errorf("no Task open yet, call create_task first")
	}
	return nil
}

func newAnalyzerTool(analyzerAgentQuick, analyzerAgentDeep adk.Agent) (tool.InvokableTool, error) {
	return utils.InferTool(
		"analyzer_agent",
		"Gathers news and/or technical evidence for the current Task's trigger condition and concludes whether it is satisfied. Also handles portfolio/chart questions.",
		func(ctx context.Context, req routeRequest) (string, error) {
			rc, ok := agent.RunContextFrom(ctx)
			if !ok {
				return "", fmt.Errorf("analyzer_agent: no run context bound to this call")
			}
			if err := ensureRecorder(rc); err != nil {
				return "", fmt.Errorf("analyzer_agent: %w", err)
			}
			if rc.OnSubTaskStarted != nil {
				rc.OnSubTaskStarted("supervisor", "route_to_analyzer", req.Label)
			}
			if _, err := rc.Recorder.Record(ctx, "supervisor", "route_to_analyzer", "done", "Supervisor routed this prompt to Analyzer for evaluation.", req.Label, nil); err != nil {
				return "", fmt.Errorf("analyzer_agent: record route_to_analyzer: %w", err)
			}

			if rc.OnSubTaskStarted != nil {
				rc.OnSubTaskStarted("analyzer", "gather_evidence", req.Label)
			}
			// The model itself changes here, not just reasoning effort within
			// one fixed model — found live, confirmed against real DeepSeek
			// billing: Depth alone (as a reasoning-effort override, §1.1/§1.2
			// of dynamic-model-tier-routing.md) never actually changed which
			// model ran, so every "quick" call was still billed at Pro rates.
			analyzerAgent := analyzerAgentQuick
			if req.Depth == "deep" {
				analyzerAgent = analyzerAgentDeep
			}
			conclusion, nestedToolCalls, err := agent.RunAgentWithTrace(ctx, analyzerAgent, req.Request, nil, reasoningEffortFor(req.Depth))
			if err != nil {
				return "", fmt.Errorf("analyzer_agent: run: %w", err)
			}
			rc.NestedToolCalls = append(rc.NestedToolCalls, nestedToolCalls...)

			if _, err := rc.Recorder.Record(ctx, "analyzer", "gather_evidence", "done", conclusion, req.Label, nil); err != nil {
				return "", fmt.Errorf("analyzer_agent: record gather_evidence: %w", err)
			}
			return conclusion, nil
		},
	)
}

func newExecutorTool(executorAgent adk.Agent) (tool.InvokableTool, error) {
	return utils.InferTool(
		"executor_agent",
		"Decides the concrete action for the current Task (sell, buy, or hold) and submits it on-chain when it decides to act.",
		func(ctx context.Context, req routeRequest) (string, error) {
			rc, ok := agent.RunContextFrom(ctx)
			if !ok {
				return "", fmt.Errorf("executor_agent: no run context bound to this call")
			}
			if err := ensureRecorder(rc); err != nil {
				return "", fmt.Errorf("executor_agent: %w", err)
			}
			if rc.OnSubTaskStarted != nil {
				rc.OnSubTaskStarted("supervisor", "route_to_executor", req.Label)
			}
			if _, err := rc.Recorder.Record(ctx, "supervisor", "route_to_executor", "done", "Supervisor forwarded this to Executor.", req.Label, nil); err != nil {
				return "", fmt.Errorf("executor_agent: record route_to_executor: %w", err)
			}

			if rc.OnSubTaskStarted != nil {
				rc.OnSubTaskStarted("executor", "decide", req.Label)
			}
			tipBeforeExecutor := rc.Recorder.TerminalHash()
			reply, nestedToolCalls, err := agent.RunAgentWithTrace(ctx, executorAgent, req.Request, nil, "max")
			if err != nil {
				return "", fmt.Errorf("executor_agent: run: %w", err)
			}
			rc.NestedToolCalls = append(rc.NestedToolCalls, nestedToolCalls...)

			if rc.Recorder.TerminalHash() == tipBeforeExecutor {
				if _, err := rc.Recorder.Record(ctx, "executor", "decide", "done", reply, req.Label, map[string]any{"action": "hold"}); err != nil {
					return "", fmt.Errorf("executor_agent: record hold decision: %w", err)
				}
			}
			return reply, nil
		},
	)
}
