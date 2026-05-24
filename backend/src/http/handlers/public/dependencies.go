package public

import (
	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
)

var repos *repository.Registry

func ConfigureRepos(r *repository.Registry) {
	repos = r
}

func ensureRepos(c *gin.Context) bool {
	if repos == nil {
		response.InternalError(c, "repos not configured")
		return false
	}
	return true
}
