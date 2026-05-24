package custodian

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	custodianMiddleware "github.com/horizonlabs/pulsarfi-backend/src/http/middleware/custodian"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
)

type submitAttestationBody struct {
	StockID           int64   `json:"stock_id"            binding:"required"`
	CustodianHoldings string  `json:"custodian_holdings"  binding:"required"`
	OnChainSupply     string  `json:"on_chain_supply"     binding:"required"`
	AttestationHash   *string `json:"attestation_hash"`
}

func SubmitAttestationHandler(c *gin.Context) {
	claims, ok := custodianMiddleware.Get(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	var req submitAttestationBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	custodian, found, err := repos.Custodian.FindByWalletAddress(c.Request.Context(), claims.WalletAddress)
	if err != nil || !found {
		response.InternalError(c, "failed to lookup custodian")
		return
	}

	record, err := repos.StockAttestation.Create(c.Request.Context(), repository.StockAttestationCreateInput{
		StockID:           req.StockID,
		CustodianID:       custodian.ID,
		CustodianHoldings: req.CustodianHoldings,
		OnChainSupply:     req.OnChainSupply,
		AttestationHash:   req.AttestationHash,
	})
	if err != nil {
		response.InternalError(c, "failed to submit attestation")
		return
	}

	if streamService != nil {
		streamService.Emit(external.LevelOK, "[attest]",
			fmt.Sprintf("proof-of-reserves submitted · stock=%d · holdings=%s · supply=%s · attester=%s",
				req.StockID, req.CustodianHoldings, req.OnChainSupply, custodian.WalletAddress[:10]+"..."))
	}

	response.Created(c, "attestation submitted", record)
}

func ListAttestationsHandler(c *gin.Context) {
	stockIDParam := c.Query("stock_id")
	if stockIDParam != "" {
		stockID, err := strconv.ParseInt(stockIDParam, 10, 64)
		if err != nil {
			response.BadRequest(c, "invalid stock_id")
			return
		}
		records, err := repos.StockAttestation.FindByStockID(c.Request.Context(), stockID)
		if err != nil {
			response.InternalError(c, "failed to fetch attestations")
			return
		}
		response.OK(c, "attestations retrieved", records)
		return
	}

	records, err := repos.StockAttestation.FindLatestPerStock(c.Request.Context())
	if err != nil {
		response.InternalError(c, "failed to fetch attestations")
		return
	}
	response.OK(c, "attestations retrieved", records)
}
