package public

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
	publicsvc "github.com/horizonlabs/pulsarfi-backend/src/service/public"
)

type recordSwapRequest struct {
	Ticker        string `json:"ticker"         binding:"required"`
	TxHash        string `json:"tx_hash"        binding:"required"`
	WalletAddress string `json:"wallet_address" binding:"required"`
	Side          string `json:"side"           binding:"required,oneof=buy sell"`
	IdrxAmount    string `json:"idrx_amount"    binding:"required"`
	StockAmount   string `json:"stock_amount"   binding:"required"`
	BlockNumber   int64  `json:"block_number"   binding:"required"`
}

func ListStockTransactionsHandler(c *gin.Context) {
	if !ensureService(c, publicStockTransactionSvc) {
		return
	}

	transactions, err := publicStockTransactionSvc.ListByWallet(c.Request.Context(), c.Query("wallet_address"))
	if errors.Is(err, publicsvc.ErrWalletAddressRequired) {
		response.BadRequest(c, "wallet_address is required")
		return
	}
	if err != nil {
		response.InternalError(c, "failed to fetch transactions")
		return
	}

	response.OK(c, "transactions retrieved", transactions)
}

func RecordSwapHandler(c *gin.Context) {
	if !ensureService(c, publicStockTransactionSvc) {
		return
	}

	var req recordSwapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	tx, created, err := publicStockTransactionSvc.Record(c.Request.Context(), publicsvc.RecordStockTransactionRequest{
		Ticker:        req.Ticker,
		TxHash:        req.TxHash,
		WalletAddress: req.WalletAddress,
		Side:          req.Side,
		IdrxAmount:    req.IdrxAmount,
		StockAmount:   req.StockAmount,
		BlockNumber:   req.BlockNumber,
	})
	if errors.Is(err, publicsvc.ErrStockNotFound) {
		response.NotFound(c, "stock not found")
		return
	}
	if errors.Is(err, publicsvc.ErrInvalidTransactionSide) {
		response.BadRequest(c, "invalid transaction side")
		return
	}
	if err != nil {
		response.InternalError(c, "failed to record transaction")
		return
	}

	if !created {
		response.OK(c, "transaction already recorded", tx)
		return
	}

	response.Created(c, "transaction recorded", tx)
}
