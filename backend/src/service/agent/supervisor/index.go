// Package supervisor is the Quasar role: the one agent that talks to the
// user. Its tools declare where a request goes (understand), open the
// on-chain Task, ask the user questions, and pause for the Arm Card. Nova and
// Comet are nodes of the orchestrator graph, not tools of Quasar. See
// docs/plans/agent-workflow-graph.md.
package supervisor

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	agentcore "github.com/horizonlabs/pulsarfi-backend/src/app/core/agent"
)

// TaskOpener is the Task lifecycle Quasar's tools need, implemented by
// *agent.Orchestrator: open_task and update_task call these, so the Postgres
// row, the on-chain createTask and the Sub Task chain stay in one place.
type TaskOpener interface {
	CommitTask(ctx context.Context, rc *agentcore.RunContext, decision agentcore.RouteDecision) error
	UpdateTask(ctx context.Context, rc *agentcore.RunContext, decision agentcore.RouteDecision) error
}

func New(ctx context.Context, chatModel model.ToolCallingChatModel, tasks TaskOpener) (adk.Agent, error) {
    understand, err := newUnderstandTool()
    if err != nil {
        return nil, fmt.Errorf("supervisor: build understand tool: %w", err)
    }

    openTask, err := newOpenTaskTool(tasks)
    if err != nil {
        return nil, fmt.Errorf("supervisor: build open_task tool: %w", err)
    }

    askUser, err := newAskUserTool()
    if err != nil {
        return nil, fmt.Errorf("supervisor: build ask_user tool: %w", err)
    }

    updateTask, err := newUpdateTaskTool(tasks)
    if err != nil {
        return nil, fmt.Errorf("supervisor: build update_task tool: %w", err)
    }

    awaitArm, err := newAwaitArmTool()
    if err != nil {
        return nil, fmt.Errorf("supervisor: build await_arm tool: %w", err)
    }

    supervisorAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
        Name:          "supervisor_agent",
        Description:   "Talks to the user and decides where each request goes.",
        Instruction:   instructions,
        Model:         chatModel,
        GenModelInput: withCurrentTime,
        ToolsConfig: adk.ToolsConfig{
            ToolsNodeConfig:    compose.ToolsNodeConfig{Tools: []tool.BaseTool{understand, openTask, askUser, updateTask, awaitArm}},
            ReturnDirectly:     map[string]bool{"await_arm": true},
            EmitInternalEvents: true,
        },
        MaxIterations: 12,
    })
    if err != nil {
        return nil, fmt.Errorf("supervisor: build agent: %w", err)
    }

    return supervisorAgent, nil
}

// withCurrentTime gives Quasar the one thing it cannot know on its own: what
// time it actually is right now. A custom input builder instead of the
// default also keeps the card contract's {id} placeholders from being read as
// prompt template variables.
func withCurrentTime(_ context.Context, instruction string, input *adk.AgentInput) ([]adk.Message, error) {
	messages := []adk.Message{schema.SystemMessage(instruction), schema.SystemMessage(agentcore.CurrentTimeContext())}
	return append(messages, input.Messages...), nil
}

// runner: the runner of Quasar.
// step: runs one agent turn for a graph node.
func NewNode(runner *adk.Runner, step agentcore.StepFunc) agentcore.NodeFunc {
    return func(ctx context.Context, state *agentcore.TurnState) (*agentcore.TurnState, error) {
        var messages []adk.Message
        if state != nil {
            messages = state.Messages
        }

        result, _, err := step(ctx, runner, "quasar", state, messages)
        return result, err
    }
}
