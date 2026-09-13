package agent

import (
	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
	"github.com/horizonlabs/pulsarfi-backend/src/service"
	agentsvc "github.com/horizonlabs/pulsarfi-backend/src/service/agent"
	publicsvc "github.com/horizonlabs/pulsarfi-backend/src/service/public"
)

var taskSvc *agentsvc.TaskService
var chatSvc *agentsvc.ChatService
var chartSvc *publicsvc.PortfolioChartReader

func ConfigureServices(s *service.Registry) {
	taskSvc = s.AgentTask
	chatSvc = s.AgentChat
	chartSvc = s.PortfolioChart
}

func ensureService(c *gin.Context) bool {
	if taskSvc == nil {
		response.InternalError(c, "agent task service not configured")
		return false
	}
	return true
}

func ensureChatService(c *gin.Context) bool {
	if chatSvc == nil {
		response.InternalError(c, "agent chat service not configured")
		return false
	}
	return true
}
