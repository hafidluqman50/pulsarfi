package custodian

import (
	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
)

func GetStatsHandler(c *gin.Context) {
	ctx := c.Request.Context()

	assetsUnderCustodyIdr, _ := repos.StockAttestation.SumLatestHoldings(ctx)

	mintCount24h, _ := repos.MintProposal.CountExecutedLast24h(ctx)
	mintVolume24hIdrx, _ := repos.MintProposal.SumIdrxLast24h(ctx)
	burnCount24h, _ := repos.RedeemProposal.CountExecutedLast24h(ctx)

	pendingMints, _ := repos.MintProposal.FindPending(ctx)
	pendingRedeems, _ := repos.RedeemProposal.FindPending(ctx)

	response.OK(c, "stats retrieved", gin.H{
		"assets_under_custody_idr": assetsUnderCustodyIdr,
		"mint_volume_24h_idrx":     mintVolume24hIdrx,
		"mint_count_24h":           mintCount24h,
		"burn_count_24h":           burnCount24h,
		"pending_requests": gin.H{
			"total":   len(pendingMints) + len(pendingRedeems),
			"mints":   len(pendingMints),
			"redeems": len(pendingRedeems),
		},
	})
}
