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
	"errors"
	"fmt"
	helper "github.com/horizonlabs/pulsarfi-backend/src/app/helper"
	"log/slog"
	"math/big"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/ethereum/go-ethereum/common"
	agentcore "github.com/horizonlabs/pulsarfi-backend/src/app/core/agent"
	"github.com/horizonlabs/pulsarfi-backend/src/contracts"
	dbmodel "github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
)

// TaskExecutor is the on-chain seam. Per design, the smart contract side is
// intentionally dumb — create/execute/cancel, plain reads and writes, no
// judgment logic on-chain at all. All decision-making already happened by
// the time this is called; submit_trade (tools_service.go) calls
// ExecuteTrade with the reasoningHash of the exact agent_sub_tasks chain
// link it just wrote — see GET /api/v1/agent/tasks/:id/reasoning for how
// that chain is served back for independent verification.
type TaskExecutor interface {
	// ExecuteTrade submits the ticker/side Executor's own LLM call decided
	// this cycle, plus everything AgentTaskManager.executeTrade itself
	// requires beyond that (docs/plans/agent-trade-execution.md): subTaskID
	// (the on-chain id of the already-recorded "decide" step — guaranteed
	// to exist by the time this is called, since SubTaskRecorder.Record is
	// chain-first), token (the stock's own ERC20 contract address),
	// amount/minimumOutputAmount (already converted to the correct raw
	// units for the given side — a Sell needs stock-token units, not IDRX,
	// see Gap C — computed by the caller, never inside this interface), and
	// summary (the human-readable label, mirrored on-chain in the
	// TradeRecord). contracts.TradeIntent.Amount (a string, IDRX-equivalent) is
	// Comet's own original decision, kept for audit/display only — it is
	// never what actually gets sent on-chain, `amount` is.
	ExecuteTrade(ctx context.Context, onChainTaskID uint64, subTaskID uint64, token common.Address, ticker string, side contracts.TradeSide, amount, minimumOutputAmount *big.Int, summary string, reasoningHash [32]byte) (out *agentcore.ExecuteTradeOutput, err error)
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

// SpotPriceReader resolves the current on-chain AMM spot price for a
// ticker — IDRX per whole stock token, from PulsarProtocol's own pool
// (PriceService.GetOnchainPriceV4), not an off-chain IDX/Yahoo quote. Used
// to convert Comet's IDRX-equivalent trade decision into the stock-token
// units a Sell's executeTrade call actually needs (Gap C), and to compute
// a slippage-bounded minimumOutputAmount for either side (Gap A) — the
// same price a real swap executes against, not a market data source that
// can drift from it.
type SpotPriceReader interface {
	OnchainSpotPrice(ctx context.Context, ticker string) (idrxPerWholeStock float64, err error)
	// QuoteStockToIdrx values an arbitrary size at the pool's live spot
	// price. It does NOT model price impact — the contract multiplies
	// linearly off sqrtPriceX96 (verified on-chain), so this is worth-right-
	// now, not what-you-would-actually-receive. Returns raw IDRX (2 dp) for
	// the given raw stock amount (18 dp).
	QuoteStockToIdrx(ctx context.Context, ticker string, stockAmountRaw *big.Int) (idrxRaw *big.Int, err error)
}

// BalanceReader reads the owner's real on-chain IDRX balance. Comet needs
// this before sizing anything: get_portfolio_holdings returns which tickers
// are held, never how much IDRX is available to spend, and there was no way
// at all to check that before this — the exact gap Comet cited when it held
// on Tasks 76/77 ("tidak ada tool cek saldo").
type BalanceReader interface {
	IDRXBalance(ctx context.Context, wallet string) (idrxRaw *big.Int, err error)
}

// TradeRecorder is Executor's own narrow view of AgentTradeRepository (ISP)
// — just enough to insert the one row a real ExecuteTrade success produces.
// agent_trades had zero writers anywhere in the codebase before this
// (docs/plans/agent-trade-execution.md Definition of Done).
type TradeRecorder interface {
	Create(ctx context.Context, trade dbmodel.AgentTrade) (dbmodel.AgentTrade, error)
}

// StockTransactionRecorder is Executor's view of StockTransactionRepository
// to record user swap and portfolio activity in stock_transactions.
type StockTransactionRecorder interface {
	Create(ctx context.Context, input repository.StockTransactionCreateInput) (dbmodel.StockTransaction, error)
}

// New builds the Executor agent — get_portfolio_holdings and submit_trade
// always ship (this role's own required tools); extraTools lets a caller
// add more (e.g. an indicator-aware price tool for technical conditions),
// the same composition pattern used across every role in this package.
func New(ctx context.Context, chatModel model.ToolCallingChatModel, portfolio PortfolioReader, exec TaskExecutor, stocks StockLookup, prices SpotPriceReader, balances BalanceReader, idrxAddress string, trades TradeRecorder, stockTransactions StockTransactionRecorder, extraTools ...tool.BaseTool) (adk.Agent, error) {
	holdingsTool, err := newHoldingsTool(portfolio)
	if err != nil {
		return nil, fmt.Errorf("executor: build holdings tool: %w", err)
	}
	submitTool, err := newSubmitTradeTool(exec, stocks, prices, trades, stockTransactions, idrxAddress)
	if err != nil {
		return nil, fmt.Errorf("executor: build submit_trade tool: %w", err)
	}
	spotPriceTool, err := newSpotPriceTool(stocks, prices)
	if err != nil {
		return nil, fmt.Errorf("executor: build get_spot_price tool: %w", err)
	}
	balanceTool, err := newBalanceTool(balances)
	if err != nil {
		return nil, fmt.Errorf("executor: build get_idrx_balance tool: %w", err)
	}
	baseTools := []tool.BaseTool{holdingsTool, spotPriceTool, balanceTool, submitTool}
	tools := append(baseTools, extraTools...)

	executorAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "executor_agent",
		Description: "Decides the concrete action for a Task (sell, buy, or hold), sizes it within the pre-approved cap, and submits it on-chain when it decides to act.",
		Instruction: instructions,
		Model:       chatModel,
		ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: helper.WrapToolsGraceful(tools)}},
		// See supervisor/index.go's own MaxIterations comment.
		MaxIterations: 10,
	})
	if err != nil {
		return nil, fmt.Errorf("executor: build agent: %w", err)
	}
	return executorAgent, nil
}

