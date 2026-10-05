package agent

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/horizonlabs/pulsarfi-backend/src/contracts"
	dbmodel "github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/service/realtime"
)

// ChatStreamTopic is the per-chat WebSocket topic a client subscribes to
// (via the existing GET /api/v1/realtime/ws infrastructure,
// docs/plans/realtime-websocket-updates.md) *before* sending a message —
// chat_id is a client-generated UUID, known upfront, unlike a Task's own id
// which does not exist until open_task runs mid-turn.
func ChatStreamTopic(chatID uuid.UUID) string {
	return fmt.Sprintf("agent-chat-stream:%s", chatID.String())
}

type chatStreamEvent struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

func PublishChatEvent(chatID uuid.UUID, eventType string, data any) {
	realtime.Publish(ChatStreamTopic(chatID), chatStreamEvent{Type: eventType, Data: data})
}

func publishChatEvent(topic, eventType string, data any) {
	realtime.Publish(topic, chatStreamEvent{Type: eventType, Data: data})
}

// liveSubTaskEntry is one row of the turn's own sub-task list, exactly as
// pushed to the frontend — the frontend never sees a fragment event for a
// single step, only this full list, so it never has to decide whether an
// incoming row is a new step, a retry, or a completion of something already
// in the list. Every decision about what happened lives here, in the
// backend, which is the only side that actually knows it.
type liveSubTaskEntry struct {
	Agent    string                `json:"agent"`
	StepName string                `json:"step_name"`
	Label    string                `json:"label,omitempty"`
	Status   string                `json:"status"` // in_progress | done | failed
	Reason   string                `json:"reason,omitempty"`
	Row      *dbmodel.AgentSubTask `json:"row,omitempty"`
}

// liveSubTasks accumulates one turn's sub-task list and republishes the
// whole thing on every change. Tools call it from the agent's goroutine
// while the event loop runs on the request's, hence the mutex.
type liveSubTasks struct {
	mu      sync.Mutex
	topic   string
	entries []liveSubTaskEntry
}

func (l *liveSubTasks) publish() {
	snapshot := make([]liveSubTaskEntry, len(l.entries))
	copy(snapshot, l.entries)
	publishChatEvent(l.topic, "sub_tasks", snapshot)
}

func (l *liveSubTasks) started(agentName, stepName, label string) {
	l.mu.Lock()
	defer l.mu.Unlock()
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
	l.mu.Lock()
	defer l.mu.Unlock()
	if i := l.lastInProgress(agentName, stepName); i != -1 {
		l.entries[i].Status = "failed"
		l.entries[i].Reason = reason
	} else {
		l.entries = append(l.entries, liveSubTaskEntry{Agent: agentName, StepName: stepName, Status: "failed", Reason: reason})
	}
	l.publish()
}

func (l *liveSubTasks) done(row dbmodel.AgentSubTask) {
	l.mu.Lock()
	defer l.mu.Unlock()
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

type turnOutcome struct {
	reply     strings.Builder
	novaCalls []toolCallResult
	interrupt *interruptOutcome
}

type interruptOutcome struct {
	kind      string
	id        string
	questions []contracts.IntakeField
	message   string
}

// consume is the one loop over the Runner's event stream. Quasar, Nova and
// Comet all arrive here, told apart by AgentName: Eino forwards the events of
// the agents Quasar calls into Quasar's own iterator. Every live signal the
// frontend gets is published from this loop (or from the Sub Task recorder).
func (o *Orchestrator) consume(ctx context.Context, rc *RunContext, iter *adk.AsyncIterator[*adk.AgentEvent], keepCheckpoint bool) (OrchestratorResult, error) {
	topic := ChatStreamTopic(rc.ChatID)
	var outcome turnOutcome
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return OrchestratorResult{}, event.Err
		}
		if event.Action != nil && event.Action.Interrupted != nil {
			outcome.interrupt = interruptFrom(event.Action.Interrupted)
			continue
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		if err := publishMessage(topic, frontendAgentName(event.AgentName), event.Output.MessageOutput, &outcome); err != nil {
			return OrchestratorResult{}, err
		}
	}
	return o.finish(ctx, rc, &outcome, keepCheckpoint), nil
}

