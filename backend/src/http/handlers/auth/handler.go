package authhandler

import (
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
	"github.com/horizonlabs/pulsarfi-backend/src/logger"
	authsvc "github.com/horizonlabs/pulsarfi-backend/src/service/auth"
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
		logger.L.WarnContext(c.Request.Context(), "verify: bind failed",
			slog.String("error", err.Error()),
			slog.String("remote_addr", c.ClientIP()),
		)
		response.BadRequest(c, err.Error())
		return
	}

	logger.L.InfoContext(c.Request.Context(), "verify: attempt",
		slog.String("address", req.Address),
		slog.String("nonce", req.Nonce),
		slog.String("remote_addr", c.ClientIP()),
	)

	token, err := svc.Verify(c.Request.Context(), authsvc.VerifyInput{
		Address:   req.Address,
		Message:   req.Message,
		Signature: req.Signature,
		Nonce:     req.Nonce,
	})
	if err != nil {
		logger.L.WarnContext(c.Request.Context(), "verify: failed",
			slog.String("address", req.Address),
			slog.String("nonce", req.Nonce),
			slog.String("error", err.Error()),
			slog.String("remote_addr", c.ClientIP()),
		)
		response.Unauthorized(c, err.Error())
		return
	}

	logger.L.InfoContext(c.Request.Context(), "verify: ok",
		slog.String("address", req.Address),
	)
	response.OK(c, "authenticated", gin.H{"access_token": token})
}
