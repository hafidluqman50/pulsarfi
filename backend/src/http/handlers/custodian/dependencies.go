package custodian

import (
	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
)

var (
	repos          *repository.Registry
	emailService   *external.EmailService
	storageService *external.StorageService
	streamService  *external.StreamService
)

type Services struct {
	Repos   *repository.Registry
	Email   *external.EmailService
	Storage *external.StorageService
	Stream  *external.StreamService
}

func ConfigureServices(s Services) {
	repos = s.Repos
	emailService = s.Email
	storageService = s.Storage
	streamService = s.Stream
}

func ensureService(c *gin.Context, svc any) bool {
	if svc == nil {
		response.InternalError(c, "service not configured")
		return false
	}
	return true
}
