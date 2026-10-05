package agent

import (
	"errors"
	"log/slog"

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

	runAgentOverSocket(c, chatID, func() (agentsvc.WorkflowCard, error) {
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

	runAgentOverSocket(c, chatID, func() (agentsvc.WorkflowCard, error) {
		return chatSvc.RetryLastMessage(c.Request.Context(), chatID, claims.WalletAddress)
	})
}

// runAgentOverSocket is a plain, blocking HTTP request-response — the
// request only ever completes once the whole turn finishes, same as any
// other endpoint in this API. Live progress (sub_tasks, reply_delta,
// thinking, tool_call) is pushed out-of-band over the chat's WebSocket topic
// by the agent package as it happens, and sub_tasks always carries the
// turn's full, current sub-task list, never a single fragment. The HTTP
// response body itself only ever carries the final result or an error — a
// client that never subscribed to the topic still gets a correct, complete
// response, it just misses the live play-by-play.
func runAgentOverSocket(c *gin.Context, chatID uuid.UUID, run func() (agentsvc.WorkflowCard, error)) {
	workflowCard, err := run()

	switch {
	case errors.Is(err, agentsvc.ErrWalletMismatch):
		response.Forbidden(c, "chat does not belong to the authenticated wallet")
		return
	case errors.Is(err, agentsvc.ErrNothingToRetry):
		response.BadRequest(c, "nothing to retry")
		return
	case err != nil:
		slog.ErrorContext(c.Request.Context(), "agent: process chat message failed", "error", err)
		agentsvc.PublishChatEvent(chatID, "error", gin.H{"message": "Failed to process chat message"})
		response.InternalError(c, "Failed to process chat message")
		return
	}

	agentsvc.PublishChatEvent(chatID, "final", workflowCard)
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
