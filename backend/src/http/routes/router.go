package routes

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/auth"
	authhandler "github.com/horizonlabs/pulsarfi-backend/src/http/handlers/auth"
	"github.com/horizonlabs/pulsarfi-backend/src/http/middleware"
	custodianRoutes "github.com/horizonlabs/pulsarfi-backend/src/http/routes/custodian"
	publicRoutes "github.com/horizonlabs/pulsarfi-backend/src/http/routes/public"
	"gorm.io/gorm"
)

func SetupRouter(db *gorm.DB, jwtConfig auth.Config) *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(middleware.Logger())
	router.Use(middleware.HTTPSRedirect())
	router.Use(middleware.MaxBodySize(10 << 20))
	router.Use(middleware.RateLimit(100, time.Minute))
	router.Use(middleware.CORS())

	router.GET("/healthz", func(c *gin.Context) {
		sqlDB, err := db.DB()
		if err != nil || sqlDB.PingContext(c.Request.Context()) != nil {
			c.JSON(503, gin.H{"status_code": 503, "message": "unhealthy", "data": nil})
			return
		}
		c.JSON(200, gin.H{"status_code": 200, "message": "ok", "data": gin.H{"network": "arbitrum-sepolia"}})
	})

	api := router.Group("/api/v1")
	api.GET("/auth/nonce", authhandler.NonceHandler)
	api.POST("/auth/verify", authhandler.VerifyHandler)

	custodianRoutes.RegisterRoutes(api.Group("/custodian"), jwtConfig)
	publicRoutes.RegisterRoutes(api.Group("/public"))

	return router
}
