package agent

import (
	"context"
	"fmt"
	agentcore "github.com/horizonlabs/pulsarfi-backend/src/app/core/agent"

	"github.com/google/uuid"
	dbmodel "github.com/horizonlabs/pulsarfi-backend/src/model"
)

// RunContext is threaded via WithRunContext into every tool of the turn —
// Quasar's own (open_task, ask_user, await_arm) and the nested ones of Nova
// and Comet (get_portfolio_snapshot, submit_trade). It is the only channel
// that reaches code the agent runtime treats as a black box.
//
// It carries exactly three things: identity (who, which chat, which prompt),
// persistence (the Task being worked and its Sub Task chain), and the turn's
// live Stream. Everything a tool needs to talk to the UI goes through Stream,
// never through loose callbacks here. It is rebuilt for every request and
// never checkpointed: a resumed turn gets its Task back from the database
// (see restoreRunContext).
type RunContext struct {
	// identity
	Wallet          string
	ChatID          uuid.UUID
	RawPrompt       string
	SourceMessageID *int64

	// persistence
	OnChainTaskID *int64
	TaskID        int64
	Recorder      *SubTaskRecorder
	Decision      agentcore.RouteDecision

	// live
	Stream *turnStream
}

type runContextKey struct{}

func WithRunContext(ctx context.Context, run *RunContext) context.Context {
	return context.WithValue(ctx, runContextKey{}, run)
}

func RunContextFrom(ctx context.Context) (*RunContext, bool) {
	run, ok := ctx.Value(runContextKey{}).(*RunContext)
	return run, ok
}

// newRunContext builds the per-turn context, including the turn's live stream.
func (orch *Orchestrator) newRunContext(chatID uuid.UUID, wallet, rawPrompt string, sourceMessageID *int64) *RunContext {
	return &RunContext{
		Wallet:          wallet,
		ChatID:          chatID,
		RawPrompt:       rawPrompt,
		SourceMessageID: sourceMessageID,
		Stream:          newTurnStream(chatID),
	}
}

// restoreFromChat gives a resumed questions turn its Task back: the chat's
// most recent Task, found through the messages it was born from.
func (orch *Orchestrator) restoreFromChat(ctx context.Context, run *RunContext) error {
	messages, err := orch.ChatMessages.FindByChatID(ctx, run.ChatID)
	if err != nil {
		return fmt.Errorf("orchestrator: load chat messages: %w", err)
	}
	messageIDs := make([]int64, len(messages))
	for i, message := range messages {
		messageIDs[i] = message.ID
	}
	task, found, err := orch.Tasks.FindByChatMessageIDs(ctx, messageIDs)
	if err != nil {
		return fmt.Errorf("orchestrator: find chat task: %w", err)
	}
	if !found {
		return fmt.Errorf("%w: chat %s is paused but has no task", agentcore.ErrNoOpenTask, run.ChatID)
	}
	return orch.restoreRunContext(ctx, run, task)
}

// restoreRunContext rebuilds what RunContext cannot carry across requests
// (it holds a live recorder): the Task identifiers, the recorder continuing
// the same hash chain, and the decision stored on the Task row.
func (orch *Orchestrator) restoreRunContext(ctx context.Context, run *RunContext, task dbmodel.AgentTask) error {
	var onChainID uint64
	if task.OnChainTaskID != nil {
		onChainID = uint64(*task.OnChainTaskID)
	}
	summary := ""
	if task.Summary != nil {
		summary = *task.Summary
	}
	recorder, err := NewSubTaskRecorder(ctx, orch.SubTasks, orch.Chain, onChainID, task.ID, summary, run.Wallet)
	if err != nil {
		return fmt.Errorf("orchestrator: rebuild subtask recorder: %w", err)
	}
	recorder.OnRecord = run.Stream.StepDone
	run.TaskID = task.ID
	run.OnChainTaskID = task.OnChainTaskID
	run.Recorder = recorder
	run.Decision = decisionFromTask(task)
	return nil
}
