package model

import "time"

type RedeemProposal struct {
	ID              int64      `gorm:"column:id;primaryKey"`
	OnChainID       int64      `gorm:"column:on_chain_id"`
	StockID         int64      `gorm:"column:stock_id"`
	TokenAmount     string     `gorm:"column:token_amount;type:numeric"`
	IdrxAmount      *string    `gorm:"column:idrx_amount;type:numeric"`
	AttestationHash string     `gorm:"column:attestation_hash"`
	Source          string     `gorm:"column:source"` // retail | institutional
	RequesterID     *int64     `gorm:"column:requester_id"`
	ApprovalCount   int16      `gorm:"column:approval_count"`
	Executed        bool       `gorm:"column:executed"`
	RequestTxHash   *string    `gorm:"column:request_tx_hash"`
	ExecuteTxHash   *string    `gorm:"column:execute_tx_hash"`
	CreatedAt       time.Time  `gorm:"column:created_at;autoCreateTime"`
	ExecutedAt      *time.Time `gorm:"column:executed_at"`

	Stock     Stock      `gorm:"foreignKey:StockID"`
	Requester *Custodian `gorm:"foreignKey:RequesterID"`
}

func (RedeemProposal) TableName() string { return "redeem_proposals" }
