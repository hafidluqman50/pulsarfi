package agent

import (
	"strings"

	"github.com/gin-gonic/gin"
	usermw "github.com/horizonlabs/pulsarfi-backend/src/http/middleware/user"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
	publicsvc "github.com/horizonlabs/pulsarfi-backend/src/service/public"
)

func GetPortfolioChartHandler(c *gin.Context) {
	if chartSvc == nil {
		response.InternalError(c, "portfolio chart service not configured")
		return
	}
	claims, ok := usermw.Get(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}

	lens := publicsvc.ChartLens(c.Query("lens"))
	if !publicsvc.PortfolioLenses[lens] {
		response.BadRequest(c, "invalid lens, price_line is a stock chart, not a portfolio chart, use /public/prices/:ticker/history instead")
		return
	}

	data, err := chartSvc.Fetch(c.Request.Context(), lens, strings.ToLower(claims.WalletAddress), c.DefaultQuery("range", "1M"))
	if err != nil {
		response.InternalError(c, "failed to fetch chart data")
		return
	}

	response.OK(c, "chart data retrieved", data)
}
