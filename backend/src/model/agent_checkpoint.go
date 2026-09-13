package model

import "time"

type AgentCheckpoint struct {
	CheckpointID string    `gorm:"column:checkpoint_id;primaryKey" json:"checkpoint_id"`
	Data         []byte    `gorm:"column:data" json:"data"`
	UpdatedAt    time.Time `gorm:"column:updated_at;autoUpdateTime" json:"updated_at"`
}

func (AgentCheckpoint) TableName() string { return "agent_checkpoints" }
