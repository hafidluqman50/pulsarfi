package custodian

import (
	"errors"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	custodianMiddleware "github.com/horizonlabs/pulsarfi-backend/src/http/middleware/custodian"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
	"github.com/horizonlabs/pulsarfi-backend/src/logger"
	custodiansvc "github.com/horizonlabs/pulsarfi-backend/src/service/custodian"
)

func ListWalletVerificationsHandler(c *gin.Context) {
	if !ensureService(c, custodianKYCSvc) {
		return
	}
	status := c.Query("status") // pending | approved | rejected | ""
	records, err := custodianKYCSvc.List(c.Request.Context(), status)
	if err != nil {
		response.InternalError(c, "failed to fetch verifications")
		return
	}
	response.OK(c, "verifications retrieved", records)
}

func CreateWalletVerificationHandler(c *gin.Context) {
	if !ensureService(c, custodianKYCSvc) {
		return
	}

	claims, ok := custodianMiddleware.Get(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	file, header, err := c.Request.FormFile("document")
	if err != nil {
		logger.L.WarnContext(c.Request.Context(), "kyc: missing document",
			slog.String("custodian", claims.WalletAddress),
			slog.String("error", err.Error()),
		)
		response.BadRequest(c, "signed statement document is required")
		return
	}
	defer file.Close()

	walletAddress := c.PostForm("wallet_address")
	kycType := c.PostForm("type")

	logger.L.InfoContext(c.Request.Context(), "kyc: create attempt",
		slog.String("custodian", claims.WalletAddress),
		slog.String("wallet_address", walletAddress),
		slog.String("type", kycType),
		slog.String("approval_tx_hash", c.PostForm("approval_tx_hash")),
		slog.String("filename", header.Filename),
		slog.Int64("file_size", header.Size),
	)

	record, err := custodianKYCSvc.CreateVerified(c.Request.Context(), custodiansvc.CreateVerifiedKYCRequest{
		WalletAddress:  walletAddress,
		Type:           kycType,
		FullName:       c.PostForm("full_name"),
		Email:          c.PostForm("email"),
		ApprovalTxHash: c.PostForm("approval_tx_hash"),
		Document:       file,
		DocumentHeader: header,
		CustodianAddr:  claims.WalletAddress,
	})
	if err != nil {
		logger.L.WarnContext(c.Request.Context(), "kyc: create failed",
			slog.String("custodian", claims.WalletAddress),
			slog.String("wallet_address", walletAddress),
			slog.String("error", err.Error()),
		)
		switch {
		case errors.Is(err, custodiansvc.ErrCustodianNotFound):
			response.Unauthorized(c, "custodian not found")
		case errors.Is(err, custodiansvc.ErrKYCStorageNotConfigured):
			response.InternalError(c, "kyc storage not configured")
		default:
			response.BadRequest(c, err.Error())
		}
		return
	}

	logger.L.InfoContext(c.Request.Context(), "kyc: created",
		slog.String("custodian", claims.WalletAddress),
		slog.String("wallet_address", walletAddress),
	)
	response.Created(c, "verification recorded", record)
}

func GetWalletVerificationDocumentURLHandler(c *gin.Context) {
	if !ensureService(c, custodianKYCSvc) {
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	result, err := custodianKYCSvc.DocumentURL(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, custodiansvc.ErrKYCRecordNotFound) {
			response.NotFound(c, "document not found")
			return
		}
		response.InternalError(c, "failed to generate document url")
		return
	}

	response.OK(c, "document url generated", result)
}
