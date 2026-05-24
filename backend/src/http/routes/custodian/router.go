package custodian

import (
	"github.com/gin-gonic/gin"
	custodianMiddleware "github.com/horizonlabs/pulsarfi-backend/src/http/middleware/custodian"
	"github.com/horizonlabs/pulsarfi-backend/src/auth"
)

func RegisterRoutes(rg *gin.RouterGroup, jwtConfig auth.Config) {
	protected := rg.Group("", custodianMiddleware.Auth(jwtConfig))
	_ = protected
	// TODO: mint proposal, approval, KYC management routes
}
