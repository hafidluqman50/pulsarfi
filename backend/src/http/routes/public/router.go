package public

import (
	"github.com/gin-gonic/gin"
	publicHandler "github.com/horizonlabs/pulsarfi-backend/src/http/handlers/public"
)

func RegisterRoutes(rg *gin.RouterGroup) {
	rg.GET("/stocks", publicHandler.ListStocksHandler)
	rg.POST("/stock-transactions", publicHandler.RecordSwapHandler)
	rg.POST("/wallet-verifications", publicHandler.SubmitWalletVerificationHandler)
	rg.GET("/reserves", publicHandler.GetReservesHandler)
	rg.GET("/prices/:ticker", publicHandler.GetStockPriceHandler)
}
