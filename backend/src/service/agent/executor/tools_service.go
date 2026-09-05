package executor

import (
	"context"
	"fmt"
	"math/big"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent"
)

type holdingsRequest struct {
	Wallet string `json:"wallet" jsonschema_description:"The wallet address to check current holdings for — use exactly the wallet address given in the server-supplied data, never a value you invent."`
}

type holdingsResponse struct {
	Holdings []string `json:"holdings" jsonschema_description:"Ticker symbols the wallet currently holds a position in, across the whole portfolio, not just the ticker under consideration."`
}

// newHoldingsTool is a read-only capability, so letting the LLM choose when
// to call it (e.g. to check overall portfolio concentration before sizing)
// carries no fund-movement risk, unlike submit_trade below.
func newHoldingsTool(portfolio PortfolioReader) (tool.InvokableTool, error) {
	return utils.InferTool(
		"get_portfolio_holdings",
		"Returns every ticker the wallet currently holds a position in, so you can weigh concentration/diversification before sizing an action.",
		func(ctx context.Context, req holdingsRequest) (holdingsResponse, error) {
			holdings, err := portfolio.Holdings(ctx, req.Wallet)
			if err != nil {
				return holdingsResponse{}, err
			}
			tickers := make([]string, 0, len(holdings))
			for _, h := range holdings {
				tickers = append(tickers, h.Ticker)
			}
			return holdingsResponse{Holdings: tickers}, nil
		},
	)
}

type submitTradeRequest struct {
	Ticker    string `json:"ticker" jsonschema_description:"The exact ticker to trade — your own conclusion, matching the Task's trigger. Must be a real, existing ticker."`
	Side      string `json:"side" jsonschema_description:"buy or sell — your own conclusion, matching the Task's trigger."`
	Amount    string `json:"amount" jsonschema_description:"IDRX-equivalent amount to act with this cycle. Clamped server-side to the Task's remaining TradePermission headroom regardless of what you request."`
	Reasoning string `json:"reasoning" jsonschema_description:"Short, specific reasoning citing the evidence and severity that justified this exact ticker, side, and amount — never a generic restatement of the trigger condition."`
}

type submitTradeResponse struct {
	TxHash string `json:"tx_hash"`
}

// clampToRemaining returns the smaller of amount/remaining as a decimal
// string, treating an unparseable amount as zero — never lets a malformed
// LLM-supplied number fall through as an unbounded value.
func clampToRemaining(amount, remaining string) string {
	amountInt, ok := new(big.Int).SetString(amount, 10)
	if !ok {
		return "0"
	}
	remainingInt, ok := new(big.Int).SetString(remaining, 10)
	if !ok {
		return "0"
	}
	if amountInt.Cmp(remainingInt) > 0 {
		return remainingInt.String()
	}
	return amountInt.String()
}

// newSubmitTradeTool is the only side-effecting capability Executor has —
// call it only once a decision to act now has actually been made. Ticker,
// side, and amount are all call-time outputs of Executor's own LLM call
// (never pre-locked on the Task, docs/plans/agent-task-manager-rebuild.md
// §3a) — clamped/validated here, never trusted as given: amount is capped
// to the live remaining TradePermission, and ticker is checked against the
// real stock catalog (StockLookup) before it ever reaches ExecuteTrade —
// the same prompt-injection hardening applied to the chart lens catalog
// (docs/plans/agent-task-manager-code-implementation.md §7.G).
func newSubmitTradeTool(exec TaskExecutor, stocks StockLookup) (tool.InvokableTool, error) {
	return utils.InferTool(
		"submit_trade",
		"Submits a trade on-chain against the current Task's armed TradePermission. Call this only once you've decided the trigger is confirmed and an action should fire now — you supply ticker, side, and amount yourself, none of it is pre-set on the Task.",
		func(ctx context.Context, req submitTradeRequest) (submitTradeResponse, error) {
			rc, ok := agent.RunContextFrom(ctx)
			if !ok {
				return submitTradeResponse{}, fmt.Errorf("submit_trade: no run context bound to this call")
			}
			if rc.OnChainTaskID == nil {
				return submitTradeResponse{}, fmt.Errorf("submit_trade: task is not armed, no TradePermission exists yet")
			}

			if _, found, err := stocks.FindByTicker(ctx, req.Ticker); err != nil {
				return submitTradeResponse{}, fmt.Errorf("submit_trade: lookup ticker: %w", err)
			} else if !found {
				return submitTradeResponse{}, fmt.Errorf("submit_trade: %q is not a known ticker, refusing", req.Ticker)
			}

			remaining, err := exec.TradePermissionRemaining(ctx, uint(*rc.OnChainTaskID))
			if err != nil {
				return submitTradeResponse{}, fmt.Errorf("submit_trade: read remaining TradePermission: %w", err)
			}
			amount := clampToRemaining(req.Amount, remaining)

			side := agent.TradeSideSell
			if req.Side == "buy" {
				side = agent.TradeSideBuy
			}
			intent := agent.TradeIntent{Ticker: req.Ticker, Side: side, Amount: amount}

			decideRow, err := rc.Recorder.Record(ctx, "executor", "decide", "done", req.Reasoning, map[string]any{
				"ticker": intent.Ticker,
				"side":   intent.Side,
				"amount": intent.Amount,
			})
			if err != nil {
				return submitTradeResponse{}, fmt.Errorf("submit_trade: record decide: %w", err)
			}

			var reasoningHash [32]byte = common.HexToHash(decideRow.DecisionHash)
			txHash, tradeID, err := exec.ExecuteTrade(ctx, uint(*rc.OnChainTaskID), intent, reasoningHash)
			if err != nil {
				_, _ = rc.Recorder.Record(ctx, "executor", "execute", "failed", err.Error(), nil)
				return submitTradeResponse{}, fmt.Errorf("submit_trade: execute: %w", err)
			}

			if _, err := rc.Recorder.Record(ctx, "executor", "execute", "done", "on-chain execution submitted", map[string]any{
				"tx_hash":  txHash,
				"trade_id": tradeID,
			}); err != nil {
				return submitTradeResponse{}, fmt.Errorf("submit_trade: record execute: %w", err)
			}

			return submitTradeResponse{TxHash: txHash}, nil
		},
	)
}
