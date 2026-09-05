package agent

import (
	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/auth"
	agentHandler "github.com/horizonlabs/pulsarfi-backend/src/http/handlers/agent"
	usermw "github.com/horizonlabs/pulsarfi-backend/src/http/middleware/user"
)

// RegisterRoutes gates every route behind usermw.Auth — private to the
// owning wallet. The reasoning endpoint is deliberately NOT gated: its
// entire purpose is third-party verification against the on-chain
// reasoningHash, not just the task owner's own use. Task creation now
// happens implicitly via chat intake (create_task, a Supervisor tool) —
// there is no more direct POST /tasks endpoint.
func RegisterRoutes(rg *gin.RouterGroup, jwtConfig auth.Config) {
	protected := rg.Group("", usermw.Auth(jwtConfig))
	protected.GET("/tasks", agentHandler.ListTasksHandler)
	protected.POST("/tasks/:id/arm", agentHandler.ArmTaskHandler)
	protected.POST("/tasks/:id/disarm", agentHandler.DisarmTaskHandler)
	protected.POST("/tasks/:id/pause", agentHandler.PauseTaskHandler)
	protected.POST("/tasks/:id/resume", agentHandler.ResumeTaskHandler)
	protected.GET("/tasks/:id/trades", agentHandler.GetTaskTradesHandler)
	protected.POST("/chats", agentHandler.CreateChatHandler)
	protected.GET("/chats", agentHandler.ListChatsHandler)
	protected.GET("/chats/:id/messages", agentHandler.GetChatMessagesHandler)
	protected.POST("/chats/:id/messages", agentHandler.PostChatMessageHandler)

	rg.GET("/tasks/:id/reasoning", agentHandler.GetReasoningHandler)
}
