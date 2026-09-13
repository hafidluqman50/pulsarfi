package realtime

import (
	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/auth"
	realtimeHandler "github.com/horizonlabs/pulsarfi-backend/src/http/handlers/realtime"
)

func RegisterRoutes(rg *gin.RouterGroup, jwtConfig auth.Config) {
	rg.GET("/ws", realtimeHandler.StreamHandler(jwtConfig))
}
