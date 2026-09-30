package repository

import (
	"context"
	"time"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type SiweNonceRepository struct {
	DB *gorm.DB
}

func (r *SiweNonceRepository) Upsert(ctx context.Context, address, nonce string, expiresAt time.Time) error {
	entry := model.SiweNonce{Address: address, Nonce: nonce, ExpiresAt: expiresAt}
	return r.DB.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "address"}},
			DoUpdates: clause.AssignmentColumns([]string{"nonce", "expires_at"}),
		}).
		Create(&entry).Error
}

func (r *SiweNonceRepository) Consume(ctx context.Context, address, nonce string, now time.Time) (bool, error) {
	result := r.DB.WithContext(ctx).
		Where("address = ? AND nonce = ? AND expires_at > ?", address, nonce, now).
		Delete(&model.SiweNonce{})
	return result.RowsAffected == 1, result.Error
}

func (r *SiweNonceRepository) DeleteExpired(ctx context.Context, now time.Time) error {
	return r.DB.WithContext(ctx).
		Where("expires_at <= ?", now).
		Delete(&model.SiweNonce{}).Error
}
