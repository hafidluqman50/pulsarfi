package model

import "time"

type MintApproval struct {
	ID          int64     `gorm:"column:id;primaryKey"`
	ProposalID  int64     `gorm:"column:proposal_id"`
	CustodianID int64     `gorm:"column:custodian_id"`
	TxHash      *string   `gorm:"column:tx_hash"`
	ApprovedAt  time.Time `gorm:"column:approved_at;autoCreateTime"`

	Proposal  MintProposal `gorm:"foreignKey:ProposalID"`
	Custodian Custodian    `gorm:"foreignKey:CustodianID"`
}

func (MintApproval) TableName() string { return "mint_approvals" }
