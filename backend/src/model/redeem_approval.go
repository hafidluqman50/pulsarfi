package model

import "time"

type RedeemApproval struct {
	ID          int64     `gorm:"column:id;primaryKey"`
	ProposalID  int64     `gorm:"column:proposal_id"`
	CustodianID int64     `gorm:"column:custodian_id"`
	TxHash      *string   `gorm:"column:tx_hash"`
	ApprovedAt  time.Time `gorm:"column:approved_at;autoCreateTime"`
}

func (RedeemApproval) TableName() string { return "redeem_approvals" }
