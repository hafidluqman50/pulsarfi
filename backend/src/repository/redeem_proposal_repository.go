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
		Order("created_at DESC").
		Find(&proposals).Error
	return proposals, err
}

func (r *RedeemProposalRepository) FindPending(ctx context.Context) ([]model.RedeemProposal, error) {
	var proposals []model.RedeemProposal
	err := r.DB.WithContext(ctx).
		Preload("Stock").
		Where("status = ?", "pending").
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
	OnChainID     int64
	StockID       int64
	TokenAmount   string
	FeeIdrx       string
	UserAddress   string
	RequestTxHash *string
}

func (r *RedeemProposalRepository) Create(ctx context.Context, input RedeemProposalCreateInput) (model.RedeemProposal, error) {
	proposal := model.RedeemProposal{
		OnChainID:     input.OnChainID,
		StockID:       input.StockID,
		TokenAmount:   input.TokenAmount,
		FeeIdrx:       input.FeeIdrx,
		UserAddress:   input.UserAddress,
		Status:        "pending",
		RequestTxHash: input.RequestTxHash,
	}
	return proposal, r.DB.WithContext(ctx).Create(&proposal).Error
}

func (r *RedeemProposalRepository) MarkRejected(ctx context.Context, onChainID int64, txHash string) error {
	return r.DB.WithContext(ctx).Model(&model.RedeemProposal{}).
		Where("on_chain_id = ?", onChainID).
		Updates(map[string]any{
			"status":          "rejected",
			"execute_tx_hash": txHash,
			"executed_at":     gorm.Expr("NOW()"),
		}).Error
}

func (r *RedeemProposalRepository) CountExecutedLast24h(ctx context.Context) (int64, error) {
	var count int64
	err := r.DB.WithContext(ctx).Model(&model.RedeemProposal{}).
		Where("status = ? AND executed_at >= NOW() - INTERVAL '24 hours'", "executed").
		Count(&count).Error
	return count, err
}

func (r *RedeemProposalRepository) MarkExecuted(ctx context.Context, onChainID int64, txHash string) error {
	return r.DB.WithContext(ctx).Model(&model.RedeemProposal{}).
		Where("on_chain_id = ?", onChainID).
		Updates(map[string]any{
			"status":          "executed",
			"execute_tx_hash": txHash,
			"executed_at":     gorm.Expr("NOW()"),
		}).Error
}
