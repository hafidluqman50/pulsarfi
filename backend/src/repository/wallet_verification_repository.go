package repository

import (
	"context"
	"errors"
	"time"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type WalletVerificationRepository struct {
	DB *gorm.DB
}

func (r *WalletVerificationRepository) FindAll(ctx context.Context, status string) ([]model.WalletVerification, error) {
	query := r.DB.WithContext(ctx)
	if status != "" {
		query = query.Where("status = ?", status)
	}
	var records []model.WalletVerification
	return records, query.Order("submitted_at DESC").Find(&records).Error
}

func (r *WalletVerificationRepository) FindByWallet(ctx context.Context, walletAddress string) (model.WalletVerification, bool, error) {
	var record model.WalletVerification
	err := r.DB.WithContext(ctx).Where("wallet_address = ?", walletAddress).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.WalletVerification{}, false, nil
	}
	return record, err == nil, err
}

func (r *WalletVerificationRepository) FindByID(ctx context.Context, id int64) (model.WalletVerification, bool, error) {
	var record model.WalletVerification
	err := r.DB.WithContext(ctx).Where("id = ?", id).First(&record).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.WalletVerification{}, false, nil
	}
	return record, err == nil, err
}

type WalletVerificationCreateInput struct {
	WalletAddress  string
	Type           string
	Status         string
	FullName       *string
	Email          *string
	DocumentRef    *string
	ApprovalTxHash *string
	VerifiedBy     *int64
}

func (r *WalletVerificationRepository) Create(ctx context.Context, input WalletVerificationCreateInput) (model.WalletVerification, error) {
	status := input.Status
	if status == "" {
		status = "pending"
	}
	var verifiedAt *time.Time
	if status == "approved" || status == "rejected" {
		now := time.Now()
		verifiedAt = &now
	}
	record := model.WalletVerification{
		WalletAddress:  input.WalletAddress,
		Type:           input.Type,
		Status:         status,
		FullName:       input.FullName,
		Email:          input.Email,
		DocumentRef:    input.DocumentRef,
		ApprovalTxHash: input.ApprovalTxHash,
		VerifiedAt:     verifiedAt,
		VerifiedBy:     input.VerifiedBy,
	}
	err := r.DB.WithContext(ctx).Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "wallet_address"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"type",
			"status",
			"full_name",
			"email",
			"document_ref",
			"approval_tx_hash",
			"verified_at",
			"verified_by",
		}),
	}).Create(&record).Error
	if err != nil {
		return record, err
	}
	return r.FindByWalletMust(ctx, input.WalletAddress)
}

func (r *WalletVerificationRepository) FindByWalletMust(ctx context.Context, walletAddress string) (model.WalletVerification, error) {
	var record model.WalletVerification
	err := r.DB.WithContext(ctx).Where("wallet_address = ?", walletAddress).First(&record).Error
	return record, err
}

func (r *WalletVerificationRepository) UpdateStatus(ctx context.Context, id int64, status string, verifiedBy *int64) error {
	updates := map[string]any{"status": status}
	if status == "approved" || status == "rejected" {
		updates["verified_at"] = gorm.Expr("NOW()")
		updates["verified_by"] = verifiedBy
	}
	return r.DB.WithContext(ctx).Model(&model.WalletVerification{}).Where("id = ?", id).Updates(updates).Error
}
