package public

import (
	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
)

func ListStocksHandler(c *gin.Context) {
	if !ensureService(c, publicStocksSvc) {
		return
	}
	stocks, err := publicStocksSvc.ListMarketStocks(c.Request.Context())
	if err != nil {
		response.InternalError(c, "failed to fetch stocks")
		return
	}
	response.OK(c, "stocks retrieved", stocks)
}
