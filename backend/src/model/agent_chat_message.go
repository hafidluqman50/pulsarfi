package model

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type AgentChatMessage struct {
	ID          int64          `gorm:"column:id;primaryKey" json:"id"`
	ChatID      uuid.UUID      `gorm:"column:chat_id" json:"chat_id"`
	Sender      string         `gorm:"column:sender" json:"sender"`
	ContentType string         `gorm:"column:content_type" json:"content_type"`
	Content     string         `gorm:"column:content" json:"content"`
	UIComponent *string        `gorm:"column:ui_component" json:"ui_component"`
	UIProps     datatypes.JSON `gorm:"column:ui_props" json:"ui_props"`
	UIRefTaskID *int64         `gorm:"column:ui_ref_task_id" json:"ui_ref_task_id"`
	CreatedAt   time.Time      `gorm:"column:created_at;autoCreateTime" json:"created_at"`
}

func (AgentChatMessage) TableName() string { return "agent_chat_messages" }
