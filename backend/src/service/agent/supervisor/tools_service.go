package supervisor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"github.com/horizonlabs/pulsarfi-backend/src/contracts"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent"
)

const noOpenTaskReason = "no Task is open yet: call open_task first"

// toolReply is what open_task and update_task hand back to the model.
type toolReply struct {
	TaskID    int64  `json:"task_id,omitempty"`
	Status    string `json:"status,omitempty"`
	ToolError string `json:"tool_error,omitempty"`
}

// refusal is how a tool says "not now" without ending the turn: the model
// reads the tool_error, fixes the order and goes on, instead of the run
// aborting. Same shape WrapToolGraceful hands back for a failed tool.
func refusal(reason string) string {
	payload, err := json.Marshal(map[string]string{
		"tool_error": reason,
		"note":       "This call was refused by the system. Fix the order of your calls and continue; never mention this to the user.",
	})
	if err != nil {
		return `{"tool_error":"refused"}`
	}
	return string(payload)
}

func runContextWithTask(ctx context.Context) (*agent.RunContext, bool) {
	rc, ok := agent.RunContextFrom(ctx)
	return rc, ok && rc.TaskID != 0
}

func newOpenTaskTool(tasks TaskOpener) (tool.InvokableTool, error) {
	return utils.InferTool(
		"open_task",
		"Opens the permanent on-chain Task for this request. Call it FIRST for anything except plain conversation, even while trade parameters are still missing. Never call it twice in one turn: use update_task afterwards.",
		func(ctx context.Context, decision agent.RouteDecision) (toolReply, error) {
			rc, ok := agent.RunContextFrom(ctx)
			if !ok {
				return toolReply{}, errors.New("open_task: no run context bound to this turn")
			}
			if rc.TaskID != 0 {
				return toolReply{ToolError: "a Task is already open: call update_task instead"}, nil
			}
			if err := tasks.CommitTask(ctx, rc, decision); err != nil {
				return toolReply{}, err
			}
			return toolReply{TaskID: rc.TaskID, Status: "open"}, nil
		},
	)
}

func newUpdateTaskTool(tasks TaskOpener) (tool.InvokableTool, error) {
	return utils.InferTool(
		"update_task",
		"Refreshes the Task that is already open once the user's answers settle (more of) the trade parameters. Send the complete, current decision, including the full card. Never opens a second Task.",
		func(ctx context.Context, decision agent.RouteDecision) (toolReply, error) {
			rc, ok := runContextWithTask(ctx)
			if !ok {
				return toolReply{ToolError: noOpenTaskReason}, nil
			}
			if err := tasks.UpdateTask(ctx, rc, decision); err != nil {
				return toolReply{}, err
			}
			return toolReply{TaskID: rc.TaskID, Status: "updated"}, nil
		},
	)
}

type askUserArgs struct {
	Message   string                  `json:"message" jsonschema_description:"one or two warm sentences in the user's language saying you need a couple of things confirmed and that nothing has been executed yet; do not restate the questions"`
	Questions []contracts.IntakeField `json:"questions" jsonschema_description:"one entry for every parameter still missing, in the user's language; see the question rules in your instructions"`
}

// askUserTool is the questions-card pause. It interrupts the turn; the
// user's next chat message resumes it and becomes this tool's result. It is a
// plain InvokableTool rather than utils.InferTool because InferTool wraps
// every error, and an interrupt must reach the runtime untouched.
type askUserTool struct {
	info *schema.ToolInfo
}

func newAskUserTool() (tool.InvokableTool, error) {
	info, err := utils.GoStruct2ToolInfo[askUserArgs](
		"ask_user",
		"Pauses the turn and shows the user a questions card for the parameters still missing. The user's answer comes back as this tool's result. Never ask in chat text what the card already asks.",
	)
	if err != nil {
		return nil, err
	}
	return &askUserTool{info: info}, nil
}

func (t *askUserTool) Info(_ context.Context) (*schema.ToolInfo, error) { return t.info, nil }

func (t *askUserTool) InvokableRun(ctx context.Context, arguments string, _ ...tool.Option) (string, error) {
	wasInterrupted, _, state := tool.GetInterruptState[agent.QuestionsInterrupt](ctx)
	if !wasInterrupted {
		if _, ok := runContextWithTask(ctx); !ok {
			return refusal(noOpenTaskReason), nil
		}
		var args askUserArgs
		if err := json.Unmarshal([]byte(arguments), &args); err != nil || len(args.Questions) == 0 {
			return refusal("ask_user needs at least one question"), nil
		}
		payload := agent.QuestionsInterrupt{Questions: args.Questions, Message: args.Message}
		return "", tool.StatefulInterrupt(ctx, payload, payload)
	}
	if isTarget, hasData, answer := tool.GetResumeContext[string](ctx); isTarget && hasData {
		return answer, nil
	}
	return "", tool.StatefulInterrupt(ctx, state, state)
}

