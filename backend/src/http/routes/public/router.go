package public

import (
	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/auth"
	publicHandler "github.com/horizonlabs/pulsarfi-backend/src/http/handlers/public"
	usermw "github.com/horizonlabs/pulsarfi-backend/src/http/middleware/user"
)

func RegisterRoutes(rg *gin.RouterGroup, jwtConfig auth.Config) {
	rg.GET("/stocks", publicHandler.ListStocksHandler)
	rg.GET("/stock-transactions", publicHandler.ListStockTransactionsHandler)
	// Requires a real, identified wallet (any SIWE-issued token) — the
	// handler binds each record to the token's wallet, not a client-claimed one.
	rg.POST("/stock-transactions", usermw.Auth(jwtConfig), publicHandler.RecordSwapHandler)
	rg.POST("/stock-transactions/transfers", usermw.Auth(jwtConfig), publicHandler.RecordTransferHandler)
	rg.POST("/wallet-verifications", usermw.Auth(jwtConfig), publicHandler.SubmitWalletVerificationHandler)
	rg.GET("/reserves", publicHandler.GetReservesHandler)
	rg.GET("/prices/:ticker", publicHandler.GetStockPriceHandler)
	rg.GET("/prices/:ticker/history", publicHandler.GetStockHistoryHandler)
	rg.GET("/stats", publicHandler.GetStatsHandler)
	rg.POST("/redeem-requests", usermw.Auth(jwtConfig), publicHandler.RecordRedeemHandler)
	rg.GET("/redeem-requests", publicHandler.ListUserRedeemsHandler)
}
