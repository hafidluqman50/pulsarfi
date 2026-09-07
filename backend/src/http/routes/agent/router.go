package agent

import (
	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/auth"
	agentHandler "github.com/horizonlabs/pulsarfi-backend/src/http/handlers/agent"
	usermw "github.com/horizonlabs/pulsarfi-backend/src/http/middleware/user"
)

func RegisterRoutes(rg *gin.RouterGroup, jwtConfig auth.Config) {
	protected := rg.Group("", usermw.Auth(jwtConfig))
	protected.GET("/tasks", agentHandler.ListTasksHandler)
	protected.GET("/activity", agentHandler.GetActivityHandler)
	protected.POST("/tasks/:id/arm", agentHandler.ArmTaskHandler)
	protected.POST("/tasks/:id/disarm", agentHandler.DisarmTaskHandler)
	protected.POST("/tasks/:id/pause", agentHandler.PauseTaskHandler)
	protected.POST("/tasks/:id/resume", agentHandler.ResumeTaskHandler)
	protected.GET("/tasks/:id/trades", agentHandler.GetTaskTradesHandler)
	protected.GET("/chats", agentHandler.ListChatsHandler)
	protected.GET("/portfolio/chart", agentHandler.GetPortfolioChartHandler)
	protected.GET("/chats/:id/messages", agentHandler.GetChatMessagesHandler)
	protected.POST("/chats/:id/messages", agentHandler.PostChatMessageHandler)
	protected.POST("/chats/:id/messages/retry", agentHandler.RetryLastMessageHandler)

	rg.GET("/tasks/:id/reasoning", agentHandler.GetReasoningHandler)
}
