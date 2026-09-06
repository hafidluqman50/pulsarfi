// Package executor is the Executor role: decides the concrete action for a
// Task (sell, buy, or hold) and, when it decides to act, submits it
// on-chain itself. This package previously only mechanically submitted an
// action someone else had already decided; it now owns sizing and the
// final call, replacing the old analyzer/ sizing logic and the old
// fundmanager/trader decision logic entirely — see
// docs/plans/agent-role-architecture.md §3/§4. It is not tied to one
// condition shape: its instructions and tools cover a sentiment-only,
// technical-only, or hybrid TriggerDescription alike.
package executor

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	dbmodel "github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent"
)

// TaskExecutor is the on-chain seam. Per design, the smart contract side is
// intentionally dumb — create/execute/cancel, plain reads and writes, no
// judgment logic on-chain at all. All decision-making already happened by
// the time this is called; submit_trade (tools_service.go) calls
// ExecuteTrade with the reasoningHash of the exact agent_sub_tasks chain
// link it just wrote — see GET /api/v1/agent/tasks/:id/reasoning for how
// that chain is served back for independent verification.
type TaskExecutor interface {
	// ExecuteTrade submits the ticker/side/amount Executor's own LLM call
	// decided this cycle. Renamed from ExecuteTask — every parameter here
	// is trade-specific and call-time, never pre-Task-locked
	// (docs/plans/agent-task-manager-rebuild.md §3a).
	ExecuteTrade(ctx context.Context, onChainTaskID uint, intent agent.TradeIntent, reasoningHash [32]byte) (txHash string, tradeID uint64, err error)
	// TradePermissionRemaining reads totalBudget - usedBudget for the armed
	// Task — submit_trade's real ceiling, replacing the old off-chain
	// AmountBpsCap. LogDecision is gone entirely: a hold is just the
	// already-recorded "decide" SubTaskRecord, matching logDecision's
	// removal from the contract.
	TradePermissionRemaining(ctx context.Context, onChainTaskID uint) (remaining string, err error)
}

// StockLookup is Executor's own narrow view of the stock catalog (ISP) —
// just enough to reject a hallucinated/injected ticker before it ever
// reaches ExecuteTrade, never the whole StockRepository surface.
type StockLookup interface {
	FindByTicker(ctx context.Context, ticker string) (dbmodel.Stock, bool, error)
}

// New builds the Executor agent — get_portfolio_holdings and submit_trade
// always ship (this role's own required tools); extraTools lets a caller
// add more (e.g. an indicator-aware price tool for technical conditions),
// the same composition pattern used across every role in this package.
func New(ctx context.Context, chatModel model.ToolCallingChatModel, portfolio PortfolioReader, exec TaskExecutor, stocks StockLookup, extraTools ...tool.BaseTool) (adk.Agent, error) {
	holdingsTool, err := newHoldingsTool(portfolio)
	if err != nil {
		return nil, fmt.Errorf("executor: build holdings tool: %w", err)
	}
	submitTool, err := newSubmitTradeTool(exec, stocks)
	if err != nil {
		return nil, fmt.Errorf("executor: build submit_trade tool: %w", err)
	}
	tools := []tool.BaseTool{agent.WrapToolGraceful(holdingsTool), agent.WrapToolGraceful(submitTool)}
	for _, t := range extraTools {
		if it, ok := t.(tool.InvokableTool); ok {
			tools = append(tools, agent.WrapToolGraceful(it))
		} else {
			tools = append(tools, t)
		}
	}

	executorAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "executor_agent",
		Description: "Decides the concrete action for a Task (sell, buy, or hold), sizes it within the pre-approved cap, and submits it on-chain when it decides to act.",
		Instruction: instructions,
		Model:       chatModel,
		ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools}},
		// See supervisor/index.go's own MaxIterations comment.
		MaxIterations: 10,
	})
	if err != nil {
		return nil, fmt.Errorf("executor: build agent: %w", err)
	}
	return executorAgent, nil
}
