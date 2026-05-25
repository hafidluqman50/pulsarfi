package custodian

import (
	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	custodiansvc "github.com/horizonlabs/pulsarfi-backend/src/service/custodian"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
)

var (
	repos          *repository.Registry
	custodianSvc   *custodiansvc.CustodianService
	emailService   *external.EmailService
	storageService *external.StorageService
	streamService  *external.StreamService
)

type Services struct {
	Repos     *repository.Registry
	Custodian *custodiansvc.CustodianService
	Email     *external.EmailService
	Storage   *external.StorageService
	Stream    *external.StreamService
}

func ConfigureServices(s Services) {
	repos = s.Repos
	custodianSvc = s.Custodian
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
