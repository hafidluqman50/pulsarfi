package custodian

import (
	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
)

var (
	emailService   *external.EmailService
	storageService *external.StorageService
)

type Services struct {
	Email   *external.EmailService
	Storage *external.StorageService
}

func ConfigureServices(s Services) {
	emailService = s.Email
	storageService = s.Storage
}

func ensureService(c *gin.Context, svc any) bool {
	if svc == nil {
		response.InternalError(c, "service not configured")
		return false
	}
	return true
}
