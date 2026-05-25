package repository

import (
	"context"
	"errors"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"gorm.io/gorm"
)

type StockRepository struct {
	DB *gorm.DB
}

func (r *StockRepository) FindAll(ctx context.Context) ([]model.Stock, error) {
	var stocks []model.Stock
	if err := r.DB.WithContext(ctx).Find(&stocks).Error; err != nil {
		return nil, err
	}
	return stocks, nil
}

func (r *StockRepository) FindMarketReady(ctx context.Context) ([]model.Stock, error) {
	var stocks []model.Stock
	if err := r.DB.WithContext(ctx).
		Where("contract_address IS NOT NULL AND contract_address <> ''").
		Order("id ASC").
		Find(&stocks).Error; err != nil {
		return nil, err
	}
	return stocks, nil
}

func (r *StockRepository) FindByTicker(ctx context.Context, ticker string) (model.Stock, bool, error) {
	var stock model.Stock
	err := r.DB.WithContext(ctx).Where("ticker = ?", ticker).First(&stock).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Stock{}, false, nil
	}
	return stock, err == nil, err
}

func (r *StockRepository) FindByTickerOrIdxTicker(ctx context.Context, ticker string) (model.Stock, bool, error) {
	var stock model.Stock
	err := r.DB.WithContext(ctx).
		Where("ticker = ? OR idx_ticker = ?", ticker, ticker).
		First(&stock).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Stock{}, false, nil
	}
	return stock, err == nil, err
}

func (r *StockRepository) FindByID(ctx context.Context, id int64) (model.Stock, bool, error) {
	var stock model.Stock
	err := r.DB.WithContext(ctx).First(&stock, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.Stock{}, false, nil
	}
	return stock, err == nil, err
}

type StockUpdateContractInput struct {
	ContractAddress string
}

func (r *StockRepository) UpdateContractAddress(ctx context.Context, id int64, input StockUpdateContractInput) error {
	return r.DB.WithContext(ctx).Model(&model.Stock{}).
		Where("id = ?", id).
		Update("contract_address", input.ContractAddress).Error
}
