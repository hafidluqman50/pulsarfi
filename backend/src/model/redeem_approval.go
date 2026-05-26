package model

import "time"

type RedeemApproval struct {
	ID          int64     `gorm:"column:id;primaryKey"`
	ProposalID  int64     `gorm:"column:proposal_id"`
	CustodianID int64     `gorm:"column:custodian_id"`
	Type        string    `gorm:"column:type"`
	TxHash      *string   `gorm:"column:tx_hash"`
	AttestedAt  time.Time `gorm:"column:attested_at;autoCreateTime"`
}

func (RedeemApproval) TableName() string { return "redeem_attestations" }
