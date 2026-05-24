package repository

import (
	"context"
	"errors"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"gorm.io/gorm"
)

type RedeemProposalRepository struct {
	DB *gorm.DB
}

func (r *RedeemProposalRepository) FindAll(ctx context.Context) ([]model.RedeemProposal, error) {
	var proposals []model.RedeemProposal
	err := r.DB.WithContext(ctx).
		Preload("Stock").
		Preload("Requester").
		Order("created_at DESC").
		Find(&proposals).Error
	return proposals, err
}

func (r *RedeemProposalRepository) FindPending(ctx context.Context) ([]model.RedeemProposal, error) {
	var proposals []model.RedeemProposal
	err := r.DB.WithContext(ctx).
		Preload("Stock").
		Preload("Requester").
		Where("executed = false").
		Order("created_at ASC").
		Find(&proposals).Error
	return proposals, err
}

func (r *RedeemProposalRepository) FindByOnChainID(ctx context.Context, onChainID int64) (model.RedeemProposal, bool, error) {
	var proposal model.RedeemProposal
	err := r.DB.WithContext(ctx).Where("on_chain_id = ?", onChainID).First(&proposal).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.RedeemProposal{}, false, nil
	}
	return proposal, err == nil, err
}

type RedeemProposalCreateInput struct {
	OnChainID       int64
	StockID         int64
	RequesterID     *int64
	TokenAmount     string
	IdrxAmount      *string
	AttestationHash string
	Source          string
	RequestTxHash   *string
}

func (r *RedeemProposalRepository) Create(ctx context.Context, input RedeemProposalCreateInput) (model.RedeemProposal, error) {
	proposal := model.RedeemProposal{
		OnChainID:       input.OnChainID,
		StockID:         input.StockID,
		RequesterID:     input.RequesterID,
		TokenAmount:     input.TokenAmount,
		IdrxAmount:      input.IdrxAmount,
		AttestationHash: input.AttestationHash,
		Source:          input.Source,
		ApprovalCount:   1,
		RequestTxHash:   input.RequestTxHash,
	}
	return proposal, r.DB.WithContext(ctx).Create(&proposal).Error
}

func (r *RedeemProposalRepository) IncrementApprovalCount(ctx context.Context, onChainID int64) error {
	return r.DB.WithContext(ctx).Model(&model.RedeemProposal{}).
		Where("on_chain_id = ?", onChainID).
		UpdateColumn("approval_count", gorm.Expr("approval_count + 1")).Error
}

func (r *RedeemProposalRepository) CountExecutedLast24h(ctx context.Context) (int64, error) {
	var count int64
	err := r.DB.WithContext(ctx).Model(&model.RedeemProposal{}).
		Where("executed = true AND executed_at >= NOW() - INTERVAL '24 hours'").
		Count(&count).Error
	return count, err
}

func (r *RedeemProposalRepository) MarkExecuted(ctx context.Context, onChainID int64, txHash string) error {
	return r.DB.WithContext(ctx).Model(&model.RedeemProposal{}).
		Where("on_chain_id = ?", onChainID).
		Updates(map[string]any{
			"executed":        true,
			"execute_tx_hash": txHash,
			"executed_at":     gorm.Expr("NOW()"),
		}).Error
}
