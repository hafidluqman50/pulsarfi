package model

import "time"

type StockAttestation struct {
	ID                int64     `gorm:"column:id;primaryKey"`
	StockID           int64     `gorm:"column:stock_id"`
	CustodianHoldings string    `gorm:"column:custodian_holdings;type:numeric"`
	OnChainSupply     string    `gorm:"column:on_chain_supply;type:numeric"`
	AttestationHash   *string   `gorm:"column:attestation_hash"`
	AttestedAt        time.Time `gorm:"column:attested_at;autoCreateTime"`

	Stock Stock `gorm:"foreignKey:StockID"`
}

func (StockAttestation) TableName() string { return "stock_attestations" }
