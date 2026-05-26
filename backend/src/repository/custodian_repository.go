package repository

import (
	"context"
	"errors"
	"strings"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"gorm.io/gorm"
)

type CustodianRepository struct {
	DB *gorm.DB
}

func (r *CustodianRepository) FindByWalletAddress(ctx context.Context, address string) (model.Custodian, bool, error) {
	var custodian model.Custodian
	err := r.DB.WithContext(ctx).
		Where("LOWER(wallet_address) = ?", strings.ToLower(address)).
		First(&custodian).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Custodian{}, false, nil
	}
	return custodian, err == nil, err
}

func (r *CustodianRepository) FindAll(ctx context.Context) ([]model.Custodian, error) {
	var custodians []model.Custodian
	return custodians, r.DB.WithContext(ctx).Find(&custodians).Error
}
