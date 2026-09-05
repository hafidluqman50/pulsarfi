package agent

type TradeSide string

const (
	TradeSideBuy  TradeSide = "buy"
	TradeSideSell TradeSide = "sell"
)

// TradeIntent is what Executor's own LLM call decides this cycle — ticker,
// side, and amount are call-time outputs, never pre-locked on the Task
// (docs/plans/agent-task-manager-rebuild.md §3a). Amount is IDRX-equivalent,
// clamped server-side to the armed TradePermission's remaining headroom.
type TradeIntent struct {
	Ticker string
	Side   TradeSide
	Amount string
}
