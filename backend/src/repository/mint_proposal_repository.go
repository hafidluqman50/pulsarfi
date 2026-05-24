package repository

import (
	"context"
	"errors"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"gorm.io/gorm"
)

type MintProposalRepository struct {
	DB *gorm.DB
}

func (r *MintProposalRepository) FindPending(ctx context.Context) ([]model.MintProposal, error) {
	var proposals []model.MintProposal
	err := r.DB.WithContext(ctx).
		Preload("Stock").
		Preload("Requester").
		Where("executed = false").
		Order("created_at ASC").
		Find(&proposals).Error
	return proposals, err
}

func (r *MintProposalRepository) FindAll(ctx context.Context) ([]model.MintProposal, error) {
	var proposals []model.MintProposal
	err := r.DB.WithContext(ctx).
		Preload("Stock").
		Preload("Requester").
		Order("created_at DESC").
		Find(&proposals).Error
	return proposals, err
}

func (r *MintProposalRepository) FindByOnChainID(ctx context.Context, onChainID int64) (model.MintProposal, bool, error) {
	var proposal model.MintProposal
	err := r.DB.WithContext(ctx).Where("on_chain_id = ?", onChainID).First(&proposal).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.MintProposal{}, false, nil
	}
	return proposal, err == nil, err
}

type MintProposalCreateInput struct {
	OnChainID       int64
	StockID         int64
	RequesterID     *int64
	TokenAmount     string
	IdrxAmount      *string
	AttestationHash string
	Destination     string
	RequestTxHash   *string
}

func (r *MintProposalRepository) Create(ctx context.Context, input MintProposalCreateInput) (model.MintProposal, error) {
	proposal := model.MintProposal{
		OnChainID:       input.OnChainID,
		StockID:         input.StockID,
		RequesterID:     input.RequesterID,
		TokenAmount:     input.TokenAmount,
		IdrxAmount:      input.IdrxAmount,
		AttestationHash: input.AttestationHash,
		Destination:     input.Destination,
		ApprovalCount:   1,
		RequestTxHash:   input.RequestTxHash,
	}
	return proposal, r.DB.WithContext(ctx).Create(&proposal).Error
}

func (r *MintProposalRepository) IncrementApprovalCount(ctx context.Context, onChainID int64) error {
	return r.DB.WithContext(ctx).Model(&model.MintProposal{}).
		Where("on_chain_id = ?", onChainID).
		UpdateColumn("approval_count", gorm.Expr("approval_count + 1")).Error
}

func (r *MintProposalRepository) CountExecutedLast24h(ctx context.Context) (int64, error) {
	var count int64
	err := r.DB.WithContext(ctx).Model(&model.MintProposal{}).
		Where("executed = true AND executed_at >= NOW() - INTERVAL '24 hours'").
		Count(&count).Error
	return count, err
}

func (r *MintProposalRepository) SumIdrxLast24h(ctx context.Context) (string, error) {
	type result struct{ Total *string }
	var res result
	err := r.DB.WithContext(ctx).Model(&model.MintProposal{}).
		Select("COALESCE(SUM(idrx_amount)::text, '0') AS total").
		Where("executed = true AND idrx_amount IS NOT NULL AND executed_at >= NOW() - INTERVAL '24 hours'").
		Scan(&res).Error
	if res.Total == nil {
		return "0", err
	}
	return *res.Total, err
}

func (r *MintProposalRepository) MarkExecuted(ctx context.Context, onChainID int64, txHash string) error {
	return r.DB.WithContext(ctx).Model(&model.MintProposal{}).
		Where("on_chain_id = ?", onChainID).
		Updates(map[string]any{
			"executed":        true,
			"execute_tx_hash": txHash,
			"executed_at":     gorm.Expr("NOW()"),
		}).Error
}
