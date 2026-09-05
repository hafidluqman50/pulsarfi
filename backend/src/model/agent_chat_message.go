package model

import (
	"time"

	"gorm.io/datatypes"
)

// AgentChatMessage is one turn in an AgentChat's transcript. UIProps is a
// frozen snapshot captured at creation time — never re-rendered from live
// AgentTask/AgentSubTask state later, so a message a user already saw can
// never silently change underneath them.
type AgentChatMessage struct {
	ID          int64          `gorm:"column:id;primaryKey"`
	ChatID      int64          `gorm:"column:chat_id"`
	Sender      string         `gorm:"column:sender"`
	ContentType string         `gorm:"column:content_type"`
	Content     string         `gorm:"column:content"`
	UIComponent *string        `gorm:"column:ui_component"`
	UIProps     datatypes.JSON `gorm:"column:ui_props"`
	UIRefTaskID *int64         `gorm:"column:ui_ref_task_id"`
	CreatedAt   time.Time      `gorm:"column:created_at;autoCreateTime"`
}

func (AgentChatMessage) TableName() string { return "agent_chat_messages" }
