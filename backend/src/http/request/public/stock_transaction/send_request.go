package stocktransactionrequest

import "github.com/gin-gonic/gin"

type SendRequest struct {
	Ticker      string `json:"ticker"       binding:"required"`
	TxHash      string `json:"tx_hash"      binding:"required"`
	FromAddress string `json:"from_address" binding:"required"`
	ToAddress   string `json:"to_address"   binding:"required"`
	IdrxAmount  string `json:"idrx_amount"  binding:"required"`
	StockAmount string `json:"stock_amount" binding:"required"`
	BlockNumber int64  `json:"block_number" binding:"required"`
	LogIndex    int    `json:"log_index"`
}

func NewSendRequest(c *gin.Context) (SendRequest, error) {
	var req SendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return SendRequest{}, err
	}
	return req, nil
}
