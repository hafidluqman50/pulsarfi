package model

import "time"

type MintProposal struct {
	ID              int64      `gorm:"column:id;primaryKey;autoIncrement"`
	OnChainID       int64      `gorm:"column:on_chain_id"`
	StockID         int64      `gorm:"column:stock_id"`
	TokenAmount     string     `gorm:"column:token_amount;type:numeric"`
	IdrxAmount      *string    `gorm:"column:idrx_amount;type:numeric"`
	AttestationHash string     `gorm:"column:attestation_hash"`
	Destination string  `gorm:"column:destination"`
	RequesterID *int64  `gorm:"column:requester_id"`
	Status          string     `gorm:"column:status"`
	RequestTxHash   *string    `gorm:"column:request_tx_hash"`
	ExecuteTxHash   *string    `gorm:"column:execute_tx_hash"`
	CreatedAt       time.Time  `gorm:"column:created_at;autoCreateTime"`
	ExecutedAt      *time.Time `gorm:"column:executed_at"`

	Stock     Stock      `gorm:"foreignKey:StockID"`
	Requester *Custodian `gorm:"foreignKey:RequesterID"`
}

func (MintProposal) TableName() string { return "mint_proposals" }
