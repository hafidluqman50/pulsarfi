package public

import (
	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
)

type submitVerificationBody struct {
	WalletAddress string  `json:"wallet_address" binding:"required"`
	Type          string  `json:"type"           binding:"required,oneof=retail institution"`
	DocumentURL   *string `json:"document_url"`
}

func SubmitWalletVerificationHandler(c *gin.Context) {
	var req submitVerificationBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	_, exists, err := repos.WalletVerification.FindByWallet(c.Request.Context(), req.WalletAddress)
	if err != nil {
		response.InternalError(c, "failed to check existing verification")
		return
	}
	if exists {
		response.OK(c, "verification already submitted", nil)
		return
	}

	record, err := repos.WalletVerification.Create(c.Request.Context(), repository.WalletVerificationCreateInput{
		WalletAddress: req.WalletAddress,
		Type:          req.Type,
		DocumentURL:   req.DocumentURL,
	})
	if err != nil {
		response.InternalError(c, "failed to submit verification")
		return
	}

	response.Created(c, "verification submitted", record)
}
