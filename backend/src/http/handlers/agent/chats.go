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

// liveSubTaskEntry is one row of the turn's own sub-task list, exactly as
// pushed to the frontend — the frontend never sees a fragment event for a
// single step, only this full list, so it never has to decide whether an
// incoming row is a new step, a retry, or a completion of something already
// in the list. Every decision about what happened lives here, in the
// backend, which is the only side that actually knows it.
type liveSubTaskEntry struct {
	Agent    string              `json:"agent"`
	StepName string              `json:"step_name"`
	Label    string              `json:"label,omitempty"`
	Status   string              `json:"status"` // in_progress | done | failed
	Reason   string              `json:"reason,omitempty"`
	Row      *model.AgentSubTask `json:"row,omitempty"`
}

// liveSubTasks accumulates one turn's sub-task list and republishes the
// whole thing on every change — scoped to a single runAgentOverSocket call
// (one HTTP request), never shared across turns or requests.
type liveSubTasks struct {
	topic   string
	entries []liveSubTaskEntry
}

func (l *liveSubTasks) publish() {
	realtime.Publish(l.topic, chatStreamEvent{Type: "sub_tasks", Data: l.entries})
}

func (l *liveSubTasks) started(agentName, stepName, label string) {
	l.entries = append(l.entries, liveSubTaskEntry{Agent: agentName, StepName: stepName, Label: label, Status: "in_progress"})
	l.publish()
}

// lastInProgress finds the most recent still-in_progress entry for
// (agentName, stepName) — the one a failure or completion belongs to. A
// step can legitimately run more than once in one turn (a retry after a
// graceful failure), so the *last* match, not the first, is always the
// right one.
func (l *liveSubTasks) lastInProgress(agentName, stepName string) int {
	for i := len(l.entries) - 1; i >= 0; i-- {
		if l.entries[i].Status == "in_progress" && l.entries[i].Agent == agentName && l.entries[i].StepName == stepName {
			return i
		}
	}
	return -1
}

func (l *liveSubTasks) failed(agentName, stepName, reason string) {
	if i := l.lastInProgress(agentName, stepName); i != -1 {
		l.entries[i].Status = "failed"
		l.entries[i].Reason = reason
	} else {
		l.entries = append(l.entries, liveSubTaskEntry{Agent: agentName, StepName: stepName, Status: "failed", Reason: reason})
	}
	l.publish()
}

func (l *liveSubTasks) done(row model.AgentSubTask) {
	label := ""
	if row.Label != nil {
		label = *row.Label
	}
	if i := l.lastInProgress(row.Agent, row.StepName); i != -1 {
		l.entries[i].Status = "done"
		l.entries[i].Label = label
		l.entries[i].Row = &row
	} else {
		l.entries = append(l.entries, liveSubTaskEntry{Agent: row.Agent, StepName: row.StepName, Label: label, Status: "done", Row: &row})
	}
	l.publish()
}

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

	runAgentOverSocket(c, chatID, func(events agentsvc.AgentEventCallbacks) (agentsvc.WorkflowCard, error) {
		return chatSvc.HandleChatMessage(c.Request.Context(), chatID, claims.WalletAddress, messageRequest.Message, messageRequest.Hidden, events)
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

	runAgentOverSocket(c, chatID, func(events agentsvc.AgentEventCallbacks) (agentsvc.WorkflowCard, error) {
		return chatSvc.RetryLastMessage(c.Request.Context(), chatID, claims.WalletAddress, events)
	})
}

// runAgentOverSocket is a plain, blocking HTTP request-response — the
// request only ever completes once the whole turn finishes, same as any
// other endpoint in this API. Live progress (sub_tasks/reply_delta) is
// pushed out-of-band, over the socket, as it happens — sub_tasks always
// carries the turn's full, current sub-task list (see liveSubTasks), never
// a single fragment the frontend would have to merge itself. The HTTP
// response body itself only ever carries the final result or an error,
// never the live events — a client that never subscribed to the topic
// still gets a correct, complete response, it just misses the live
// play-by-play.
func runAgentOverSocket(c *gin.Context, chatID uuid.UUID, run func(events agentsvc.AgentEventCallbacks) (agentsvc.WorkflowCard, error)) {
	topic := chatStreamTopic(chatID)
	subTasks := &liveSubTasks{topic: topic}

	workflowCard, err := run(agentsvc.AgentEventCallbacks{
		OnSubTask:        subTasks.done,
		OnSubTaskStarted: subTasks.started,
		OnSubTaskFailed:  subTasks.failed,
		OnTextDelta: func(delta string) {
			realtime.Publish(topic, chatStreamEvent{Type: "reply_delta", Data: gin.H{"delta": delta}})
		},
		OnToolCall: func(agentName, toolName, phase string) {
			realtime.Publish(topic, chatStreamEvent{Type: "tool_call", Data: gin.H{"agent": agentName, "tool": toolName, "phase": phase}})
		},
		OnThinking: func(agentName, delta string) {
			realtime.Publish(topic, chatStreamEvent{Type: "thinking", Data: gin.H{"agent": agentName, "delta": delta}})
		},
		OnFinalizing: func() {
			realtime.Publish(topic, chatStreamEvent{Type: "finalizing", Data: gin.H{}})
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
		realtime.Publish(topic, chatStreamEvent{Type: "error", Data: gin.H{"message": "Failed to process chat message"}})
		response.InternalError(c, "Failed to process chat message")
		return
	}

	realtime.Publish(topic, chatStreamEvent{Type: "final", Data: workflowCard})
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