type ArmedTaskReader interface {
    FindByID(ctx context.Context, id int64) (dbmodel.AgentTask, bool, error)
}

// runner: the runner of Comet.
// tasks: reads the Task to confirm it was armed on-chain.
// step: runs one agent turn for a graph node.
func NewNode(runner *adk.Runner, tasks ArmedTaskReader, step agentcore.StepFunc) agentcore.NodeFunc {
    return func(ctx context.Context, state *agentcore.TurnState) (*agentcore.TurnState, error) {
        run, ok := agentcore.RunContextFrom(ctx)
        if !ok {
            return state, errors.New("executor: no run context bound to this node")
        }

        if state == nil {
            return nil, errors.New("executor: node started without state")
        }

        task, err := requireArmedTask(ctx, tasks, run)
        if err != nil {
            return state, err
        }

        decision := run.Decision
        rawPrompt := run.RawPrompt
        if task.RawPrompt != nil {
            rawPrompt = *task.RawPrompt
        }

        request := helper.BuildExecutorRequest(rawPrompt, decision.Shape, decision.MentionedTicker, decision.Side, decision.BudgetIDRX, decision.SellAmount, state.NovaFindings)
        if err := recordCometRoute(ctx, run); err != nil {
            return state, err
        }

        tipBefore := run.Recorder.TerminalHash()
        run.Stream.SetCurrent("executor", "decide")

        result, reply, err := step(ctx, runner, "comet", state, []adk.Message{schema.UserMessage(request)})
        if err != nil {
            recordCometFailure(ctx, run, err)
            return result, fmt.Errorf("executor: comet run: %w", err)
        }

        return recordCometResult(ctx, run, result, reply, tipBefore)
    }
}

func requireArmedTask(ctx context.Context, tasks ArmedTaskReader, run *agentcore.RunContext) (dbmodel.AgentTask, error) {
    task, found, err := tasks.FindByID(ctx, run.TaskID)
    if err != nil {
        return dbmodel.AgentTask{}, fmt.Errorf("executor: load task %d: %w", run.TaskID, err)
    }

    if !found || task.ArmedAt == nil || task.OnChainTaskID == nil {
        return dbmodel.AgentTask{}, fmt.Errorf("executor: task %d is not armed", run.TaskID)
    }

    return task, nil
}

func recordCometRoute(ctx context.Context, run *agentcore.RunContext) error {
    label := run.Decision.Label
    run.Stream.StepStarted("supervisor", "route_to_executor", label)
    if _, err := run.Recorder.Record(ctx, "supervisor", "route_to_executor", "done", "Routed to Comet for execution.", label, nil); err != nil {
        return fmt.Errorf("executor: record route_to_executor: %w", err)
    }

    return nil
}

func recordCometFailure(ctx context.Context, run *agentcore.RunContext, cause error) {
    if _, err := run.Recorder.Record(ctx, "executor", "decide", "failed", cause.Error(), run.Decision.Label, nil); err != nil {
        slog.ErrorContext(ctx, "executor: record decide failed step", "chat_id", run.ChatID.String(), "error", err)
    }
}

func recordCometResult(ctx context.Context, run *agentcore.RunContext, state *agentcore.TurnState, reply, tipBefore string) (*agentcore.TurnState, error) {
    if run.Recorder.TerminalHash() == tipBefore {
        if _, err := run.Recorder.Record(ctx, "executor", "decide", "done", reply, run.Decision.Label, nil); err != nil {
            return state, fmt.Errorf("executor: record decide step: %w", err)
        }
    }

    state.CometDone = true
    note := fmt.Sprintf("System note: Comet finished executing Task %d. His result:\n\n%s\n\nConfirm the result to the user.", state.TaskID, reply)
    state.Messages = append(state.Messages, schema.UserMessage(note))
    return state, nil
}
