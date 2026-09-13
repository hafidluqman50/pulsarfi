package model

import (
	"time"

	"github.com/google/uuid"
)

type AgentChat struct {
	ID          uuid.UUID `gorm:"column:id;primaryKey" json:"id"`
	OwnerWallet string    `gorm:"column:owner_wallet" json:"owner_wallet"`
	Description *string   `gorm:"column:description" json:"description"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (AgentChat) TableName() string { return "agent_chats" }
