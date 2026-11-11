package supervisor

import (
	"context"
	"encoding/json"
	"errors"
	agentcore "github.com/horizonlabs/pulsarfi-backend/src/app/core/agent"
	"log/slog"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
	"github.com/horizonlabs/pulsarfi-backend/src/contracts"
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

func runContextWithTask(ctx context.Context) (*agentcore.RunContext, bool) {
	run, ok := agentcore.RunContextFrom(ctx)
	return run, ok && run.TaskID != 0
}

func newOpenTaskTool(tasks TaskOpener) (tool.InvokableTool, error) {
	return utils.InferTool(
		"open_task",
		"Opens the permanent on-chain Task for this request. Call it FIRST for anything except plain conversation, even while trade parameters are still missing. Never call it twice in one turn: use update_task afterwards.",
		func(ctx context.Context, decision agentcore.RouteDecision) (toolReply, error) {
			run, ok := agentcore.RunContextFrom(ctx)
			if !ok {
				return toolReply{}, errors.New("open_task: no run context bound to this turn")
			}
			if run.TaskID != 0 {
				return toolReply{ToolError: "a Task is already open: call update_task instead"}, nil
			}
			if err := tasks.CommitTask(ctx, run, decision); err != nil {
				return toolReply{}, err
			}
			return toolReply{TaskID: run.TaskID, Status: "open"}, nil
		},
	)
}

func newUpdateTaskTool(tasks TaskOpener) (tool.InvokableTool, error) {
	return utils.InferTool(
		"update_task",
		"Refreshes the Task that is already open once the user's answers settle (more of) the trade parameters. Send the complete, current decision, including the full card. Never opens a second Task.",
		func(ctx context.Context, decision agentcore.RouteDecision) (toolReply, error) {
			run, ok := runContextWithTask(ctx)
			if !ok {
				return toolReply{ToolError: noOpenTaskReason}, nil
			}
			if err := tasks.UpdateTask(ctx, run, decision); err != nil {
				return toolReply{}, err
			}
			return toolReply{TaskID: run.TaskID, Status: "updated"}, nil
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
	wasInterrupted, _, state := tool.GetInterruptState[agentcore.QuestionsInterrupt](ctx)
	if !wasInterrupted {
		if _, ok := runContextWithTask(ctx); !ok {
			return refusal(noOpenTaskReason), nil
		}
		var args askUserArgs
		if err := json.Unmarshal([]byte(arguments), &args); err != nil || len(args.Questions) == 0 {
			return refusal("ask_user needs at least one question"), nil
		}
		payload := agentcore.QuestionsInterrupt{Questions: args.Questions, Message: args.Message}
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
	wasInterrupted, _, state := tool.GetInterruptState[agentcore.ArmInterrupt](ctx)
	if !wasInterrupted {
		var args awaitArmArgs
		_ = json.Unmarshal([]byte(arguments), &args)
		run, ok := runContextWithTask(ctx)
		if !ok {
			return refusal(noOpenTaskReason), nil
		}
		if !run.Decision.IsActionable {
			return refusal("this Task is not an actionable trade: update_task with is_actionable true first"), nil
		}
        if agentcore.RouteNeedsNova(run.Route) && !run.NovaDone {
            return refusal("Nova has not analysed this Task yet: end your turn now, the system runs Nova next and brings you back"), nil
        }

		recordAwaitConfirmation(ctx, run)
		payload := agentcore.ArmInterrupt{TaskID: run.TaskID, Message: args.Message}
		return "", tool.StatefulInterrupt(ctx, payload, payload)
	}
	if isTarget, _, _ := tool.GetResumeContext[string](ctx); isTarget {
        if run, ok := agentcore.RunContextFrom(ctx); ok {
            run.Armed = true
        }

		return "armed", nil
	}
	return "", tool.StatefulInterrupt(ctx, state, state)
}

func recordAwaitConfirmation(ctx context.Context, run *agentcore.RunContext) {
	if run.Recorder == nil {
		return
	}
	label := run.Decision.Label
	run.Stream.StepStarted("supervisor", "await_confirmation", label)
	if _, err := run.Recorder.Record(ctx, "supervisor", "await_confirmation", "done", "Arm Card presented, awaiting the user's on-chain confirmation.", label, nil); err != nil {
		slog.WarnContext(ctx, "supervisor: record await_confirmation failed", "error", err)
	}
}

type understandArgs struct {
    Route          string `json:"route" jsonschema_description:"where this request goes: none (plain conversation), analyzer_only (ask Nova and relay her findings), analyzer_then_executor (ask Nova, then execute the trade once armed), or executor_only (execute the trade once armed, no Nova)"`
    RequestForNova string `json:"request_for_nova,omitempty" jsonschema_description:"what Nova should gather and conclude, in plain words; required when the route includes Nova, with every ticker, index and news topic the user named"`
}

type understandReply struct {
    Route     string `json:"route,omitempty"`
    ToolError string `json:"tool_error,omitempty"`
}

func newUnderstandTool() (tool.InvokableTool, error) {
    return utils.InferTool(
        "understand",
        "Declares where this request goes: Nova, execution, both, or neither. Call it FIRST for anything except plain conversation, before open_task. Call it again if the user's answers change the route or what Nova should gather. The system runs Nova by itself once the Task is open and settled.",
        func(ctx context.Context, args understandArgs) (understandReply, error) {
            run, ok := agentcore.RunContextFrom(ctx)
            if !ok {
                return understandReply{}, errors.New("understand: no run context bound to this turn")
            }

            if !agentcore.IsKnownRoute(args.Route) {
                return understandReply{ToolError: "route must be one of none, analyzer_only, analyzer_then_executor, executor_only"}, nil
            }

            if agentcore.RouteNeedsNova(args.Route) && args.RequestForNova == "" {
                return understandReply{ToolError: "request_for_nova is required when the route includes Nova"}, nil
            }

            run.Route = args.Route
            run.NovaRequest = args.RequestForNova
            return understandReply{Route: args.Route}, nil
        },
    )
}