type awaitArmArgs struct {
	Message string `json:"message" jsonschema_description:"one short sentence in the user's language telling them to review the parameters and click Arm on the card below; when Nova analyzed this turn, first 1-2 sentences summarizing her findings"`
}

// awaitArmTool is the Arm pause: its only job is to interrupt and be resumed
// by TaskService.ExecuteTask once the user has armed on-chain. It must not
// wrap the Comet call: an agent tool invoked inside a tool that interrupted
// itself reads that interrupt as its own and fails (spike S3).
type awaitArmTool struct {
	info *schema.ToolInfo
}

func newAwaitArmTool() (tool.InvokableTool, error) {
	info, err := utils.GoStruct2ToolInfo[awaitArmArgs](
		"await_arm",
		"Presents the Arm Card for the Task and pauses until the user has armed it on-chain; returns \"armed\" when they have. Call it once every trade parameter is settled (and Nova's analysis is done, if the user wanted it), right after the one sentence that points the user to the card.",
	)
	if err != nil {
		return nil, err
	}
	return &awaitArmTool{info: info}, nil
}

func (t *awaitArmTool) Info(_ context.Context) (*schema.ToolInfo, error) { return t.info, nil }

func (t *awaitArmTool) InvokableRun(ctx context.Context, arguments string, _ ...tool.Option) (string, error) {
	wasInterrupted, _, state := tool.GetInterruptState[agent.ArmInterrupt](ctx)
	if !wasInterrupted {
		var args awaitArmArgs
		_ = json.Unmarshal([]byte(arguments), &args)
		rc, ok := runContextWithTask(ctx)
		if !ok {
			return refusal(noOpenTaskReason), nil
		}
		if !rc.Decision.IsActionable {
			return refusal("this Task is not an actionable trade: update_task with is_actionable true first"), nil
		}
		recordAwaitConfirmation(ctx, rc)
		payload := agent.ArmInterrupt{TaskID: rc.TaskID, Message: args.Message}
		return "", tool.StatefulInterrupt(ctx, payload, payload)
	}
	if isTarget, _, _ := tool.GetResumeContext[string](ctx); isTarget {
		return "armed", nil
	}
	return "", tool.StatefulInterrupt(ctx, state, state)
}

func recordAwaitConfirmation(ctx context.Context, rc *agent.RunContext) {
	if rc.Recorder == nil {
		return
	}
	label := rc.Decision.Label
	rc.OnSubTaskStarted("supervisor", "await_confirmation", label)
	if _, err := rc.Recorder.Record(ctx, "supervisor", "await_confirmation", "done", "Arm Card presented, awaiting the user's on-chain confirmation.", label, nil); err != nil {
		slog.WarnContext(ctx, "supervisor: record await_confirmation failed", "error", err)
	}
}

type agentRequest struct {
	Request string `json:"request"`
}

func rewrapRequest(arguments string, build func(userRequest string) string) (string, error) {
	var request agentRequest
	if err := json.Unmarshal([]byte(arguments), &request); err != nil {
		return "", fmt.Errorf("invalid request arguments: %w", err)
	}
	wrapped, err := json.Marshal(agentRequest{Request: build(request.Request)})
	if err != nil {
		return "", err
	}
	return string(wrapped), nil
}

func innerAgentTool(ctx context.Context, inner adk.Agent) (tool.InvokableTool, error) {
	invokable, ok := adk.NewAgentTool(ctx, inner).(tool.InvokableTool)
	if !ok {
		return nil, errors.New("agent tool is not invokable")
	}
	return invokable, nil
}

// analyzerCall calls Nova and writes the two Sub Tasks around her work. It
// forwards every tool option to the inner agent tool: dropping them silently
// loses all of Nova's live events (spike S5).
type analyzerCall struct {
	inner tool.InvokableTool
}

func newAnalyzerCall(ctx context.Context, analyzerAgent adk.Agent) (tool.InvokableTool, error) {
	inner, err := innerAgentTool(ctx, analyzerAgent)
	if err != nil {
		return nil, err
	}
	return &analyzerCall{inner: inner}, nil
}

func (c *analyzerCall) Info(ctx context.Context) (*schema.ToolInfo, error) { return c.inner.Info(ctx) }

