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
	TokenAmountRaw  string
	IdrxAmountRaw   *string
	AttestationHash string
	Destination     string
	RequestTxHash   *string
}

func (r *MintProposalRepository) Create(ctx context.Context, input MintProposalCreateInput) (model.MintProposal, error) {
	proposal := model.MintProposal{
		OnChainID:       input.OnChainID,
		StockID:         input.StockID,
		RequesterID:     input.RequesterID,
		AttestationHash: input.AttestationHash,
		Destination:     input.Destination,
		ApprovalCount:   1,
		RequestTxHash:   input.RequestTxHash,
	}
	if err := r.DB.WithContext(ctx).Create(&proposal).Error; err != nil {
		return model.MintProposal{}, err
	}
	return proposal, nil
}

func (r *MintProposalRepository) IncrementApprovalCount(ctx context.Context, id int64) error {
	return r.DB.WithContext(ctx).Model(&model.MintProposal{}).
		Where("id = ?", id).
		UpdateColumn("approval_count", gorm.Expr("approval_count + 1")).Error
}

func (r *MintProposalRepository) MarkExecuted(ctx context.Context, id int64, txHash string) error {
	return r.DB.WithContext(ctx).Model(&model.MintProposal{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"executed":        true,
			"execute_tx_hash": txHash,
			"executed_at":     gorm.Expr("NOW()"),
		}).Error
}