func (o *Orchestrator) finish(ctx context.Context, rc *RunContext, outcome *turnOutcome, keepCheckpoint bool) OrchestratorResult {
	result := OrchestratorResult{TaskID: rc.TaskID, OnChainTaskID: rc.OnChainTaskID, Reply: outcome.reply.String()}

	switch {
	case outcome.interrupt != nil:
		o.storeInterrupt(ctx, rc.ChatID, outcome.interrupt.kind, outcome.interrupt.id)
		if outcome.interrupt.kind == interruptQuestions {
			result.PendingQuestions = outcome.interrupt.questions
		}
		// A card needs a sentence above it. The pause carries one; use it
		// only when the model did not already write text this turn.
		if strings.TrimSpace(result.Reply) == "" && outcome.interrupt.message != "" {
			result.Reply = outcome.interrupt.message
			PublishChatEvent(rc.ChatID, "reply_delta", map[string]any{"delta": outcome.interrupt.message})
		}
	case !keepCheckpoint && rc.ChatID != uuid.Nil:
		_ = o.CheckPointStore.Delete(ctx, rc.ChatID.String())
	}

	result.ContentType, result.UIProps = classifyReply(result.PendingQuestions, o.cardOfTask(ctx, rc.TaskID), outcome.novaCalls)
	if result.ContentType == "workflow_card" {
		result.UIComponent = "clarifying_questions"
	}
	return result
}

func interruptFrom(info *adk.InterruptInfo) *interruptOutcome {
	for _, interruptCtx := range info.InterruptContexts {
		if !interruptCtx.IsRootCause {
			continue
		}
		switch payload := interruptCtx.Info.(type) {
		case QuestionsInterrupt:
			return &interruptOutcome{kind: interruptQuestions, id: interruptCtx.ID, questions: payload.Questions, message: payload.Message}
		case *QuestionsInterrupt:
			return &interruptOutcome{kind: interruptQuestions, id: interruptCtx.ID, questions: payload.Questions, message: payload.Message}
		case ArmInterrupt:
			return &interruptOutcome{kind: interruptArm, id: interruptCtx.ID, message: payload.Message}
		case *ArmInterrupt:
			return &interruptOutcome{kind: interruptArm, id: interruptCtx.ID, message: payload.Message}
		}
	}
	return nil
}

// frontendAgentName maps an Eino agent name to the agent label the frontend
// already knows from the Sub Task rows.
func frontendAgentName(adkName string) string {
	switch adkName {
	case "analyzer_agent":
		return "analyzer"
	case "executor_agent":
		return "executor"
	default:
		return "supervisor"
	}
}

func publishMessage(topic, agentName string, output *adk.MessageVariant, outcome *turnOutcome) error {
	switch output.Role {
	case schema.Assistant:
		return publishAssistant(topic, agentName, output, outcome)
	case schema.Tool:
		return publishToolResult(topic, agentName, output, outcome)
	}
	return nil
}

