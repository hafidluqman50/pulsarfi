package model

import (
	"math/big"
	"time"
)

type StockTransaction struct {
	ID            int64     `gorm:"column:id;primaryKey"`
	StockID       int64     `gorm:"column:stock_id"`
	WalletAddress string    `gorm:"column:wallet_address"`
	Side          string    `gorm:"column:side"`
	IdrxAmount    *big.Int  `gorm:"column:idrx_amount;type:numeric"`
	StockAmount   *big.Int  `gorm:"column:stock_amount;type:numeric"`
	TxHash        string    `gorm:"column:tx_hash"`
	BlockNumber   int64     `gorm:"column:block_number"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"`

	Stock Stock `gorm:"foreignKey:StockID"`
}

func (StockTransaction) TableName() string { return "stock_transactions" }
