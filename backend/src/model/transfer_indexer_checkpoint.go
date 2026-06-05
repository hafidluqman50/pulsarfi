package model

import "time"

type TransferIndexerCheckpoint struct {
	ID               int64     `gorm:"column:id;primaryKey"`
	LastIndexedBlock int64     `gorm:"column:last_indexed_block"`
	UpdatedAt        time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (TransferIndexerCheckpoint) TableName() string { return "transfer_indexer_checkpoints" }
