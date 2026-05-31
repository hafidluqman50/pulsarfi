package public

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
	publicsvc "github.com/horizonlabs/pulsarfi-backend/src/service/public"
)

type recordRedeemBody struct {
	OnChainID   int64  `json:"on_chain_id"  binding:"required"`
	Ticker      string `json:"ticker"       binding:"required"`
	TokenAmount string `json:"token_amount" binding:"required"`
	FeeIdrx     string `json:"fee_idrx"`
	UserAddress string `json:"user_address" binding:"required"`
	TxHash      string `json:"tx_hash"      binding:"required"`
}

func RecordRedeemHandler(c *gin.Context) {
	if !ensureService(c, publicRedeemSvc) {
		return
	}

	var req recordRedeemBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	proposal, created, err := publicRedeemSvc.Record(c.Request.Context(), publicsvc.RecordRedeemRequest{
		OnChainID:   req.OnChainID,
		Ticker:      req.Ticker,
		TokenAmount: req.TokenAmount,
		FeeIdrx:     req.FeeIdrx,
		UserAddress: req.UserAddress,
		TxHash:      req.TxHash,
	})
	if errors.Is(err, publicsvc.ErrStockNotFound) {
		response.NotFound(c, "stock not found")
		return
	}
	if err != nil {
		response.InternalError(c, "failed to record redeem request")
		return
	}

	if !created {
		response.OK(c, "redeem request already recorded", proposal)
		return
	}

	response.Created(c, "redeem request recorded", proposal)
}

func ListUserRedeemsHandler(c *gin.Context) {
	if !ensureService(c, publicRedeemSvc) {
		return
	}

	walletAddress := c.Query("wallet_address")
	if walletAddress == "" {
		response.BadRequest(c, "wallet_address is required")
		return
	}

	proposals, err := publicRedeemSvc.ListByUser(c.Request.Context(), walletAddress)
	if err != nil {
		response.InternalError(c, "failed to fetch redeem requests")
		return
	}

	response.OK(c, "redeem requests retrieved", proposals)
}
