package public

import (
	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
)

func GetStatsHandler(c *gin.Context) {
	if !ensureService(c, publicStatsSvc) {
		return
	}
	stats, err := publicStatsSvc.GetStats(c.Request.Context())
	if err != nil {
		response.InternalError(c, "failed to fetch stats")
		return
	}
	response.OK(c, "stats retrieved", stats)
}
