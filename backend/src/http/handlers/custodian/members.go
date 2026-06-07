package custodian

import (
	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
)

func ListMembersHandler(c *gin.Context) {
	ctx := c.Request.Context()

	custodians, err := custodianSvc.ListMembers(ctx)
	if err != nil {
		response.InternalError(c, "Failed to fetch custodians")
		return
	}

	response.OK(c, "OK", custodians)
}
