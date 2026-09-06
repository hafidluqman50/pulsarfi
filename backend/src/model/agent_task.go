package model

import "time"

// AgentTask is a request Supervisor recognized as a distinct Task — a row
// exists for every recognized request, informational or actionable, not
// for every raw chat message (see AgentChatMessage). Task itself carries
// no money-related number: budget/expiry live in the on-chain
// TradePermission (granted separately via ArmTask, never at creation), and
// ticker/side/amount live per-fill in agent_trades, never pre-locked here.
type AgentTask struct {
	ID                 int64      `gorm:"column:id;primaryKey" json:"id"`
	WalletAddress      string     `gorm:"column:wallet_address" json:"wallet_address"`
	SourceMessageID    *int64     `gorm:"column:source_message_id" json:"source_message_id"`
	RawPrompt          *string    `gorm:"column:raw_prompt" json:"raw_prompt"`
	IsActionable       bool       `gorm:"column:is_actionable" json:"is_actionable"`
	Status             string     `gorm:"column:status" json:"status"`
	Summary            *string    `gorm:"column:summary" json:"summary"`
	TriggerDescription *string    `gorm:"column:trigger_description" json:"trigger_description"`
	OnChainTaskID      *int64     `gorm:"column:on_chain_task_id" json:"on_chain_task_id"`
	Paused             bool       `gorm:"column:paused" json:"paused"`
	PausedAt           *time.Time `gorm:"column:paused_at" json:"paused_at"`
	CreatedAt          time.Time  `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt          time.Time  `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	ExecutedAt         *time.Time `gorm:"column:executed_at" json:"executed_at"`
}

func (AgentTask) TableName() string { return "agent_tasks" }
