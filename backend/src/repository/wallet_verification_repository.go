package repository

import (
	"context"
	"errors"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"gorm.io/gorm"
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

type WalletVerificationCreateInput struct {
	WalletAddress string
	Type          string
	DocumentURL   *string
}

func (r *WalletVerificationRepository) Create(ctx context.Context, input WalletVerificationCreateInput) (model.WalletVerification, error) {
	record := model.WalletVerification{
		WalletAddress: input.WalletAddress,
		Type:          input.Type,
		Status:        "pending",
		DocumentURL:   input.DocumentURL,
	}
	return record, r.DB.WithContext(ctx).Create(&record).Error
}

func (r *WalletVerificationRepository) UpdateStatus(ctx context.Context, id int64, status string, verifiedBy *int64) error {
	updates := map[string]any{"status": status}
	if status == "approved" || status == "rejected" {
		updates["verified_at"] = gorm.Expr("NOW()")
		updates["verified_by"] = verifiedBy
	}
	return r.DB.WithContext(ctx).Model(&model.WalletVerification{}).Where("id = ?", id).Updates(updates).Error
}
