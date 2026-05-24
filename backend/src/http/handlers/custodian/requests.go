package custodian

import (
	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
)

// ListPendingRequestsHandler returns all unexecuted mint and redeem proposals
// in a single response so the custodian UI can render the unified request queue.
func ListPendingRequestsHandler(c *gin.Context) {
	mints, err := repos.MintProposal.FindPending(c.Request.Context())
	if err != nil {
		response.InternalError(c, "failed to fetch mint proposals")
		return
	}

	redeems, err := repos.RedeemProposal.FindPending(c.Request.Context())
	if err != nil {
		response.InternalError(c, "failed to fetch redeem proposals")
		return
	}

	response.OK(c, "requests retrieved", gin.H{
		"mints":   mints,
		"redeems": redeems,
	})
}
