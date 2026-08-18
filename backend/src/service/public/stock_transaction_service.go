package public

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
)

var ErrInvalidTransactionSide = errors.New("invalid transaction side")
var ErrWalletAddressRequired = errors.New("wallet address is required")
var ErrTransferAddressRequired = errors.New("transfer from and to addresses are required")
var ErrTransferSameAddress = errors.New("transfer from and to addresses must differ")
var ErrTransferRecordIncomplete = errors.New("transfer transaction record is incomplete")
var ErrOnChainMismatch = errors.New("submitted data does not match the on-chain transaction")

// SwapVerifier re-derives swap/transfer facts from real on-chain events via a
// single tx_hash lookup — no polling, no indexing — so a valid-but-dishonest
// caller can't submit fabricated amounts under their own wallet.
type SwapVerifier interface {
	VerifyV4Swapped(ctx context.Context, txHash, ticker string) (*external.V4SwappedEvent, error)
	VerifyTransfer(ctx context.Context, txHash string, tokenAddress common.Address, logIndex int) (*external.TransferEvent, error)
}

type StockTransactionService struct {
	Stocks       *repository.StockRepository
	Transactions *repository.StockTransactionRepository
	Verifier     SwapVerifier
}

type RecordStockTransactionRequest struct {
	Ticker              string
	TxHash              string
	WalletAddress       string
	Side                string
	IdrxAmount          string
	StockAmount         string
	ProtocolFeeIdrx     string
	BlockNumber         int64
	LogIndex            int
	AuthenticatedWallet string
}

type RecordTransferRequest struct {
	Ticker              string
	TxHash              string
	FromAddress         string
	ToAddress           string
	IdrxAmount          string
	StockAmount         string
	BlockNumber         int64
	LogIndex            int
	AuthenticatedWallet string
}

type TransferTransactionResponse struct {
	Out StockTransactionResponse `json:"out"`
	In  StockTransactionResponse `json:"in"`
}

type StockTransactionResponse struct {
	ID              int64     `json:"id"`
	StockID         int64     `json:"stock_id"`
	Ticker          string    `json:"ticker"`
	StockName       string    `json:"stock_name"`
	IdxTicker       string    `json:"idx_ticker"`
	WalletAddress   string    `json:"wallet_address"`
	Side            string    `json:"side"`
	IdrxAmount      string    `json:"idrx_amount"`
	StockAmount     string    `json:"stock_amount"`
	ProtocolFeeIdrx string    `json:"protocol_fee_idrx"`
	TxHash          string    `json:"tx_hash"`
	BlockNumber     int64     `json:"block_number"`
	LogIndex        int       `json:"log_index"`
	CreatedAt       time.Time `json:"created_at"`
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

	// 1. The token proves who is calling — blocks submitting a swap record
	// under someone else's wallet.
	if !strings.EqualFold(req.AuthenticatedWallet, req.WalletAddress) {
		return model.StockTransaction{}, false, ErrWalletMismatch
	}

	// 2. tx_hash proves what actually happened — blocks a valid caller from
	// fabricating amounts/side under their own wallet. One RPC call, no
	// indexing. protocol_fee_idrx isn't re-derived — not a field the DB
	// treats as authoritative, so no chain lookup needed for it.
	if s.Verifier == nil {
		return model.StockTransaction{}, false, ErrVerifierUnavailable
	}
	event, err := s.Verifier.VerifyV4Swapped(ctx, req.TxHash, stock.Ticker)
	if err != nil {
		return model.StockTransaction{}, false, ErrOnChainMismatch
	}
	if event.BuyStock != (side == "buy") {
		return model.StockTransaction{}, false, ErrOnChainMismatch
	}
	if !strings.EqualFold(event.User.Hex(), req.WalletAddress) {
		return model.StockTransaction{}, false, ErrOnChainMismatch
	}

	var idrxAmount, stockAmount string
	if event.BuyStock {
		idrxAmount, stockAmount = event.AmountIn.String(), event.AmountOut.String()
	} else {
		idrxAmount, stockAmount = event.AmountOut.String(), event.AmountIn.String()
	}

	protocolFeeIdrx := req.ProtocolFeeIdrx
	if protocolFeeIdrx == "" {
		protocolFeeIdrx = "0"
	}

	tx, err := s.Transactions.Create(ctx, repository.StockTransactionCreateInput{
		StockID:         stock.ID,
		WalletAddress:   strings.ToLower(req.WalletAddress),
		Side:            side,
		IdrxAmount:      idrxAmount,
		StockAmount:     stockAmount,
		ProtocolFeeIdrx: protocolFeeIdrx,
		TxHash:          req.TxHash,
		BlockNumber:     req.BlockNumber,
		LogIndex:        req.LogIndex,
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

	// 1. The token proves who is calling — a transfer report can only be
	// filed by the party who actually sent it. (The indexer, a trusted
	// internal caller, passes fromAddress itself.)
	if !strings.EqualFold(req.AuthenticatedWallet, fromAddress) {
		return TransferTransactionResponse{}, false, ErrWalletMismatch
	}

	// 2. tx_hash proves what actually happened — blocks fabricating
	// stock_amount under a real tx_hash. idrx_amount has no on-chain fact
	// to check (a plain transfer moves no IDRX), stays client-estimated.
	if stock.ContractAddress == nil || strings.TrimSpace(*stock.ContractAddress) == "" {
		return TransferTransactionResponse{}, false, ErrStockNotFound
	}
	if s.Verifier == nil {
		return TransferTransactionResponse{}, false, ErrVerifierUnavailable
	}
	event, err := s.Verifier.VerifyTransfer(ctx, req.TxHash, common.HexToAddress(*stock.ContractAddress), req.LogIndex)
	if err != nil {
		return TransferTransactionResponse{}, false, ErrOnChainMismatch
	}
	if !strings.EqualFold(event.From.Hex(), fromAddress) || !strings.EqualFold(event.To.Hex(), toAddress) {
		return TransferTransactionResponse{}, false, ErrOnChainMismatch
	}

	stockAmount := event.Amount.String()

	rows, err := s.Transactions.CreateMany(ctx, []repository.StockTransactionCreateInput{
		{
			StockID:       stock.ID,
			WalletAddress: fromAddress,
			Side:          "transfer-out",
			IdrxAmount:    req.IdrxAmount,
			StockAmount:   stockAmount,
			TxHash:        req.TxHash,
			BlockNumber:   req.BlockNumber,
			LogIndex:      req.LogIndex,
		},
		{
			StockID:       stock.ID,
			WalletAddress: toAddress,
			Side:          "transfer-in",
			IdrxAmount:    req.IdrxAmount,
			StockAmount:   stockAmount,
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
		ID:              tx.ID,
		StockID:         tx.StockID,
		Ticker:          tx.Stock.Ticker,
		StockName:       tx.Stock.StockName,
		IdxTicker:       tx.Stock.IdxTicker,
		WalletAddress:   tx.WalletAddress,
		Side:            tx.Side,
		IdrxAmount:      tx.IdrxAmount,
		StockAmount:     tx.StockAmount,
		ProtocolFeeIdrx: tx.ProtocolFeeIdrx,
		TxHash:          tx.TxHash,
		BlockNumber:     tx.BlockNumber,
		LogIndex:        tx.LogIndex,
		CreatedAt:       tx.CreatedAt,
	}
}
