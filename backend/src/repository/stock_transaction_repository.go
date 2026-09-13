package repository

import (
	"context"
	"errors"
	"strings"

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
	walletAddress = strings.ToLower(strings.TrimSpace(walletAddress))
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

func (r *StockTransactionRepository) FindByTxHashWalletSide(
	ctx context.Context,
	txHash string,
	walletAddress string,
	side string,
) (model.StockTransaction, bool, error) {
	var tx model.StockTransaction
	walletAddress = strings.ToLower(strings.TrimSpace(walletAddress))
	err := r.DB.WithContext(ctx).
		Preload("Stock").
		Where("tx_hash = ? AND wallet_address = ? AND side = ?", txHash, walletAddress, side).
		First(&tx).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.StockTransaction{}, false, nil
	}
	return tx, err == nil, err
}

func (r *StockTransactionRepository) FindByTxHashLogWalletSide(
	ctx context.Context,
	txHash string,
	logIndex int,
	walletAddress string,
	side string,
) (model.StockTransaction, bool, error) {
	var tx model.StockTransaction
	walletAddress = strings.ToLower(strings.TrimSpace(walletAddress))
	err := r.DB.WithContext(ctx).
		Preload("Stock").
		Where("tx_hash = ? AND log_index = ? AND wallet_address = ? AND side = ?", txHash, logIndex, walletAddress, side).
		First(&tx).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.StockTransaction{}, false, nil
	}
	return tx, err == nil, err
}

type StatsRow struct {
	// gorm column tag required here: GORM's default naming strategy maps
	// "Volume24h" to "volume24h" (no underscore before the digits), not
	// "volume_24h" — the raw SQL below aliases the column "volume_24h", so
	// without this tag Scan silently never matched it and this field stayed
	// 0 always, regardless of what the query actually computed. Found live
	// while investigating why "24h volume" was 0 even after removing the
	// time window entirely — the window was never the real bug.
	Volume24h float64 `gorm:"column:volume_24h"`
	TvlIdrx   float64
}

func (r *StockTransactionRepository) ComputeStats(ctx context.Context) (StatsRow, error) {
	var row StatsRow
	// Volume24h is actually all-time volume, not the last 24h — this demo's
	// own transaction history is real but sparse (weeks between trades), so
	// a strict 24h window sat at 0 far more often than it showed anything
	// real. Field/column name kept as-is (volume_24h) to avoid an unrelated
	// migration+rename for what is otherwise still "trading volume, one
	// number" — only the window changed.
	err := r.DB.WithContext(ctx).Raw(`
		SELECT
			COALESCE(SUM(CASE WHEN side IN ('buy', 'sell') THEN idrx_amount::numeric / 100 ELSE 0 END), 0) AS volume_24h,
			COALESCE(SUM(CASE
				WHEN side = 'buy' THEN idrx_amount::numeric / 100
				WHEN side IN ('sell', 'redeemed') THEN -idrx_amount::numeric / 100
				ELSE 0
			END), 0) AS tvl_idrx
		FROM stock_transactions
	`).Scan(&row).Error
	if row.TvlIdrx < 0 {
		row.TvlIdrx = 0
	}
	return row, err
}

type StockTransactionCreateInput struct {
	StockID         int64
	WalletAddress   string
	Side            string
	IdrxAmount      string
	StockAmount     string
	ProtocolFeeIdrx string
	TxHash          string
	BlockNumber     int64
	LogIndex        int
}

func (r *StockTransactionRepository) Create(ctx context.Context, input StockTransactionCreateInput) (model.StockTransaction, error) {
	tx := model.StockTransaction{
		StockID:         input.StockID,
		WalletAddress:   input.WalletAddress,
		Side:            input.Side,
		IdrxAmount:      input.IdrxAmount,
		StockAmount:     input.StockAmount,
		ProtocolFeeIdrx: input.ProtocolFeeIdrx,
		TxHash:          input.TxHash,
		BlockNumber:     input.BlockNumber,
		LogIndex:        input.LogIndex,
	}
	return tx, r.DB.WithContext(ctx).Create(&tx).Error
}

func (r *StockTransactionRepository) CreateMany(
	ctx context.Context,
	inputs []StockTransactionCreateInput,
) ([]model.StockTransaction, error) {
	txs := make([]model.StockTransaction, 0, len(inputs))
	for _, input := range inputs {
		txs = append(txs, model.StockTransaction{
			StockID:         input.StockID,
			WalletAddress:   input.WalletAddress,
			Side:            input.Side,
			IdrxAmount:      input.IdrxAmount,
			StockAmount:     input.StockAmount,
			ProtocolFeeIdrx: input.ProtocolFeeIdrx,
			TxHash:          input.TxHash,
			BlockNumber:     input.BlockNumber,
			LogIndex:        input.LogIndex,
		})
	}

	if err := r.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		return tx.Create(&txs).Error
	}); err != nil {
		return nil, err
	}
	return txs, nil
}