func (c *analyzerCall) InvokableRun(ctx context.Context, arguments string, opts ...tool.Option) (string, error) {
	rc, ok := runContextWithTask(ctx)
	if !ok {
		return refusal(noOpenTaskReason), nil
	}
	decision := rc.Decision
	// Nova's request is built from the Task. A trade that is still being
	// configured would send her the wrong instructions (no verification, no
	// verdict), so the Task must be settled first.
	if decision.Side != "" && !decision.IsActionable {
		return refusal("this trade is not settled yet: call update_task with the settled parameters and is_actionable true before analyzer_agent"), nil
	}
	wrappedArguments, err := rewrapRequest(arguments, func(userRequest string) string {
		return agent.BuildAnalyzerRequest(userRequest, decision.Shape, decision.MentionedTicker, decision.Side, decision.IsActionable)
	})
	if err != nil {
		return "", fmt.Errorf("analyzer_agent: %w", err)
	}

	rc.OnSubTaskStarted("supervisor", "route_to_analyzer", decision.Label)
	if _, err := rc.Recorder.Record(ctx, "supervisor", "route_to_analyzer", "done", "Routed to Nova for evaluation.", decision.Label, nil); err != nil {
		return "", fmt.Errorf("analyzer_agent: record route_to_analyzer: %w", err)
	}

	rc.OnSubTaskStarted("analyzer", "gather_evidence", decision.Label)
	rc.CurrentAgent = "analyzer"
	rc.CurrentStepName = "gather_evidence"
	reply, err := c.inner.InvokableRun(ctx, wrappedArguments, opts...)
	if err != nil {
		if _, recErr := rc.Recorder.Record(ctx, "analyzer", "gather_evidence", "failed", err.Error(), decision.Label, nil); recErr != nil {
			slog.ErrorContext(ctx, "supervisor: record gather_evidence failed step", "error", recErr)
		}
		return "", fmt.Errorf("analyzer_agent run: %w", err)
	}

	// A tradeable/entry_price verdict is only meaningful for an actionable
	// request — for an informational one there is no trade to verdict on, so
	// nothing is parsed as one; Nova's whole reply is just her answer.
	prose := reply
	var verdict *agent.NovaVerdict
	if decision.IsActionable {
		verdict, prose = agent.ParseNovaVerdict(reply)
	}
	if _, err := rc.Recorder.Record(ctx, "analyzer", "gather_evidence", "done", prose, decision.Label, nil); err != nil {
		return "", fmt.Errorf("analyzer_agent: record gather_evidence: %w", err)
	}
	if verdict == nil {
		return prose, nil
	}
	return prose + "\n\n" + agent.FormatNovaVerdict(verdict), nil
}

const executeInstruction = "\n\nThe task is armed and approved on-chain. Size within the approved budget and execute the trade on-chain using submit_trade. If Nova's findings or verdict appear above, reflect them in your chat reasoning to explain your decision."

// executorCall calls Comet. It never interrupts (the Arm pause is
// await_arm); it only refuses until the Task is armed, reading the same
// ArmedAt condition ExecuteTask already checks, then writes route_to_executor
// and, when Comet's own tools recorded nothing, the decide step.
type executorCall struct {
	inner tool.InvokableTool
	armed ArmedTaskReader
}

func newExecutorCall(ctx context.Context, executorAgent adk.Agent, armed ArmedTaskReader) (tool.InvokableTool, error) {
	inner, err := innerAgentTool(ctx, executorAgent)
	if err != nil {
		return nil, err
	}
	return &executorCall{inner: inner, armed: armed}, nil
}

func (c *executorCall) Info(ctx context.Context) (*schema.ToolInfo, error) { return c.inner.Info(ctx) }

func (c *executorCall) InvokableRun(ctx context.Context, arguments string, opts ...tool.Option) (string, error) {
	rc, ok := runContextWithTask(ctx)
	if !ok {
		return refusal(noOpenTaskReason), nil
	}
	task, found, err := c.armed.FindByID(ctx, rc.TaskID)
	if err != nil {
		return "", fmt.Errorf("executor_agent: load task: %w", err)
	}
	if !found || task.ArmedAt == nil || task.OnChainTaskID == nil {
		return refusal("the Task is not armed yet: call await_arm first and wait for \"armed\""), nil
	}
	wrappedArguments, err := rewrapRequest(arguments, func(userRequest string) string {
		return userRequest + executeInstruction
	})
	if err != nil {
		return "", fmt.Errorf("executor_agent: %w", err)
	}

	label := rc.Decision.Label
	rc.OnSubTaskStarted("supervisor", "route_to_executor", label)
	if _, err := rc.Recorder.Record(ctx, "supervisor", "route_to_executor", "done", "Routed to Comet for execution.", label, nil); err != nil {
		return "", fmt.Errorf("executor_agent: record route_to_executor: %w", err)
	}

	tipBefore := rc.Recorder.TerminalHash()
	rc.CurrentAgent = "executor"
	rc.CurrentStepName = "decide"
	reply, err := c.inner.InvokableRun(ctx, wrappedArguments, opts...)
	if err != nil {
		if _, recErr := rc.Recorder.Record(ctx, "executor", "decide", "failed", err.Error(), label, nil); recErr != nil {
			slog.ErrorContext(ctx, "supervisor: record decide failed step", "error", recErr)
		}
		return "", fmt.Errorf("executor_agent run: %w", err)
	}
	if rc.Recorder.TerminalHash() == tipBefore {
		if _, err := rc.Recorder.Record(ctx, "executor", "decide", "done", reply, label, nil); err != nil {
			return "", fmt.Errorf("executor_agent: record decide step: %w", err)
		}
	}
	return reply, nil
}
