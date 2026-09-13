package repository

import (
	"context"
	"errors"
	"time"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type AgentCheckpointRepository struct {
	DB *gorm.DB
}

func (r *AgentCheckpointRepository) FindByID(
	ctx context.Context,
	checkpointID string,
) (model.AgentCheckpoint, bool, error) {
	var checkpoint model.AgentCheckpoint
	err := r.DB.WithContext(ctx).First(&checkpoint, "checkpoint_id = ?", checkpointID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.AgentCheckpoint{}, false, nil
	}
	return checkpoint, err == nil, err
}

func (r *AgentCheckpointRepository) Upsert(ctx context.Context, checkpointID string, data []byte) error {
	checkpoint := model.AgentCheckpoint{
		CheckpointID: checkpointID,
		Data:         data,
		UpdatedAt:    time.Now(),
	}
	return r.DB.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "checkpoint_id"}},
			DoUpdates: clause.AssignmentColumns([]string{"data", "updated_at"}),
		}).
		Create(&checkpoint).Error
}

func (r *AgentCheckpointRepository) Delete(ctx context.Context, checkpointID string) error {
	return r.DB.WithContext(ctx).
		Where("checkpoint_id = ?", checkpointID).
		Delete(&model.AgentCheckpoint{}).Error
}
