package agent

import (
	"errors"
	"log/slog"
	"strconv"

	"github.com/gin-gonic/gin"
	usermw "github.com/horizonlabs/pulsarfi-backend/src/http/middleware/user"
	chatrequest "github.com/horizonlabs/pulsarfi-backend/src/http/request/agent/chat"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
	agentsvc "github.com/horizonlabs/pulsarfi-backend/src/service/agent"
)

// PostChatMessageHandler feeds one prompt into Supervisor's chat-intake
// flow and returns whatever card shape this turn produced — a plain
// reply, or a chart. It never itself calls createTask on-chain — that
// only happens at ArmTaskHandler, once every question is answered.
func PostChatMessageHandler(c *gin.Context) {
	if !ensureService(c) {
		return
	}
	claims, ok := usermw.Get(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	chatID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid chat id")
		return
	}
	messageRequest, err := chatrequest.NewMessageRequest(c)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	workflowCard, err := taskSvc.HandleChatMessage(c.Request.Context(), chatID, claims.WalletAddress, messageRequest.Message)
	if errors.Is(err, agentsvc.ErrChatNotFound) {
		response.NotFound(c, "chat not found")
		return
	}
	if errors.Is(err, agentsvc.ErrWalletMismatch) {
		response.Forbidden(c, "chat does not belong to the authenticated wallet")
		return
	}
	if err != nil {
		slog.ErrorContext(c.Request.Context(), "agent: process chat message failed", "chat_id", chatID, "error", err)
		response.InternalError(c, "failed to process message")
		return
	}

	response.OK(c, "message processed", workflowCard)
}

// CreateChatHandler opens a new, empty conversation thread — "+ new chat".
// No Task exists yet; one only appears once a message in this chat is
// recognized as a genuinely distinct, data-needing request.
func CreateChatHandler(c *gin.Context) {
	if !ensureService(c) {
		return
	}
	claims, ok := usermw.Get(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}

	chat, err := taskSvc.CreateChat(c.Request.Context(), claims.WalletAddress, nil)
	if err != nil {
		response.InternalError(c, "failed to create chat")
		return
	}

	response.Created(c, "chat created", chat)
}

// ListChatsHandler always scopes to the authenticated wallet — chat
// history is private.
func ListChatsHandler(c *gin.Context) {
	if !ensureService(c) {
		return
	}
	claims, ok := usermw.Get(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}

	chats, err := taskSvc.ListChats(c.Request.Context(), claims.WalletAddress)
	if err != nil {
		response.InternalError(c, "failed to fetch chats")
		return
	}

	response.OK(c, "chats retrieved", chats)
}

// GetChatMessagesHandler returns a chat's full transcript in order — what
// the Quasar panel replays when a user reopens an existing chat.
func GetChatMessagesHandler(c *gin.Context) {
	if !ensureService(c) {
		return
	}
	claims, ok := usermw.Get(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	chatID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid chat id")
		return
	}

	messages, err := taskSvc.GetChatMessages(c.Request.Context(), chatID, claims.WalletAddress)
	if errors.Is(err, agentsvc.ErrChatNotFound) {
		response.NotFound(c, "chat not found")
		return
	}
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
