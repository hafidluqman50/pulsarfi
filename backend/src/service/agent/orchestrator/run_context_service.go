package orchestrator

import (
	"context"
	"fmt"
	agentcore "github.com/horizonlabs/pulsarfi-backend/src/app/core/agent"

	"github.com/google/uuid"
	dbmodel "github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/subtask"
)

// newRunContext builds the per-turn context, including the turn's live stream.
func (orch *Orchestrator) newRunContext(chatID uuid.UUID, wallet, rawPrompt string, sourceMessageID *int64) *agentcore.RunContext {
	return &agentcore.RunContext{
		Wallet:          wallet,
		ChatID:          chatID,
		RawPrompt:       rawPrompt,
		SourceMessageID: sourceMessageID,
		Stream:          newTurnStream(chatID),
	}
}

// restoreFromChat gives a resumed questions turn its Task back: the chat's
// most recent Task, found through the messages it was born from.
func (orch *Orchestrator) restoreFromChat(ctx context.Context, run *agentcore.RunContext) error {
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

// restoreRunContext rebuilds what agentcore.RunContext cannot carry across requests
// (it holds a live recorder): the Task identifiers, the recorder continuing
// the same hash chain, and the decision stored on the Task row.
func (orch *Orchestrator) restoreRunContext(ctx context.Context, run *agentcore.RunContext, task dbmodel.AgentTask) error {
	var onChainID uint64
	if task.OnChainTaskID != nil {
		onChainID = uint64(*task.OnChainTaskID)
	}
	summary := ""
	if task.Summary != nil {
		summary = *task.Summary
	}
	recorder, err := subtask.NewSubTaskRecorder(ctx, orch.SubTasks, orch.Chain, onChainID, task.ID, summary, run.Wallet)
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
