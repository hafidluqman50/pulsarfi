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
var ErrTransferAddressRequired = errors.New("transfer from and to addresses are required")
var ErrTransferSameAddress = errors.New("transfer from and to addresses must differ")
var ErrTransferRecordIncomplete = errors.New("transfer transaction record is incomplete")

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
	LogIndex      int
}

type RecordTransferRequest struct {
	Ticker      string
	TxHash      string
	FromAddress string
	ToAddress   string
	IdrxAmount  string
	StockAmount string
	BlockNumber int64
	LogIndex    int
}

type TransferTransactionResponse struct {
	Out StockTransactionResponse `json:"out"`
	In  StockTransactionResponse `json:"in"`
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
	LogIndex      int       `json:"log_index"`
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
		WalletAddress: strings.ToLower(strings.TrimSpace(req.WalletAddress)),
		Side:          side,
		IdrxAmount:    req.IdrxAmount,
		StockAmount:   req.StockAmount,
		TxHash:        req.TxHash,
		BlockNumber:   req.BlockNumber,
		LogIndex:      req.LogIndex,
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

func (s *StockTransactionService) RecordTransfer(
	ctx context.Context,
	req RecordTransferRequest,
) (TransferTransactionResponse, bool, error) {
	fromAddress := strings.ToLower(strings.TrimSpace(req.FromAddress))
	toAddress := strings.ToLower(strings.TrimSpace(req.ToAddress))
	if fromAddress == "" || toAddress == "" {
		return TransferTransactionResponse{}, false, ErrTransferAddressRequired
	}
	if fromAddress == toAddress {
		return TransferTransactionResponse{}, false, ErrTransferSameAddress
	}

	stock, found, err := s.Stocks.FindByTickerOrIdxTicker(ctx, req.Ticker)
	if err != nil {
		return TransferTransactionResponse{}, false, err
	}
	if !found {
		return TransferTransactionResponse{}, false, ErrStockNotFound
	}

	existingOut, outFound, err := s.Transactions.FindByTxHashLogWalletSide(
		ctx,
		req.TxHash,
		req.LogIndex,
		fromAddress,
		"transfer-out",
	)
	if err != nil {
		return TransferTransactionResponse{}, false, err
	}
	existingIn, inFound, err := s.Transactions.FindByTxHashLogWalletSide(
		ctx,
		req.TxHash,
		req.LogIndex,
		toAddress,
		"transfer-in",
	)
	if err != nil {
		return TransferTransactionResponse{}, false, err
	}
	if outFound && inFound {
		return TransferTransactionResponse{
			Out: stockTransactionResponse(existingOut),
			In:  stockTransactionResponse(existingIn),
		}, false, nil
	}
	if outFound || inFound {
		return TransferTransactionResponse{}, false, ErrTransferRecordIncomplete
	}

	rows, err := s.Transactions.CreateMany(ctx, []repository.StockTransactionCreateInput{
		{
			StockID:       stock.ID,
			WalletAddress: fromAddress,
			Side:          "transfer-out",
			IdrxAmount:    req.IdrxAmount,
			StockAmount:   req.StockAmount,
			TxHash:        req.TxHash,
			BlockNumber:   req.BlockNumber,
			LogIndex:      req.LogIndex,
		},
		{
			StockID:       stock.ID,
			WalletAddress: toAddress,
			Side:          "transfer-in",
			IdrxAmount:    req.IdrxAmount,
			StockAmount:   req.StockAmount,
			TxHash:        req.TxHash,
			BlockNumber:   req.BlockNumber,
			LogIndex:      req.LogIndex,
		},
	})
	if err != nil {
		return TransferTransactionResponse{}, false, err
	}
	rows[0].Stock = stock
	rows[1].Stock = stock

	return TransferTransactionResponse{
		Out: stockTransactionResponse(rows[0]),
		In:  stockTransactionResponse(rows[1]),
	}, true, nil
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
		LogIndex:      tx.LogIndex,
		CreatedAt:     tx.CreatedAt,
	}
}
