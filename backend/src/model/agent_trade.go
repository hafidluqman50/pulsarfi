package model

import "time"

// AgentTrade mirrors one on-chain TradeRecord — one row per fill, never
// per Task (a Task can produce zero, one, or many Trades, potentially
// against different tickers or directions each time).
type AgentTrade struct {
	ID             int64     `gorm:"column:id;primaryKey" json:"id"`
	TaskID         int64     `gorm:"column:task_id" json:"task_id"`
	SubTaskID      int64     `gorm:"column:sub_task_id" json:"sub_task_id"`
	OnChainTradeID *int64    `gorm:"column:on_chain_trade_id" json:"on_chain_trade_id"`
	TxHash         *string   `gorm:"column:tx_hash" json:"tx_hash"`
	Ticker         string    `gorm:"column:ticker" json:"ticker"`
	Side           string    `gorm:"column:side" json:"side"` // "buy" | "sell" — matches stock_transactions.side
	Amount         string    `gorm:"column:amount" json:"amount"`
	Summary        string    `gorm:"column:summary" json:"summary"`
	ExecutedAt     time.Time `gorm:"column:executed_at;autoCreateTime" json:"executed_at"`
}

func (AgentTrade) TableName() string { return "agent_trades" }
