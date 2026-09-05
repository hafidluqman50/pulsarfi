package model

import "time"

// AgentChat is a conversation thread with Supervisor. It carries no role
// selection — Executor is a single, unified role, so there is nothing to
// pre-select per docs/plans/ai-agent-role-dispatcher.md.
type AgentChat struct {
	ID          int64     `gorm:"column:id;primaryKey"`
	OwnerWallet string    `gorm:"column:owner_wallet"`
	Description *string   `gorm:"column:description"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (AgentChat) TableName() string { return "agent_chats" }
