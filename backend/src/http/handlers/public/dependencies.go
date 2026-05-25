package public

import (
	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service"
	publicsvc "github.com/horizonlabs/pulsarfi-backend/src/service/public"
)

var repos *repository.Registry
var (
	publicStocksSvc   *publicsvc.StocksService
	publicPriceSvc    *publicsvc.PriceService
	publicReservesSvc *publicsvc.ReservesService
)

func ConfigureRepos(r *repository.Registry) {
	repos = r
}

func ConfigureServices(s *service.Registry) {
	publicStocksSvc = s.PublicStocks
	publicPriceSvc = s.PublicPrice
	publicReservesSvc = s.PublicReserves
}

func ensureRepos(c *gin.Context) bool {
	if repos == nil {
		response.InternalError(c, "repos not configured")
		return false
	}
	return true
}

func ensureService(c *gin.Context, svc any) bool {
	if svc == nil {
		response.InternalError(c, "service not configured")
		return false
	}
	return true
}
