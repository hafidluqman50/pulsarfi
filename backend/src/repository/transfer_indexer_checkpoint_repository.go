package repository

import (
	"context"
	"errors"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type TransferIndexerCheckpointRepository struct {
	DB *gorm.DB
}

func (r *TransferIndexerCheckpointRepository) FindByID(
	ctx context.Context,
	id int64,
) (model.TransferIndexerCheckpoint, bool, error) {
	var checkpoint model.TransferIndexerCheckpoint
	err := r.DB.WithContext(ctx).First(&checkpoint, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.TransferIndexerCheckpoint{}, false, nil
	}
	return checkpoint, err == nil, err
}

func (r *TransferIndexerCheckpointRepository) Upsert(ctx context.Context, id int64, lastIndexedBlock int64) error {
	checkpoint := model.TransferIndexerCheckpoint{
		ID:               id,
		LastIndexedBlock: lastIndexedBlock,
	}
	return r.DB.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"last_indexed_block", "updated_at"}),
		}).
		Create(&checkpoint).Error
}
