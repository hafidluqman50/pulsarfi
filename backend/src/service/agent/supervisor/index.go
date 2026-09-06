// Package supervisor is the Supervisor role: the mandatory entry point for
// every prompt against a chat. It reads context and dynamically chooses
// what to do — open a new Task (create_task) when it recognizes a
// genuinely distinct, data-needing request, then route to Analyzer and/or
// Executor as the situation calls for — by wiring create_task, Analyzer,
// and Executor in as callable tools, the same dynamic multi-agent routing
// Eino's adk.NewAgentTool is documented for, rather than a fixed
// compose.Graph sequence. Uses a thin custom wrapper (tools_service.go)
// instead of a bare adk.NewAgentTool for each: the hash chain
// (docs/plans/agent-role-architecture.md §7) must record
// route_to_analyzer/route_to_executor in true chronological order relative
// to what each node actually did, which a bare AgentTool's black-box
// invocation gives no hook to do. Replaces orchestrator/ — see
// docs/plans/agent-role-architecture.md §3/§4.
package supervisor

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent"
)

// New builds the Supervisor agent, with analyzerAgent and executorAgent
// (each already built by their own package's New) wired in as tools, plus
// its own create_task tool (docs/plans/agent-task-manager-code-implementation.md
// §7.B) backed directly by the Task/Sub Task repositories — Supervisor
// never receives raw search/price tools directly, and never receives a
// chain client either; it routes and recognizes, it does not gather,
// decide, or move funds itself.
func New(ctx context.Context, chatModel model.ToolCallingChatModel, analyzerAgent, executorAgent adk.Agent, tasks *repository.AgentTaskRepository, subTasks *repository.AgentSubTaskRepository) (adk.Agent, error) {
	createTaskTool, err := newCreateTaskTool(tasks, subTasks)
	if err != nil {
		return nil, fmt.Errorf("supervisor: build create_task tool: %w", err)
	}
	analyzerTool, err := newAnalyzerTool(analyzerAgent, subTasks)
	if err != nil {
		return nil, fmt.Errorf("supervisor: build analyzer tool: %w", err)
	}
	executorTool, err := newExecutorTool(executorAgent, subTasks)
	if err != nil {
		return nil, fmt.Errorf("supervisor: build executor tool: %w", err)
	}

	supervisorAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "supervisor_agent",
		Description: "Reads a chat's context, opens new Tasks it recognizes, and routes to Analyzer and/or Executor as the situation calls for.",
		Instruction: instructions,
		Model:       chatModel,
		ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: []tool.BaseTool{agent.WrapToolGraceful(createTaskTool), agent.WrapToolGraceful(analyzerTool), agent.WrapToolGraceful(executorTool)}}},
		// Eino's own default (20) let one real turn (a compound chart +
		// rebalancing-judgment question) loop for 3 real minutes before
		// failing with ErrExceedMaxIterations — a genuine tool-call loop,
		// not just a slow turn. Lower ceiling fails that same loop in well
		// under a minute instead; it does not fix why the loop happens.
		MaxIterations: 10,
		// Attached only here, never on Analyzer's/Executor's own configs —
		// so only Supervisor's own final synthesis streams to the user (see
		// text_delta_middleware_service.go for why this is the only
		// mechanism that actually produces real token deltas).
		Handlers: []adk.ChatModelAgentMiddleware{&agent.TextDeltaMiddleware{}},
	})
	if err != nil {
		return nil, fmt.Errorf("supervisor: build agent: %w", err)
	}
	return supervisorAgent, nil
}
