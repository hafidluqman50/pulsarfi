package repository

import (
	"context"
	"errors"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"gorm.io/gorm"
)

type StockTransactionRepository struct {
	DB *gorm.DB
}

func (r *StockTransactionRepository) FindByStockID(ctx context.Context, stockID int64, limit int) ([]model.StockTransaction, error) {
	var txs []model.StockTransaction
	err := r.DB.WithContext(ctx).
		Where("stock_id = ?", stockID).
		Order("block_number DESC").
		Limit(limit).
		Find(&txs).Error
	return txs, err
}

func (r *StockTransactionRepository) FindByWallet(ctx context.Context, walletAddress string) ([]model.StockTransaction, error) {
	var txs []model.StockTransaction
	err := r.DB.WithContext(ctx).
		Preload("Stock").
		Where("LOWER(wallet_address) = LOWER(?)", walletAddress).
		Order("block_number DESC").
		Find(&txs).Error
	return txs, err
}

func (r *StockTransactionRepository) ExistsByTxHash(ctx context.Context, txHash string) (bool, error) {
	var tx model.StockTransaction
	err := r.DB.WithContext(ctx).Where("tx_hash = ?", txHash).First(&tx).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return false, nil
	}
	return err == nil, err
}

func (r *StockTransactionRepository) FindByTxHash(ctx context.Context, txHash string) (model.StockTransaction, bool, error) {
	var tx model.StockTransaction
	err := r.DB.WithContext(ctx).
		Preload("Stock").
		Where("tx_hash = ?", txHash).
		First(&tx).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.StockTransaction{}, false, nil
	}
	return tx, err == nil, err
}

type StatsRow struct {
	Volume24h float64
	TvlIdrx   float64
}

func (r *StockTransactionRepository) ComputeStats(ctx context.Context) (StatsRow, error) {
	var row StatsRow
	err := r.DB.WithContext(ctx).Raw(`
		SELECT
			COALESCE(SUM(CASE WHEN created_at >= NOW() - INTERVAL '24 hours' THEN idrx_amount::numeric / 100 ELSE 0 END), 0) AS volume_24h,
			COALESCE(SUM(CASE WHEN side = 'buy' THEN idrx_amount::numeric / 100 ELSE -idrx_amount::numeric / 100 END), 0)  AS tvl_idrx
		FROM stock_transactions
	`).Scan(&row).Error
	if row.TvlIdrx < 0 {
		row.TvlIdrx = 0
	}
	return row, err
}

type StockTransactionCreateInput struct {
	StockID       int64
	WalletAddress string
	Side          string
	IdrxAmount    string
	StockAmount   string
	TxHash        string
	BlockNumber   int64
}

func (r *StockTransactionRepository) Create(ctx context.Context, input StockTransactionCreateInput) (model.StockTransaction, error) {
	tx := model.StockTransaction{
		StockID:       input.StockID,
		WalletAddress: input.WalletAddress,
		Side:          input.Side,
		IdrxAmount:    input.IdrxAmount,
		StockAmount:   input.StockAmount,
		TxHash:        input.TxHash,
		BlockNumber:   input.BlockNumber,
	}
	return tx, r.DB.WithContext(ctx).Create(&tx).Error
}