// publishAssistant forwards a model message as it streams: Quasar's text is
// the reply the user reads, Nova's and Comet's is what the UI shows as their
// thinking. Real reasoning wins when the model returns it; a message without
// any falls back to its text, as before.
func publishAssistant(topic, agentName string, output *adk.MessageVariant, outcome *turnOutcome) error {
	isQuasar := agentName == "supervisor"
	sawReasoning := false

	// A reasoning model streams thousands of tiny deltas per turn, more than
	// the hub's send buffer keeps up with (it drops updates when full, seen
	// live: ~5600 events from one Nova turn). Thinking is batched; the reply
	// is short and goes out as it comes.
	var pendingThinking strings.Builder
	flushThinking := func() {
		if pendingThinking.Len() == 0 {
			return
		}
		publishChatEvent(topic, "thinking", map[string]any{"agent": agentName, "delta": pendingThinking.String()})
		pendingThinking.Reset()
	}
	addThinking := func(delta string) {
		pendingThinking.WriteString(delta)
		if pendingThinking.Len() >= thinkingBatchBytes {
			flushThinking()
		}
	}
	emit := func(chunk *schema.Message) {
		if chunk.ReasoningContent != "" {
			sawReasoning = true
			addThinking(chunk.ReasoningContent)
		}
		if chunk.Content == "" {
			return
		}
		switch {
		case isQuasar:
			outcome.reply.WriteString(chunk.Content)
			publishChatEvent(topic, "reply_delta", map[string]any{"delta": chunk.Content})
		case !sawReasoning:
			addThinking(chunk.Content)
		}
	}

	var message *schema.Message
	if output.IsStreaming {
		defer output.MessageStream.Close()
		var chunks []*schema.Message
		for {
			chunk, err := output.MessageStream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				flushThinking()
				return fmt.Errorf("orchestrator: read %s stream: %w", agentName, err)
			}
			emit(chunk)
			chunks = append(chunks, chunk)
		}
		if len(chunks) > 0 {
			concatenated, err := schema.ConcatMessages(chunks)
			if err != nil {
				flushThinking()
				return fmt.Errorf("orchestrator: concat %s stream: %w", agentName, err)
			}
			message = concatenated
		}
	} else if output.Message != nil {
		emit(output.Message)
		message = output.Message
	}
	flushThinking()

	if !isQuasar && message != nil {
		for _, call := range message.ToolCalls {
			publishChatEvent(topic, "tool_call", map[string]any{"agent": agentName, "tool": call.Function.Name, "phase": "start"})
		}
	}
	return nil
}

const thinkingBatchBytes = 160

// publishToolResult closes the tool_call signal for Nova's and Comet's own
// tools and keeps Nova's results, which classifyReply turns into charts and
// news cards. Quasar's own calls (open_task, analyzer_agent, ...) are
// hand-offs, already shown as Sub Tasks, not tool activity.
func publishToolResult(topic, agentName string, output *adk.MessageVariant, outcome *turnOutcome) error {
	if agentName == "supervisor" {
		return nil
	}
	message, err := output.GetMessage()
	if err != nil {
		return fmt.Errorf("orchestrator: read %s tool result: %w", agentName, err)
	}
	toolName := output.ToolName
	if toolName == "" && message != nil {
		toolName = message.ToolName
	}
	publishChatEvent(topic, "tool_call", map[string]any{"agent": agentName, "tool": toolName, "phase": "end"})
	if agentName == "analyzer" && message != nil {
		outcome.novaCalls = append(outcome.novaCalls, toolCallResult{ToolName: toolName, Result: message.Content})
	}
	return nil
}

const statusTimeout = 4 * time.Second

// statusInstructions states the task only and names no language: the
// language comes from the user's own message, never from code.
const statusInstructions = `Write at most four words that mean "Quasar is thinking", in the same language as the user's message below. Output only those words with a trailing ellipsis: no quotes, no explanation, nothing else.`

// announceStatus runs beside the turn and gives the chat's placeholder its
// words in the user's language. It is best effort: a missing model, a
// failure or a timeout just leaves the placeholder as a bare dot, and never
// touches the turn itself.
func (o *Orchestrator) announceStatus(ctx context.Context, chatID uuid.UUID, userMessage string) {
	if o.ReplyModel == nil || strings.TrimSpace(userMessage) == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), statusTimeout)
	defer cancel()

	reply, err := o.ReplyModel.Generate(ctx, []*schema.Message{
		schema.SystemMessage(statusInstructions),
		schema.UserMessage(userMessage),
	})
	if err != nil {
		slog.DebugContext(ctx, "orchestrator: status phrase failed", "error", err)
		return
	}
	text := ""
	if reply != nil {
		text = strings.TrimSpace(reply.Content)
	}
	if text == "" {
		return
	}
	PublishChatEvent(chatID, "status", map[string]any{"text": text})
}
