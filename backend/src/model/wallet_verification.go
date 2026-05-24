package model

import "time"

type WalletVerification struct {
	ID            int64      `gorm:"column:id;primaryKey"`
	WalletAddress string     `gorm:"column:wallet_address"`
	Type          string     `gorm:"column:type"`
	Status        string     `gorm:"column:status"`
	DocumentURL   *string    `gorm:"column:document_url"`
	SubmittedAt   time.Time  `gorm:"column:submitted_at;autoCreateTime"`
	VerifiedAt    *time.Time `gorm:"column:verified_at"`
	VerifiedBy    *int64     `gorm:"column:verified_by"`
}

func (WalletVerification) TableName() string { return "wallet_verifications" }
