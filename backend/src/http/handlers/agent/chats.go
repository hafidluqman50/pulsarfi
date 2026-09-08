package agent

import (
	"errors"
	"fmt"
	"log/slog"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	usermw "github.com/horizonlabs/pulsarfi-backend/src/http/middleware/user"
	chatrequest "github.com/horizonlabs/pulsarfi-backend/src/http/request/agent/chat"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
	"github.com/horizonlabs/pulsarfi-backend/src/model"
	agentsvc "github.com/horizonlabs/pulsarfi-backend/src/service/agent"
	"github.com/horizonlabs/pulsarfi-backend/src/service/realtime"
)

// chatStreamTopic is the per-chat WebSocket topic a client subscribes to
// (via the existing GET /api/v1/realtime/ws infrastructure,
// docs/plans/realtime-websocket-updates.md) *before* sending a message —
// chat_id is a client-generated UUID, known upfront, unlike a Task's own id
// which does not exist until the route node opens one mid-turn. Replaces
// the old per-request SSE stream entirely
// (docs/plans/agent-orchestration-graph-rebuild.md v2.6): "no SSE, disini
// pake socket."
func chatStreamTopic(chatID uuid.UUID) string {
	return fmt.Sprintf("agent-chat-stream:%s", chatID.String())
}

type chatStreamEvent struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

func PostChatMessageHandler(c *gin.Context) {
	if !ensureService(c) {
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

	runAgentOverSocket(c, chatID, func(events agentsvc.AgentEventCallbacks) (agentsvc.WorkflowCard, error) {
		return taskSvc.HandleChatMessage(c.Request.Context(), chatID, claims.WalletAddress, messageRequest.Message, events)
	})
}

func RetryLastMessageHandler(c *gin.Context) {
	if !ensureService(c) {
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

	runAgentOverSocket(c, chatID, func(events agentsvc.AgentEventCallbacks) (agentsvc.WorkflowCard, error) {
		return taskSvc.RetryLastMessage(c.Request.Context(), chatID, claims.WalletAddress, events)
	})
}

// runAgentOverSocket is a plain, blocking HTTP request-response — the
// request only ever completes once the whole turn finishes, same as any
// other endpoint in this API. Live progress (sub_task/sub_task_started/
// reply_delta) is pushed out-of-band, over the socket, as it happens; the
// HTTP response body itself only ever carries the final result or an
// error, never the live events — a client that never subscribed to the
// topic still gets a correct, complete response, it just misses the live
// play-by-play.
func runAgentOverSocket(c *gin.Context, chatID uuid.UUID, run func(events agentsvc.AgentEventCallbacks) (agentsvc.WorkflowCard, error)) {
	topic := chatStreamTopic(chatID)

	workflowCard, err := run(agentsvc.AgentEventCallbacks{
		OnSubTask: func(row model.AgentSubTask) {
			realtime.Publish(topic, chatStreamEvent{Type: "sub_task", Data: row})
		},
		OnSubTaskStarted: func(agentName, stepName, label string) {
			realtime.Publish(topic, chatStreamEvent{Type: "sub_task_started", Data: gin.H{"agent": agentName, "step_name": stepName, "label": label}})
		},
		OnTextDelta: func(delta string) {
			realtime.Publish(topic, chatStreamEvent{Type: "reply_delta", Data: gin.H{"delta": delta}})
		},
		OnToolCall: func(agentName, toolName, phase string) {
			realtime.Publish(topic, chatStreamEvent{Type: "tool_call", Data: gin.H{"agent": agentName, "tool": toolName, "phase": phase}})
		},
	})

	switch {
	case errors.Is(err, agentsvc.ErrWalletMismatch):
		response.Forbidden(c, "chat does not belong to the authenticated wallet")
		return
	case errors.Is(err, agentsvc.ErrNothingToRetry):
		response.BadRequest(c, "nothing to retry")
		return
	case err != nil:
		slog.ErrorContext(c.Request.Context(), "agent: process chat message failed", "error", err)
		realtime.Publish(topic, chatStreamEvent{Type: "error", Data: gin.H{"message": agentsvc.QuasarErrorMessage}})
		response.InternalError(c, agentsvc.QuasarErrorMessage)
		return
	}

	realtime.Publish(topic, chatStreamEvent{Type: "final", Data: workflowCard})
	response.OK(c, "message processed", workflowCard)
}

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

func GetChatMessagesHandler(c *gin.Context) {
	if !ensureService(c) {
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

	messages, err := taskSvc.GetChatMessages(c.Request.Context(), chatID, claims.WalletAddress)
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
