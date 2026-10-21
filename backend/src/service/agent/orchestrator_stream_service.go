package agent

import (
	"context"
	"errors"
	"fmt"
	agentcore "github.com/horizonlabs/pulsarfi-backend/src/app/core/agent"
	"io"
	"strings"
	"sync"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	helper "github.com/horizonlabs/pulsarfi-backend/src/app/helper"
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

func (live *liveSubTasks) publish() {
	snapshot := make([]liveSubTaskEntry, len(live.entries))
	copy(snapshot, live.entries)
	publishChatEvent(live.topic, "sub_tasks", snapshot)
}

func (live *liveSubTasks) started(agentName, stepName, label string) {
	live.mu.Lock()
	defer live.mu.Unlock()
	live.entries = append(live.entries, liveSubTaskEntry{Agent: agentName, StepName: stepName, Label: label, Status: "in_progress"})
	live.publish()
}

// lastInProgress finds the most recent still-in_progress entry for
// (agentName, stepName) — the one a failure or completion belongs to. A
// step can legitimately run more than once in one turn (a retry after a
// graceful failure), so the *last* match, not the first, is always the
// right one.
func (live *liveSubTasks) lastInProgress(agentName, stepName string) int {
	for i := len(live.entries) - 1; i >= 0; i-- {
		if live.entries[i].Status == "in_progress" && live.entries[i].Agent == agentName && live.entries[i].StepName == stepName {
			return i
		}
	}
	return -1
}

func (live *liveSubTasks) failed(agentName, stepName, reason string) {
	live.mu.Lock()
	defer live.mu.Unlock()
	if i := live.lastInProgress(agentName, stepName); i != -1 {
		live.entries[i].Status = "failed"
		live.entries[i].Reason = reason
	} else {
		live.entries = append(live.entries, liveSubTaskEntry{Agent: agentName, StepName: stepName, Status: "failed", Reason: reason})
	}
	live.publish()
}

func (live *liveSubTasks) done(row dbmodel.AgentSubTask) {
	live.mu.Lock()
	defer live.mu.Unlock()
	label := ""
	if row.Label != nil {
		label = *row.Label
	}
	if i := live.lastInProgress(row.Agent, row.StepName); i != -1 {
		live.entries[i].Status = "done"
		live.entries[i].Label = label
		live.entries[i].Row = &row
	} else {
		live.entries = append(live.entries, liveSubTaskEntry{Agent: row.Agent, StepName: row.StepName, Label: label, Status: "done", Row: &row})
	}
	live.publish()
}

// turnStream owns everything live about one turn: the sub-task list the
// frontend sees, and which agent/step is currently working (so a caught tool
// failure can be attributed to the right row). It is built once per turn, by
// newRunContext, and reached by every tool through RunContext.Stream — this
// is the one file to read for "how does the UI get sub_tasks".
type turnStream struct {
	chatID uuid.UUID
	live   *liveSubTasks
	agent  string
	step   string
}

func newTurnStream(chatID uuid.UUID) *turnStream {
	return &turnStream{chatID: chatID, live: &liveSubTasks{topic: ChatStreamTopic(chatID)}}
}

// StepStarted announces a step the instant it begins, before its work runs.
func (s *turnStream) StepStarted(agentName, stepName, label string) {
	s.live.started(agentName, stepName, label)
}

// SetCurrent records which agent/step is running its tool-calling loop, so a
// failure caught in between — gracefulTool only knows the failing tool's own
// name — can be attributed to it.
func (s *turnStream) SetCurrent(agentName, stepName string) {
	s.agent, s.step = agentName, stepName
}

// StepFailed marks the currently running step failed.
func (s *turnStream) StepFailed(reason string) {
	s.live.failed(s.agent, s.step, reason)
}

// StepDone publishes a step that finished; SubTaskRecorder.OnRecord is wired
// to it.
func (s *turnStream) StepDone(row dbmodel.AgentSubTask) {
	s.live.done(row)
}

// errToolBudgetExhausted is not a failure to report: the agent used its whole
// tool-call budget without writing a final answer. consume returns it together
// with the partial result so Send can finish the turn instead of failing it.
var errToolBudgetExhausted = errors.New("agent: tool budget exhausted")

// isToolBudgetExhausted reports whether err is Eino's "exceeds max iterations",
// bare or wrapped by the node/run error types.
func isToolBudgetExhausted(err error) bool {
	if err == nil {
		return false
	}
	return errors.Is(err, adk.ErrExceedMaxIterations) ||
		strings.Contains(err.Error(), adk.ErrExceedMaxIterations.Error())
}

type turnOutcome struct {
	reply     strings.Builder
	novaCalls []helper.ToolCallResult
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
func (orch *Orchestrator) consume(ctx context.Context, run *RunContext, events *adk.AsyncIterator[*adk.AgentEvent], keepCheckpoint bool) (OrchestratorResult, error) {
	topic := ChatStreamTopic(run.ChatID)
	var outcome turnOutcome
	for {
		event, ok := events.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			if isToolBudgetExhausted(event.Err) {
				// The cap is a budget, not an error: keep whatever the turn
				// produced and let Send ("dapat gak dapat") finalize it.
				return orch.finish(ctx, run, &outcome, keepCheckpoint), errToolBudgetExhausted
			}
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
	return orch.finish(ctx, run, &outcome, keepCheckpoint), nil
}

// finish turns a completed event loop into the turn's result: first what
// happened to the pause (finishInterrupt), then what the frontend renders
// (finalizeResult).
func (orch *Orchestrator) finish(ctx context.Context, run *RunContext, outcome *turnOutcome, keepCheckpoint bool) OrchestratorResult {
	result := OrchestratorResult{TaskID: run.TaskID, OnChainTaskID: run.OnChainTaskID, Reply: outcome.reply.String()}
	orch.finishInterrupt(ctx, run, outcome.interrupt, keepCheckpoint, &result)
	orch.finalizeResult(ctx, run, outcome, &result)
	return result
}

// finishInterrupt records the pause the turn ended on, or clears the chat's
// checkpoint when a finished turn is over. It is the only writer of the
// interrupt marker.
func (orch *Orchestrator) finishInterrupt(ctx context.Context, run *RunContext, interrupt *interruptOutcome, keepCheckpoint bool, result *OrchestratorResult) {
	if interrupt != nil {
		orch.interrupts().record(ctx, run.ChatID, interrupt.kind, interrupt.id)
		if interrupt.kind == interruptQuestions {
			result.PendingQuestions = interrupt.questions
		}
		// A card needs a sentence above it. The pause carries one; use it
		// only when the model did not already write text this turn.
		if strings.TrimSpace(result.Reply) == "" && interrupt.message != "" {
			result.Reply = interrupt.message
			PublishChatEvent(run.ChatID, "reply_delta", map[string]any{"delta": interrupt.message})
		}
		return
	}
	if !keepCheckpoint {
		orch.interrupts().clear(ctx, run.ChatID)
	}
}

// finalizeResult decides what the frontend renders: plain text, a chart, news,
// or the clarifying-questions card.
func (orch *Orchestrator) finalizeResult(ctx context.Context, run *RunContext, outcome *turnOutcome, result *OrchestratorResult) {
	result.ContentType, result.UIProps = helper.ClassifyReply(result.PendingQuestions, orch.cardOfTask(ctx, run.TaskID), outcome.novaCalls)
	if result.ContentType == "workflow_card" {
		result.UIComponent = "clarifying_questions"
	}
}

func interruptFrom(info *adk.InterruptInfo) *interruptOutcome {
	for _, interruptCtx := range info.InterruptContexts {
		if !interruptCtx.IsRootCause {
			continue
		}
		switch payload := interruptCtx.Info.(type) {
		case agentcore.QuestionsInterrupt:
			return &interruptOutcome{kind: interruptQuestions, id: interruptCtx.ID, questions: payload.Questions, message: payload.Message}
		case *agentcore.QuestionsInterrupt:
			return &interruptOutcome{kind: interruptQuestions, id: interruptCtx.ID, questions: payload.Questions, message: payload.Message}
		case agentcore.ArmInterrupt:
			return &interruptOutcome{kind: interruptArm, id: interruptCtx.ID, message: payload.Message}
		case *agentcore.ArmInterrupt:
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
// tools and keeps Nova's results, which helper.ClassifyReply turns into charts and
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
		outcome.novaCalls = append(outcome.novaCalls, helper.ToolCallResult{ToolName: toolName, Result: message.Content})
	}
	return nil
}
