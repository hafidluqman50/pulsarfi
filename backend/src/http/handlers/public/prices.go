package public

import (
	"errors"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
	publicsvc "github.com/horizonlabs/pulsarfi-backend/src/service/public"
)

func GetStockPriceHandler(c *gin.Context) {
	if !ensureService(c, publicPriceSvc) {
		return
	}

	ticker := strings.ToUpper(c.Param("ticker"))
	if ticker == "" {
		response.BadRequest(c, "ticker is required")
		return
	}

	entry, err := publicPriceSvc.GetStockPrice(c.Request.Context(), ticker)
	if errors.Is(err, publicsvc.ErrStockNotFound) {
		response.NotFound(c, "stock not found")
		return
	}
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c, "price retrieved", entry)
}
