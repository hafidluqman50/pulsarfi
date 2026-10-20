package agent

import (
	"errors"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	usermw "github.com/horizonlabs/pulsarfi-backend/src/http/middleware/user"
	chatrequest "github.com/horizonlabs/pulsarfi-backend/src/http/request/agent/chat"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
	agentsvc "github.com/horizonlabs/pulsarfi-backend/src/service/agent"
)

func PostChatMessageHandler(c *gin.Context) {
	if !ensureChatService(c) {
		return
	}
	claims, ok := usermw.Get(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	chatID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid chat id")
		return
	}
	messageRequest, err := chatrequest.NewMessageRequest(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	respondWithTurn(c, func() (agentsvc.WorkflowCard, error) {
		return chatSvc.HandleChatMessage(c.Request.Context(), chatID, claims.WalletAddress, messageRequest.Message, messageRequest.Hidden)
	})
}

func RetryLastMessageHandler(c *gin.Context) {
	if !ensureChatService(c) {
		return
	}
	claims, ok := usermw.Get(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	chatID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid chat id")
		return
	}

	respondWithTurn(c, func() (agentsvc.WorkflowCard, error) {
		return chatSvc.RetryLastMessage(c.Request.Context(), chatID, claims.WalletAddress)
	})
}

// respondWithTurn is a plain, blocking HTTP request-response: the request only
// completes once the whole turn finishes, and the body carries only the final
// result or an error. Live progress (sub_tasks, reply_delta, thinking,
// tool_call) and the terminal event are published by the agent package out of
// band over the chat's WebSocket topic as the turn runs — this handler owns no
// event, only the HTTP status mapping.
func respondWithTurn(c *gin.Context, run func() (agentsvc.WorkflowCard, error)) {
	workflowCard, err := run()

	switch {
	case errors.Is(err, agentsvc.ErrWalletMismatch):
		response.Forbidden(c, "chat does not belong to the authenticated wallet")
		return
	case errors.Is(err, agentsvc.ErrNothingToRetry):
		response.BadRequest(c, "nothing to retry")
		return
	case errors.Is(err, agentsvc.ErrTurnInFlight):
		response.Conflict(c, "a turn is already running for this chat")
		return
	case err != nil:
		response.InternalError(c, "Failed to process chat message")
		return
	}

	response.OK(c, "message processed", workflowCard)
}

func ListChatsHandler(c *gin.Context) {
	if !ensureChatService(c) {
		return
	}
	claims, ok := usermw.Get(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}

	chats, err := chatSvc.ListChats(c.Request.Context(), claims.WalletAddress)
	if err != nil {
		response.InternalError(c, "failed to fetch chats")
		return
	}

	response.OK(c, "chats retrieved", chats)
}

func GetChatMessagesHandler(c *gin.Context) {
	if !ensureChatService(c) {
		return
	}
	claims, ok := usermw.Get(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	chatID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid chat id")
		return
	}

	messages, err := chatSvc.GetChatMessages(c.Request.Context(), chatID, claims.WalletAddress)
	if errors.Is(err, agentsvc.ErrWalletMismatch) {
		response.Forbidden(c, "chat does not belong to the authenticated wallet")
		return
	}
	if err != nil {
		response.InternalError(c, "failed to fetch chat messages")
		return
	}

	response.OK(c, "chat messages retrieved", messages)
}
