package stocktransactionrequest

import "github.com/gin-gonic/gin"

type SwapRequest struct {
	Ticker        string `json:"ticker"         binding:"required"`
	TxHash        string `json:"tx_hash"        binding:"required"`
	WalletAddress string `json:"wallet_address" binding:"required"`
	Side          string `json:"side"           binding:"required,oneof=buy sell"`
	IdrxAmount    string `json:"idrx_amount"    binding:"required"`
	StockAmount   string `json:"stock_amount"   binding:"required"`
	BlockNumber   int64  `json:"block_number"   binding:"required"`
	LogIndex      int    `json:"log_index"`
}

func NewSwapRequest(c *gin.Context) (SwapRequest, error) {
	var req SwapRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return SwapRequest{}, err
	}
	return req, nil
}
