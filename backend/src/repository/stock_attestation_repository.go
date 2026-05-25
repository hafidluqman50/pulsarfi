package repository

import (
	"context"
	"errors"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"gorm.io/gorm"
)

type StockAttestationRepository struct {
	DB *gorm.DB
}

type StockAttestationCreateInput struct {
	StockID           int64
	CustodianHoldings string
	OnChainSupply     string
	AttestationHash   *string
}

func (r *StockAttestationRepository) Create(ctx context.Context, input StockAttestationCreateInput) (model.StockAttestation, error) {
	record := model.StockAttestation{
		StockID:           input.StockID,
		CustodianHoldings: input.CustodianHoldings,
		OnChainSupply:     input.OnChainSupply,
		AttestationHash:   input.AttestationHash,
	}
	return record, r.DB.WithContext(ctx).Create(&record).Error
}

func (r *StockAttestationRepository) FindLatestPerStock(ctx context.Context) ([]model.StockAttestation, error) {
	type result struct{ ID int64 }
	var rows []result
	if err := r.DB.WithContext(ctx).
		Model(&model.StockAttestation{}).
		Select("MAX(id) AS id").
		Group("stock_id").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return nil, nil
	}
	ids := make([]int64, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	var records []model.StockAttestation
	err := r.DB.WithContext(ctx).
		Preload("Stock").
		Where("id IN ?", ids).
		Find(&records).Error
	return records, err
}

func (r *StockAttestationRepository) FindLatestByStockID(ctx context.Context, stockID int64) (model.StockAttestation, bool, error) {
	var record model.StockAttestation
	err := r.DB.WithContext(ctx).
		Where("stock_id = ?", stockID).
		Order("id DESC").
		First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.StockAttestation{}, false, nil
	}
	return record, err == nil, err
}

func (r *StockAttestationRepository) ExistsByAttestationHash(ctx context.Context, hash string) (bool, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&model.StockAttestation{}).
		Where("attestation_hash = ?", hash).
		Count(&count).Error
	return count > 0, err
}

func (r *StockAttestationRepository) SumLatestHoldings(ctx context.Context) (string, error) {
	type idResult struct{ ID int64 }
	var rows []idResult
	if err := r.DB.WithContext(ctx).
		Model(&model.StockAttestation{}).
		Select("MAX(id) AS id").
		Group("stock_id").
		Scan(&rows).Error; err != nil {
		return "0", err
	}
	if len(rows) == 0 {
		return "0", nil
	}
	ids := make([]int64, len(rows))
	for i, row := range rows {
		ids[i] = row.ID
	}
	type sumResult struct{ Total *string }
	var res sumResult
	err := r.DB.WithContext(ctx).
		Model(&model.StockAttestation{}).
		Select("COALESCE(SUM(custodian_holdings)::text, '0') AS total").
		Where("id IN ?", ids).
		Scan(&res).Error
	if res.Total == nil {
		return "0", err
	}
	return *res.Total, err
}

func (r *StockAttestationRepository) FindByStockID(ctx context.Context, stockID int64) ([]model.StockAttestation, error) {
	var records []model.StockAttestation
	err := r.DB.WithContext(ctx).
		Where("stock_id = ?", stockID).
		Order("attested_at DESC").
		Find(&records).Error
	return records, err
}
