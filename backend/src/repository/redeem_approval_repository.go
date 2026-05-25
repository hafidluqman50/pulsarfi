package repository

import (
	"context"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"gorm.io/gorm"
)

type RedeemApprovalRepository struct {
	DB *gorm.DB
}

func (r *RedeemApprovalRepository) Create(ctx context.Context, proposalID, custodianID int64, voteType string, txHash *string) error {
	record := model.RedeemApproval{
		ProposalID:  proposalID,
		CustodianID: custodianID,
		Type:        voteType,
		TxHash:      txHash,
	}
	return r.DB.WithContext(ctx).Create(&record).Error
}

func (r *RedeemApprovalRepository) FindByProposalID(ctx context.Context, proposalID int64) ([]model.RedeemApproval, error) {
	var records []model.RedeemApproval
	err := r.DB.WithContext(ctx).Where("proposal_id = ?", proposalID).Order("attested_at ASC").Find(&records).Error
	return records, err
}

func (r *RedeemApprovalRepository) CountByProposalID(ctx context.Context, proposalID int64, voteType string) (int64, error) {
	var count int64
	err := r.DB.WithContext(ctx).
		Model(&model.RedeemApproval{}).
		Where("proposal_id = ? AND type = ?", proposalID, voteType).
		Count(&count).Error
	return count, err
}
