package public

import (
	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
)

func GetReservesHandler(c *gin.Context) {
	if !ensureService(c, publicReservesSvc) {
		return
	}

	entries, err := publicReservesSvc.GetReserves(c.Request.Context())
	if err != nil {
		response.InternalError(c, "failed to fetch reserves")
		return
	}

	response.OK(c, "reserves retrieved", entries)
}
