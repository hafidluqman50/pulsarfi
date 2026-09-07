package agent

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	usermw "github.com/horizonlabs/pulsarfi-backend/src/http/middleware/user"
	chatrequest "github.com/horizonlabs/pulsarfi-backend/src/http/request/agent/chat"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
	"github.com/horizonlabs/pulsarfi-backend/src/model"
	agentsvc "github.com/horizonlabs/pulsarfi-backend/src/service/agent"
)

type sseEvent struct {
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

	streamAgentRun(c, func(onSubTask func(model.AgentSubTask), onSubTaskStarted func(agentName, stepName, label string), onTextDelta func(string)) (agentsvc.WorkflowCard, error) {
		return taskSvc.HandleChatMessage(c.Request.Context(), chatID, claims.WalletAddress, messageRequest.Message, onSubTask, onSubTaskStarted, onTextDelta)
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

	streamAgentRun(c, func(onSubTask func(model.AgentSubTask), onSubTaskStarted func(agentName, stepName, label string), onTextDelta func(string)) (agentsvc.WorkflowCard, error) {
		return taskSvc.RetryLastMessage(c.Request.Context(), chatID, claims.WalletAddress, onSubTask, onSubTaskStarted, onTextDelta)
	})
}

func streamAgentRun(c *gin.Context, run func(onSubTask func(model.AgentSubTask), onSubTaskStarted func(agentName, stepName, label string), onTextDelta func(string)) (agentsvc.WorkflowCard, error)) {
	flusher, canFlush := c.Writer.(http.Flusher)
	if !canFlush {
		response.InternalError(c, "streaming not supported")
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no")
	c.Writer.WriteHeader(http.StatusOK)
	flusher.Flush()

	events := make(chan sseEvent, 32)
	done := make(chan struct{})

	go func() {
		defer close(done)
		workflowCard, err := run(
			func(row model.AgentSubTask) {
				events <- sseEvent{Type: "sub_task", Data: row}
			},
			func(agentName, stepName, label string) {
				events <- sseEvent{Type: "sub_task_started", Data: gin.H{"agent": agentName, "step_name": stepName, "label": label}}
			},
			func(delta string) {
				events <- sseEvent{Type: "reply_delta", Data: gin.H{"delta": delta}}
			},
		)
		switch {
		case errors.Is(err, agentsvc.ErrWalletMismatch):
			events <- sseEvent{Type: "error", Data: gin.H{"message": "chat does not belong to the authenticated wallet"}}
		case errors.Is(err, agentsvc.ErrNothingToRetry):
			events <- sseEvent{Type: "error", Data: gin.H{"message": "nothing to retry"}}
		case err != nil:
			slog.ErrorContext(c.Request.Context(), "agent: process chat message failed", "error", err)
			events <- sseEvent{Type: "error", Data: gin.H{"message": "failed to process message"}}
		default:
			events <- sseEvent{Type: "final", Data: workflowCard}
		}
	}()

	write := func(ev sseEvent) {
		payload, err := json.Marshal(ev)
		if err != nil {
			return
		}
		fmt.Fprintf(c.Writer, "data: %s\n\n", payload)
		flusher.Flush()
	}

	for {
		select {
		case ev := <-events:
			write(ev)
		case <-done:
			for {
				select {
				case ev := <-events:
					write(ev)
				default:
					return
				}
			}
		case <-c.Request.Context().Done():
			return
		}
	}
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
