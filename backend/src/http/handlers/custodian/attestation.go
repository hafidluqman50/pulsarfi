package custodian

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	custodianMiddleware "github.com/horizonlabs/pulsarfi-backend/src/http/middleware/custodian"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
	custodiansvc "github.com/horizonlabs/pulsarfi-backend/src/service/custodian"
)

func SubmitAttestationHandler(c *gin.Context) {
	if !ensureService(c, custodianSvc) {
		return
	}

	claims, ok := custodianMiddleware.Get(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	var req custodiansvc.SubmitAttestationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	record, err := custodianSvc.SubmitAttestation(
		c.Request.Context(),
		claims.WalletAddress,
		req,
	)
	if err != nil {
		if errors.Is(err, custodiansvc.ErrCustodianNotFound) {
			response.Unauthorized(c, "custodian not found")
			return
		}
		response.InternalError(c, "failed to submit attestation")
		return
	}

	response.Created(c, "attestation submitted", record)
}

func ListAttestationsHandler(c *gin.Context) {
	if !ensureService(c, custodianSvc) {
		return
	}

	stockIDParam := c.Query("stock_id")
	var stockID *int64
	if stockIDParam != "" {
		parsedStockID, err := strconv.ParseInt(stockIDParam, 10, 64)
		if err != nil {
			response.BadRequest(c, "invalid stock_id")
			return
		}
		stockID = &parsedStockID
	}

	records, err := custodianSvc.ListAttestations(c.Request.Context(), stockID)
	if err != nil {
		response.InternalError(c, "failed to fetch attestations")
		return
	}
	response.OK(c, "attestations retrieved", records)
}
