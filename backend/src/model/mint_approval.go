package model

import "time"

type MintApproval struct {
	ID          int64     `gorm:"column:id;primaryKey"`
	ProposalID  int64     `gorm:"column:proposal_id"`
	CustodianID int64     `gorm:"column:custodian_id"`
	Type        string    `gorm:"column:type"`
	TxHash      *string   `gorm:"column:tx_hash"`
	AttestedAt  time.Time `gorm:"column:attested_at;autoCreateTime"`

	Proposal  MintProposal `gorm:"foreignKey:ProposalID"`
	Custodian Custodian    `gorm:"foreignKey:CustodianID"`
}

func (MintApproval) TableName() string { return "mint_attestations" }
