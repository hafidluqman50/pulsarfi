package model

import "time"

type Stock struct {
	ID              int64     `gorm:"column:id;primaryKey"`
	Ticker          string    `gorm:"column:ticker"`
	StockName       string    `gorm:"column:stock_name"`
	IdxTicker       string    `gorm:"column:idx_ticker"`
	Sector          *string   `gorm:"column:sector"`
	ContractAddress *string   `gorm:"column:contract_address"`
	CreatedAt       time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (Stock) TableName() string { return "stocks" }
