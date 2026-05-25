package model

import "time"

type RedeemProposal struct {
	ID            int64      `gorm:"column:id;primaryKey"`
	OnChainID     int64      `gorm:"column:on_chain_id"`
	StockID       int64      `gorm:"column:stock_id"`
	TokenAmount   string     `gorm:"column:token_amount;type:numeric"`
	FeeIdrx       string     `gorm:"column:fee_idrx;type:numeric"`
	UserAddress   string     `gorm:"column:user_address"`
	Status        string     `gorm:"column:status"`
	RequestTxHash *string    `gorm:"column:request_tx_hash"`
	ExecuteTxHash *string    `gorm:"column:execute_tx_hash"`
	CreatedAt     time.Time  `gorm:"column:created_at;autoCreateTime"`
	ExecutedAt    *time.Time `gorm:"column:executed_at"`

	Stock Stock `gorm:"foreignKey:StockID"`
}

func (RedeemProposal) TableName() string { return "redeem_proposals" }
