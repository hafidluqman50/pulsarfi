package public

import (
	"errors"
	"log/slog"

	"github.com/gin-gonic/gin"
	usermw "github.com/horizonlabs/pulsarfi-backend/src/http/middleware/user"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
	"github.com/horizonlabs/pulsarfi-backend/src/logger"
	publicsvc "github.com/horizonlabs/pulsarfi-backend/src/service/public"
)

type recordRedeemBody struct {
	OnChainID       *int64 `json:"on_chain_id"       binding:"required"`
	Ticker          string `json:"ticker"            binding:"required"`
	TokenAmount     string `json:"token_amount"      binding:"required"`
	FeeIdrx         string `json:"fee_idrx"`
	UserAddress     string `json:"user_address"      binding:"required"`
	AttestationHash string `json:"attestation_hash"  binding:"required"`
	TxHash          string `json:"tx_hash"           binding:"required"`
	BlockNumber     int64  `json:"block_number"`
}

func RecordRedeemHandler(c *gin.Context) {
	if !ensureService(c, publicRedeemSvc) {
		return
	}

	claims, ok := usermw.Get(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}

	var req recordRedeemBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	proposal, created, err := publicRedeemSvc.Record(c.Request.Context(), publicsvc.RecordRedeemRequest{
		OnChainID:           *req.OnChainID,
		Ticker:              req.Ticker,
		TokenAmount:         req.TokenAmount,
		FeeIdrx:             req.FeeIdrx,
		UserAddress:         req.UserAddress,
		AttestationHash:     req.AttestationHash,
		TxHash:              req.TxHash,
		BlockNumber:         req.BlockNumber,
		AuthenticatedWallet: claims.WalletAddress,
	})
	if errors.Is(err, publicsvc.ErrStockNotFound) {
		response.NotFound(c, "stock not found")
		return
	}
	if errors.Is(err, publicsvc.ErrWalletMismatch) {
		response.Forbidden(c, "authenticated wallet does not match user_address")
		return
	}
	if errors.Is(err, publicsvc.ErrOnChainMismatch) {
		response.UnprocessableEntity(c, "submitted redeem data does not match the on-chain transaction", nil)
		return
	}
	if errors.Is(err, publicsvc.ErrVerifierUnavailable) {
		response.InternalError(c, "on-chain verification is unavailable, try again later")
		return
	}
	if err != nil {
		logger.L.ErrorContext(c.Request.Context(), "redeem: record failed",
			slog.String("ticker", req.Ticker),
			slog.String("user_address", req.UserAddress),
			slog.Int64("on_chain_id", *req.OnChainID),
			slog.String("tx_hash", req.TxHash),
			slog.String("error", err.Error()),
		)
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
