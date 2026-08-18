package public

import (
	"github.com/gin-gonic/gin"
	usermw "github.com/horizonlabs/pulsarfi-backend/src/http/middleware/user"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
)

type submitVerificationBody struct {
	Type        string  `json:"type" binding:"required,oneof=retail institution"`
	DocumentRef *string `json:"document_ref"`
}

func SubmitWalletVerificationHandler(c *gin.Context) {
	claims, ok := usermw.Get(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}

	var req submitVerificationBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	// wallet_address always comes from the verified token, never the body —
	// otherwise anyone with a valid token could submit on someone else's behalf.
	walletAddress := claims.WalletAddress

	_, exists, err := repos.WalletVerification.FindByWallet(c.Request.Context(), walletAddress)
	if err != nil {
		response.InternalError(c, "failed to check existing verification")
		return
	}
	if exists {
		response.OK(c, "verification already submitted", nil)
		return
	}

	record, err := repos.WalletVerification.Create(c.Request.Context(), repository.WalletVerificationCreateInput{
		WalletAddress: walletAddress,
		Type:          req.Type,
		DocumentRef:   req.DocumentRef,
	})
	if err != nil {
		response.InternalError(c, "failed to submit verification")
		return
	}

	response.Created(c, "verification submitted", record)
}
