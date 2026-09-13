package repository

import (
	"context"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"gorm.io/gorm"
)

type AgentTradeRepository struct {
	DB *gorm.DB
}

func (r *AgentTradeRepository) FindByTaskID(ctx context.Context, taskID int64) ([]model.AgentTrade, error) {
	var trades []model.AgentTrade
	err := r.DB.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("executed_at ASC").
		Find(&trades).Error
	return trades, err
}

func (r *AgentTradeRepository) Create(ctx context.Context, trade model.AgentTrade) (model.AgentTrade, error) {
	return trade, r.DB.WithContext(ctx).Create(&trade).Error
}
