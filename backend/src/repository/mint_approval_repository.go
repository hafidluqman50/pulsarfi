package repository

import (
	"context"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"gorm.io/gorm"
)

type MintApprovalRepository struct {
	DB *gorm.DB
}

func (r *MintApprovalRepository) Create(ctx context.Context, proposalID, custodianID int64, txHash *string) (model.MintApproval, error) {
	approval := model.MintApproval{
		ProposalID:  proposalID,
		CustodianID: custodianID,
		TxHash:      txHash,
	}
	if err := r.DB.WithContext(ctx).Create(&approval).Error; err != nil {
		return model.MintApproval{}, err
	}
	return approval, nil
}

func (r *MintApprovalRepository) FindByProposalID(ctx context.Context, proposalID int64) ([]model.MintApproval, error) {
	var approvals []model.MintApproval
	err := r.DB.WithContext(ctx).
		Preload("Custodian").
		Where("proposal_id = ?", proposalID).
		Find(&approvals).Error
	return approvals, err
}
