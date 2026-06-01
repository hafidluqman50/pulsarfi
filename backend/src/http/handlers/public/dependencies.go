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
	publicStockSvc            *publicsvc.StockService
	publicPriceSvc            *publicsvc.PriceService
	publicReserveSvc          *publicsvc.ReserveService
	publicStockTransactionSvc *publicsvc.StockTransactionService
	publicStatsSvc            *publicsvc.StatsService
	publicRedeemSvc           *publicsvc.PublicRedeemService
)

func ConfigureRepos(r *repository.Registry) {
	repos = r
}

func ConfigureServices(s *service.Registry) {
	publicStockSvc = s.PublicStock
	publicPriceSvc = s.PublicPrice
	publicReserveSvc = s.PublicReserve
	publicStockTransactionSvc = s.PublicStockTransaction
	publicStatsSvc = s.PublicStats
	publicRedeemSvc = s.PublicRedeem
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
