package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
)

// SubTaskRecorder appends one hash-chained agent_sub_tasks row per step a
// node takes while working a Task — see
// docs/plans/agent-role-architecture.md §3a/§7. One recorder is scoped to
// exactly one Task's run; Supervisor, Analyzer, and Executor all record
// through the same recorder instance so every row in a run shares one
// continuous chain.
type SubTaskRecorder struct {
	subTasks  *repository.AgentSubTaskRepository
	taskID    int64
	nextOrder int
	prevHash  string
	rows      []model.AgentSubTask // every row this instance has written, in order
	// OnRecord, if set, fires synchronously right after a row is persisted
	// — the hook HandleChatMessage uses to stream each Sub Task out over
	// SSE the moment it's created, instead of the frontend only learning
	// about the whole run once it fully finishes.
	OnRecord func(model.AgentSubTask)
}

// NewSubTaskRecorder loads the current tip of taskID's chain, or computes
// its genesis hash if this Task has never recorded a step before — so a
// recorder built fresh for a re-evaluation tick continues the same chain
// instead of starting a new, disconnected one.
func NewSubTaskRecorder(ctx context.Context, subTasks *repository.AgentSubTaskRepository, taskID int64, triggerDescription, owner string) (*SubTaskRecorder, error) {
	last, found, err := subTasks.LastForTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("sub_task_recorder: load chain tip: %w", err)
	}
	if found {
		return &SubTaskRecorder{subTasks: subTasks, taskID: taskID, nextOrder: last.StepOrder + 1, prevHash: last.DecisionHash}, nil
	}
	return &SubTaskRecorder{subTasks: subTasks, taskID: taskID, nextOrder: 1, prevHash: GenesisHash(taskID, triggerDescription, owner)}, nil
}

// Record appends one link to the chain. output is marshaled as given; pass
// nil when a step has nothing structured to attach beyond its reasoning.
func (r *SubTaskRecorder) Record(ctx context.Context, agentName, stepName, status, reasoning string, output any) (model.AgentSubTask, error) {
	var outputJSON []byte
	if output != nil {
		var err error
		outputJSON, err = json.Marshal(output)
		if err != nil {
			return model.AgentSubTask{}, fmt.Errorf("sub_task_recorder: marshal output: %w", err)
		}
	}

	decisionHash := DecisionHash(agentName, stepName, reasoning, outputJSON, r.prevHash)

	var outputStr *string
	if outputJSON != nil {
		s := string(outputJSON)
		outputStr = &s
	}

	row, err := r.subTasks.Create(ctx, model.AgentSubTask{
		TaskID:           r.taskID,
		StepOrder:        r.nextOrder,
		Agent:            agentName,
		StepName:         stepName,
		Status:           status,
		Reasoning:        reasoning,
		Output:           outputStr,
		PrevDecisionHash: r.prevHash,
		DecisionHash:     decisionHash,
	})
	if err != nil {
		return model.AgentSubTask{}, fmt.Errorf("sub_task_recorder: persist row: %w", err)
	}

	r.nextOrder++
	r.prevHash = decisionHash
	r.rows = append(r.rows, row)
	if r.OnRecord != nil {
		r.OnRecord(row)
	}
	return row, nil
}

// TerminalHash is the chain's current tip — the value submitted on-chain as
// reasoningHash once a run reaches an execute/skip step (see
// docs/plans/agent-role-architecture.md §7).
func (r *SubTaskRecorder) TerminalHash() string {
	return r.prevHash
}

// RowCount is how many rows this instance has written so far in its own
// lifetime (not the Task's lifetime total) — a caller captures this before
// a run starts so RowsSince can report only what that run actually wrote.
func (r *SubTaskRecorder) RowCount() int {
	return len(r.rows)
}

// RowsSince returns every row recorded from index n onward — what
// HandleChatMessage/Evaluate (docs/plans/agent-task-manager-code-implementation.md
// §7.B/§7.D) batch into one on-chain recordSubTasks call once their run
// finishes.
func (r *SubTaskRecorder) RowsSince(n int) []model.AgentSubTask {
	return r.rows[n:]
}
