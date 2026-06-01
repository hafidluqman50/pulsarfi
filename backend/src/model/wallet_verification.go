package model

import "time"

type WalletVerification struct {
	ID             int64      `gorm:"column:id;primaryKey" json:"id"`
	WalletAddress  string     `gorm:"column:wallet_address" json:"wallet_address"`
	Type           string     `gorm:"column:type" json:"type"`
	Status         string     `gorm:"column:status" json:"status"`
	FullName       *string    `gorm:"column:full_name" json:"full_name"`
	Email          *string    `gorm:"column:email" json:"email"`
	DocumentRef    *string    `gorm:"column:document_ref" json:"document_ref"`
	ApprovalTxHash *string    `gorm:"column:approval_tx_hash" json:"approval_tx_hash"`
	SubmittedAt    time.Time  `gorm:"column:submitted_at;autoCreateTime" json:"submitted_at"`
	VerifiedAt     *time.Time `gorm:"column:verified_at" json:"verified_at"`
	VerifiedBy     *int64     `gorm:"column:verified_by" json:"verified_by"`
}

func (WalletVerification) TableName() string { return "wallet_verifications" }
