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
		Where("wallet_address = ?", walletAddress).
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

type StockTransactionCreateInput struct {
	StockID       int64
	WalletAddress string
	Side          string
	IdrxAmountRaw string
	StockAmountRaw string
	TxHash        string
	BlockNumber   int64
}

func (r *StockTransactionRepository) Create(ctx context.Context, input StockTransactionCreateInput) (model.StockTransaction, error) {
	tx := model.StockTransaction{
		StockID:       input.StockID,
		WalletAddress: input.WalletAddress,
		Side:          input.Side,
		TxHash:        input.TxHash,
		BlockNumber:   input.BlockNumber,
	}
	return tx, r.DB.WithContext(ctx).Create(&tx).Error
}
