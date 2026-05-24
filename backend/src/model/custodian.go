package model

import "time"

type Custodian struct {
	ID            int64     `gorm:"column:id;primaryKey"`
	WalletAddress string    `gorm:"column:wallet_address"`
	Name          string    `gorm:"column:name"`
	Email         *string   `gorm:"column:email"`
	CreatedAt     time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (Custodian) TableName() string { return "custodians" }
