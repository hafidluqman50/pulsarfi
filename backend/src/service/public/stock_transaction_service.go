package public

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
)

var ErrInvalidTransactionSide = errors.New("invalid transaction side")
var ErrWalletAddressRequired = errors.New("wallet address is required")

type StockTransactionService struct {
	Stocks       *repository.StockRepository
	Transactions *repository.StockTransactionRepository
}

type RecordStockTransactionRequest struct {
	Ticker        string
	TxHash        string
	WalletAddress string
	Side          string
	IdrxAmount    string
	StockAmount   string
	BlockNumber   int64
}

type StockTransactionResponse struct {
	ID            int64     `json:"id"`
	StockID       int64     `json:"stock_id"`
	Ticker        string    `json:"ticker"`
	StockName     string    `json:"stock_name"`
	IdxTicker     string    `json:"idx_ticker"`
	WalletAddress string    `json:"wallet_address"`
	Side          string    `json:"side"`
	IdrxAmount    string    `json:"idrx_amount"`
	StockAmount   string    `json:"stock_amount"`
	TxHash        string    `json:"tx_hash"`
	BlockNumber   int64     `json:"block_number"`
	CreatedAt     time.Time `json:"created_at"`
}

func (s *StockTransactionService) Record(ctx context.Context, req RecordStockTransactionRequest) (model.StockTransaction, bool, error) {
	side := strings.ToLower(req.Side)
	if side != "buy" && side != "sell" {
		return model.StockTransaction{}, false, ErrInvalidTransactionSide
	}

	existing, found, err := s.Transactions.FindByTxHash(ctx, req.TxHash)
	if err != nil {
		return model.StockTransaction{}, false, err
	}
	if found {
		return existing, false, nil
	}

	stock, found, err := s.Stocks.FindByTickerOrIdxTicker(ctx, req.Ticker)
	if err != nil {
		return model.StockTransaction{}, false, err
	}
	if !found {
		return model.StockTransaction{}, false, ErrStockNotFound
	}

	tx, err := s.Transactions.Create(ctx, repository.StockTransactionCreateInput{
		StockID:       stock.ID,
		WalletAddress: req.WalletAddress,
		Side:          side,
		IdrxAmount:    req.IdrxAmount,
		StockAmount:   req.StockAmount,
		TxHash:        req.TxHash,
		BlockNumber:   req.BlockNumber,
	})
	return tx, err == nil, err
}

func (s *StockTransactionService) ListByWallet(ctx context.Context, walletAddress string) ([]StockTransactionResponse, error) {
	if strings.TrimSpace(walletAddress) == "" {
		return nil, ErrWalletAddressRequired
	}

	transactions, err := s.Transactions.FindByWallet(ctx, walletAddress)
	if err != nil {
		return nil, err
	}

	items := make([]StockTransactionResponse, 0, len(transactions))
	for _, tx := range transactions {
		items = append(items, stockTransactionResponse(tx))
	}
	return items, nil
}

func stockTransactionResponse(tx model.StockTransaction) StockTransactionResponse {
	return StockTransactionResponse{
		ID:            tx.ID,
		StockID:       tx.StockID,
		Ticker:        tx.Stock.Ticker,
		StockName:     tx.Stock.StockName,
		IdxTicker:     tx.Stock.IdxTicker,
		WalletAddress: tx.WalletAddress,
		Side:          tx.Side,
		IdrxAmount:    tx.IdrxAmount,
		StockAmount:   tx.StockAmount,
		TxHash:        tx.TxHash,
		BlockNumber:   tx.BlockNumber,
		CreatedAt:     tx.CreatedAt,
	}
}
