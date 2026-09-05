package agent

import (
	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
	"github.com/horizonlabs/pulsarfi-backend/src/service"
	agentsvc "github.com/horizonlabs/pulsarfi-backend/src/service/agent"
)

var taskSvc *agentsvc.TaskService

func ConfigureServices(s *service.Registry) {
	taskSvc = s.AgentTask
}

func ensureService(c *gin.Context) bool {
	if taskSvc == nil {
		response.InternalError(c, "agent task service not configured")
		return false
	}
	return true
}
