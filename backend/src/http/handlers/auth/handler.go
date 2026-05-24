package authhandler

import (
	"github.com/gin-gonic/gin"
	authsvc "github.com/horizonlabs/pulsarfi-backend/src/service/auth"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
)

var svc *authsvc.AuthService

func Configure(s *authsvc.AuthService) {
	svc = s
}

func NonceHandler(c *gin.Context) {
	address := c.Query("address")
	if address == "" {
		response.BadRequest(c, "address query param is required")
		return
	}
	nonce := svc.Nonce(address)
	response.OK(c, "nonce issued", gin.H{"nonce": nonce})
}

type verifyRequest struct {
	Address   string `json:"address"   binding:"required"`
	Message   string `json:"message"   binding:"required"`
	Signature string `json:"signature" binding:"required"`
	Nonce     string `json:"nonce"     binding:"required"`
}

func VerifyHandler(c *gin.Context) {
	var req verifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	token, err := svc.Verify(c.Request.Context(), authsvc.VerifyInput{
		Address:   req.Address,
		Message:   req.Message,
		Signature: req.Signature,
		Nonce:     req.Nonce,
	})
	if err != nil {
		response.Unauthorized(c, err.Error())
		return
	}

	response.OK(c, "authenticated", gin.H{"access_token": token})
}
