package model

import "time"

type SiweNonce struct {
	Address   string    `gorm:"column:address;primaryKey"`
	Nonce     string    `gorm:"column:nonce"`
	ExpiresAt time.Time `gorm:"column:expires_at"`
}

func (SiweNonce) TableName() string { return "siwe_nonces" }
