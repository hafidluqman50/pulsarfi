package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/horizonlabs/pulsarfi-backend/src/contracts"
	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service/realtime"
)

// SubTaskRecorder appends one hash-chained agent_sub_tasks row per step a
// node takes while working a Task — see
// docs/plans/agent-role-architecture.md §3a/§7. One recorder is scoped to
// exactly one Task's run; Supervisor, Analyzer, and Executor all record
// through the same recorder instance so every row in a run shares one
// continuous chain.
type SubTaskRecorder struct {
	subTasks      *repository.AgentSubTaskRepository
	chain         ChainSubTaskRecorder
	onChainTaskID uint64
	taskID        int64
	nextOrder     int
	prevHash      string
	// OnRecord, if set, fires synchronously right after a row is persisted
	// — the hook HandleChatMessage uses to stream each Sub Task out over
	// SSE the moment it's created, instead of the frontend only learning
	// about the whole run once it fully finishes.
	OnRecord func(model.AgentSubTask)
}

// ChainSubTaskRecorder is the narrow chain capability SubTaskRecorder
// itself needs — a subset of OrchestratorChainClient (ISP): this package
// never needs CreateTask, only the ability to record one Sub Task on-chain
// and learn its real, contract-assigned id.
type ChainSubTaskRecorder interface {
	RecordSubTasks(ctx context.Context, onChainTaskID uint64, rows []model.AgentSubTask) (onChainSubTaskIDs []uint64, txHash string, err error)
}

// NewSubTaskRecorder loads the current tip of taskID's chain, or computes
// its genesis hash if this Task has never recorded a step before — so a
// recorder built fresh for a re-evaluation tick continues the same chain
// instead of starting a new, disconnected one.
func NewSubTaskRecorder(ctx context.Context, subTasks *repository.AgentSubTaskRepository, chain ChainSubTaskRecorder, onChainTaskID uint64, taskID int64, triggerDescription, owner string) (*SubTaskRecorder, error) {
	last, found, err := subTasks.LastForTask(ctx, taskID)
	if err != nil {
		return nil, fmt.Errorf("sub_task_recorder: load chain tip: %w", err)
	}
	if found {
		return &SubTaskRecorder{subTasks: subTasks, chain: chain, onChainTaskID: onChainTaskID, taskID: taskID, nextOrder: last.StepOrder + 1, prevHash: last.DecisionHash}, nil
	}
	return &SubTaskRecorder{subTasks: subTasks, chain: chain, onChainTaskID: onChainTaskID, taskID: taskID, nextOrder: 1, prevHash: contracts.GenesisHash(taskID, triggerDescription, owner)}, nil
}

// Record appends one link to the chain. output is marshaled as given; pass
// nil when a step has nothing structured to attach beyond its reasoning.
// label is a short, human-readable description of this step in whatever
// language the conversation is in (e.g. "Meneruskan ke Analyzer untuk cek
// tren VKTR") — display only, pass "" when the caller has nothing better
// than stepName itself to show (the UI falls back to a humanized stepName
// in that case). label is never part of the hash: stepName is what
// DecisionHash actually uses, so the chain stays verifiable regardless of
// which language a label happens to be written in.
func (r *SubTaskRecorder) Record(ctx context.Context, agentName, stepName, status, reasoning, label string, output any) (model.AgentSubTask, error) {
	var outputJSON []byte
	if output != nil {
		var err error
		outputJSON, err = json.Marshal(output)
		if err != nil {
			return model.AgentSubTask{}, fmt.Errorf("sub_task_recorder: marshal output: %w", err)
		}
	}

	decisionHash := contracts.DecisionHash(agentName, stepName, reasoning, outputJSON, r.prevHash)

	var outputStr *string
	if outputJSON != nil {
		s := string(outputJSON)
		outputStr = &s
	}

	var labelPtr *string
	if label != "" {
		labelPtr = &label
	}

	unsaved := model.AgentSubTask{
		TaskID:           r.taskID,
		StepOrder:        r.nextOrder,
		Agent:            agentName,
		StepName:         stepName,
		Label:            labelPtr,
		Status:           status,
		Reasoning:        reasoning,
		Output:           outputStr,
		PrevDecisionHash: r.prevHash,
		DecisionHash:     decisionHash,
	}

	// On-chain first, same absolute rule as createTask (v2.1) — applies to
	// every Sub Task, not just trade-related ones
	// (docs/plans/agent-orchestration-graph-rebuild.md v2.14/v2.15). If this
	// fails, the whole turn aborts right here: no Postgres row, nothing
	// partially created. A step existing off-chain only, even transiently,
	// is not a degraded state to tolerate — the hash chain's own value as a
	// commitment made *before* the next (possibly irreversible) step
	// depends on this never happening.
	onChainIDs, txHash, chainErr := r.chain.RecordSubTasks(ctx, r.onChainTaskID, []model.AgentSubTask{unsaved})
	if chainErr != nil {
		return model.AgentSubTask{}, fmt.Errorf("sub_task_recorder: on-chain recordSubTasks failed, aborting (on-chain is the source of truth, no off-chain-only step is allowed): %w", chainErr)
	}
	onChainID := int64(onChainIDs[0])
	unsaved.OnChainSubTaskID = &onChainID
	unsaved.RecordedOnChain = true
	unsaved.OnChainTxHash = &txHash

	row, err := r.subTasks.Create(ctx, unsaved)
	if err != nil {
		return model.AgentSubTask{}, fmt.Errorf("sub_task_recorder: on-chain step %d recorded but Postgres insert failed: %w", onChainID, err)
	}

	r.nextOrder++
	r.prevHash = decisionHash
	if r.OnRecord != nil {
		r.OnRecord(row)
	}
	// Pushed to any WebSocket client watching this Task's reasoning
	// (docs/plans/realtime-websocket-updates.md §3), independent of
	// OnRecord above — OnRecord only fires for the one in-flight HTTP
	// request that triggered this run; this reaches every other client
	// with this Task's detail view open, replacing what used to be a 5s
	// poll.
	realtime.Publish(fmt.Sprintf("agent-task-reasoning:%d", r.taskID), row)
	return row, nil
}

// TerminalHash is the chain's current tip — the value submitted on-chain as
// reasoningHash once a run reaches an execute/skip step (see
// docs/plans/agent-role-architecture.md §7).
func (r *SubTaskRecorder) TerminalHash() string {
	return r.prevHash
}

// TaskID is the Postgres id of the Task this recorder's chain belongs to —
// needed by submit_trade to attach a real agent_trades row to the right
// Task (docs/plans/agent-trade-execution.md).
func (r *SubTaskRecorder) TaskID() int64 {
	return r.taskID
}
