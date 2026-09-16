package model

import "time"

// AgentTask is a request Supervisor recognized as a distinct Task — a row
// exists for every recognized request, informational or actionable, not
// for every raw chat message (see AgentChatMessage). Task itself carries
// no money-related number: budget/expiry live in the on-chain
// TradePermission (granted separately via ArmTask, never at creation), and
// ticker/side/amount live per-fill in agent_trades, never pre-locked here.
type AgentTask struct {
	ID                 int64   `gorm:"column:id;primaryKey" json:"id"`
	WalletAddress      string  `gorm:"column:wallet_address" json:"wallet_address"`
	SourceMessageID    *int64  `gorm:"column:source_message_id" json:"source_message_id"`
	RawPrompt          *string `gorm:"column:raw_prompt" json:"raw_prompt"`
	IsActionable       bool    `gorm:"column:is_actionable" json:"is_actionable"`
	Status             string  `gorm:"column:status" json:"status"`
	Summary            *string `gorm:"column:summary" json:"summary"`
	TriggerDescription *string `gorm:"column:trigger_description" json:"trigger_description"`
	OnChainTaskID      *int64  `gorm:"column:on_chain_task_id" json:"on_chain_task_id"`
	// ArmedAt is set only when GrantTradePermission actually succeeded —
	// never inferred from OnChainTaskID, which is set at recognition for
	// every Task and therefore says nothing about arming.
	ArmedAt    *time.Time `gorm:"column:armed_at" json:"armed_at"`
	Paused     bool       `gorm:"column:paused" json:"paused"`
	PausedAt   *time.Time `gorm:"column:paused_at" json:"paused_at"`
	CreatedAt  time.Time  `gorm:"column:created_at;autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time  `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
	ExecutedAt *time.Time `gorm:"column:executed_at" json:"executed_at"`

	// Standing orders (DCA) and horizon guardrails
	IsRecurring       bool       `gorm:"column:is_recurring;default:false" json:"is_recurring"`
	CooldownSec       int32      `gorm:"column:cooldown_sec;default:0" json:"cooldown_sec"`
	MaxPerTrade       int64      `gorm:"column:max_per_trade;default:0" json:"max_per_trade"`
	NextRunAt         *time.Time `gorm:"column:next_run_at" json:"next_run_at"`
	HorizonExpiresAt  *time.Time `gorm:"column:horizon_expires_at" json:"horizon_expires_at"`
	HorizonNotifiedAt *time.Time `gorm:"column:horizon_notified_at" json:"horizon_notified_at"`
	ExitPolicy        string     `gorm:"column:exit_policy;default:'undecided'" json:"exit_policy"`
}

func (AgentTask) TableName() string { return "agent_tasks" }
