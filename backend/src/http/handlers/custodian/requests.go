package custodian

import (
	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
)

// ListPendingRequestsHandler returns all pending mint and redeem proposals
// in a single response so the custodian UI can render the unified request queue.
func ListPendingRequestsHandler(c *gin.Context) {
	if !ensureService(c, custodianSvc) {
		return
	}

	result, err := custodianSvc.ListPendingRequests(c.Request.Context())
	if err != nil {
		response.InternalError(c, "failed to fetch requests")
		return
	}
	response.OK(c, "requests retrieved", result)
}
