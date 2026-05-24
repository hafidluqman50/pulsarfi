package repository

import (
	"context"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"gorm.io/gorm"
)

type RedeemApprovalRepository struct {
	DB *gorm.DB
}

func (r *RedeemApprovalRepository) Create(ctx context.Context, proposalID, custodianID int64, txHash *string) error {
	record := model.RedeemApproval{
		ProposalID:  proposalID,
		CustodianID: custodianID,
		TxHash:      txHash,
	}
	return r.DB.WithContext(ctx).Create(&record).Error
}

func (r *RedeemApprovalRepository) FindByProposalID(ctx context.Context, proposalID int64) ([]model.RedeemApproval, error) {
	var records []model.RedeemApproval
	err := r.DB.WithContext(ctx).Where("proposal_id = ?", proposalID).Order("approved_at ASC").Find(&records).Error
	return records, err
}
