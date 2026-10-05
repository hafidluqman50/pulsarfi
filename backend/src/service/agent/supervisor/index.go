// Package supervisor is the Quasar role: the one agent that talks to the
// user and coordinates Nova (analyzer_agent) and Comet (executor_agent).
// Nova and Comet are complete agents of their own; Quasar calls them as
// tools, so everything they do comes out of Quasar's single event stream.
// Quasar's own tools open the on-chain Task, ask the user questions, and
// pause for the Arm Card. See docs/plans/agent-as-tool-orchestration.md.
package supervisor

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	dbmodel "github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent"
)

// TaskOpener is the Task lifecycle Quasar's tools need, implemented by
// *agent.Orchestrator: open_task and update_task call these, so the Postgres
// row, the on-chain createTask and the Sub Task chain stay in one place.
type TaskOpener interface {
	CommitTask(ctx context.Context, rc *agent.RunContext, decision agent.RouteDecision) error
	UpdateTask(ctx context.Context, rc *agent.RunContext, decision agent.RouteDecision) error
}

// ArmedTaskReader is the one read executor_agent's guard needs: whether the
// Task was armed on-chain. Implemented by the existing AgentTaskRepository,
// the same ArmedAt condition TaskService.ExecuteTask already checks.
type ArmedTaskReader interface {
	FindByID(ctx context.Context, id int64) (dbmodel.AgentTask, bool, error)
}

// New builds Quasar. analyzerAgent and executorAgent are the already-built
// Nova and Comet agents, wrapped here as the tools analyzer_agent and
// executor_agent.
func New(ctx context.Context, chatModel model.ToolCallingChatModel, analyzerAgent, executorAgent adk.Agent, tasks TaskOpener, armed ArmedTaskReader) (adk.Agent, error) {
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
	analyzerCall, err := newAnalyzerCall(ctx, analyzerAgent)
	if err != nil {
		return nil, fmt.Errorf("supervisor: build analyzer_agent tool: %w", err)
	}
	awaitArm, err := newAwaitArmTool()
	if err != nil {
		return nil, fmt.Errorf("supervisor: build await_arm tool: %w", err)
	}
	executorCall, err := newExecutorCall(ctx, executorAgent, armed)
	if err != nil {
		return nil, fmt.Errorf("supervisor: build executor_agent tool: %w", err)
	}

	supervisorAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "supervisor_agent",
		Description:   "Talks to the user and coordinates the analyzer and the executor.",
		Instruction:   instructions,
		Model:         chatModel,
		GenModelInput: withCurrentTime,
		ToolsConfig: adk.ToolsConfig{
			ToolsNodeConfig:    compose.ToolsNodeConfig{Tools: []tool.BaseTool{openTask, askUser, updateTask, analyzerCall, awaitArm, executorCall}},
			EmitInternalEvents: true,
		},
		// Bounded: a full trade turn is open_task, ask_user, update_task,
		// analyzer_agent, await_arm, executor_agent plus replies; the rest is
		// headroom for a refused call that has to be reordered.
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
	messages := []adk.Message{schema.SystemMessage(instruction), schema.SystemMessage(agent.CurrentTimeContext())}
	return append(messages, input.Messages...), nil
}
