package public

import (
	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
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

func RecordSwapHandler(c *gin.Context) {
	if !ensureRepos(c) {
		return
	}

	var req recordSwapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	stock, found, err := repos.Stock.FindByTicker(c.Request.Context(), req.Ticker)
	if err != nil {
		response.InternalError(c, "failed to lookup stock")
		return
	}
	if !found {
		response.NotFound(c, "stock not found")
		return
	}

	exists, err := repos.StockTransaction.ExistsByTxHash(c.Request.Context(), req.TxHash)
	if err != nil {
		response.InternalError(c, "failed to check transaction")
		return
	}
	if exists {
		response.OK(c, "transaction already recorded", nil)
		return
	}

	tx, err := repos.StockTransaction.Create(c.Request.Context(), repository.StockTransactionCreateInput{
		StockID:        stock.ID,
		WalletAddress:  req.WalletAddress,
		Side:           req.Side,
		IdrxAmountRaw:  req.IdrxAmount,
		StockAmountRaw: req.StockAmount,
		TxHash:         req.TxHash,
		BlockNumber:    req.BlockNumber,
	})
	if err != nil {
		response.InternalError(c, "failed to record transaction")
		return
	}

	response.Created(c, "transaction recorded", tx)
}
