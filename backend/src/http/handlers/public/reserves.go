package public

import (
	"fmt"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
)

type reserveEntry struct {
	Stock             any     `json:"stock"`
	CustodianHoldings string  `json:"custodian_holdings"`
	OnChainSupply     string  `json:"on_chain_supply"`
	PegRatio          string  `json:"peg_ratio"`
	PegStatus         string  `json:"peg_status"` // pegged | depegged
	LastAttestedAt    any     `json:"last_attested_at"`
	AttestationHash   *string `json:"attestation_hash"`
}

func GetReservesHandler(c *gin.Context) {
	attestations, err := repos.StockAttestation.FindLatestPerStock(c.Request.Context())
	if err != nil {
		response.InternalError(c, "failed to fetch reserves")
		return
	}

	entries := make([]reserveEntry, 0, len(attestations))
	for _, a := range attestations {
		holdings, errH := strconv.ParseFloat(a.CustodianHoldings, 64)
		supply, errS := strconv.ParseFloat(a.OnChainSupply, 64)

		pegRatio := "N/A"
		pegStatus := "unknown"
		if errH == nil && errS == nil && supply > 0 {
			ratio := holdings / supply
			pegRatio = fmt.Sprintf("%.4f", ratio)
			if ratio >= 1.0 {
				pegStatus = "pegged"
			} else {
				pegStatus = "depegged"
			}
		}

		entries = append(entries, reserveEntry{
			Stock:             a.Stock,
			CustodianHoldings: a.CustodianHoldings,
			OnChainSupply:     a.OnChainSupply,
			PegRatio:          pegRatio,
			PegStatus:         pegStatus,
			LastAttestedAt:    a.AttestedAt,
			AttestationHash:   a.AttestationHash,
		})
	}

	response.OK(c, "reserves retrieved", entries)
}
