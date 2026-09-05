package agent

import (
	"context"
	"log/slog"
	"time"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
)

const (
	recordSubTasksGracePeriod = 2 * time.Minute
	subTaskRetryPollInterval  = 1 * time.Minute
)

// ChainClient is SubTaskRetryService's own narrow view of the contract
// (ISP) — deliberately not shared with TaskService's broader
// AgentContractClient (task_service.go), which also needs
// CreateTask/GrantTradePermission/CancelTask that this service never
// calls. Any real AgentContractClient implementation satisfies this too.
type ChainClient interface {
	RecordSubTasks(ctx context.Context, onChainTaskID uint64, rows []model.AgentSubTask) (txHash string, err error)
}

// SubTaskRetryService re-submits any agent_sub_tasks batch still
// recorded_on_chain = false past a grace period — the real failure story
// for recordSubTasks: a reverted or dropped transaction never gets
// speculatively marked confirmed, so this is what makes "retrievable by
// anyone, forever" actually true once the chain confirms. Rows belonging
// to a not-yet-armed Task are excluded by
// AgentSubTaskRepository.FindUnconfirmed itself — that is expected state,
// not a failure to retry.
type SubTaskRetryService struct {
	Tasks    *repository.AgentTaskRepository
	SubTasks *repository.AgentSubTaskRepository
	Chain    ChainClient
}

// Run loops forever on its own ticker until ctx is cancelled — same
// pattern as indexer.TransferIndexerService.Run, launched once via
// `go svcs.AgentSubTaskRetry.Run(ctx)` at bootstrap.
func (s *SubTaskRetryService) Run(ctx context.Context) {
	s.runOnce(ctx)

	ticker := time.NewTicker(subTaskRetryPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runOnce(ctx)
		}
	}
}

func (s *SubTaskRetryService) runOnce(ctx context.Context) {
	cutoff := time.Now().Add(-recordSubTasksGracePeriod)
	unconfirmed, err := s.SubTasks.FindUnconfirmed(ctx, cutoff)
	if err != nil {
		slog.ErrorContext(ctx, "subtask_retry: failed to load unconfirmed rows", "error", err)
		return
	}
	if len(unconfirmed) == 0 {
		return
	}

	rowsByTask := map[int64][]model.AgentSubTask{}
	for _, row := range unconfirmed {
		rowsByTask[row.TaskID] = append(rowsByTask[row.TaskID], row)
	}

	for taskID, rows := range rowsByTask {
		task, found, err := s.Tasks.FindByID(ctx, taskID)
		if err != nil || !found || task.OnChainTaskID == nil {
			slog.ErrorContext(ctx, "subtask_retry: task missing or not armed, skipping", "task_id", taskID, "error", err)
			continue
		}

		txHash, err := s.Chain.RecordSubTasks(ctx, uint64(*task.OnChainTaskID), rows)
		if err != nil {
			slog.ErrorContext(ctx, "subtask_retry: resubmit failed, will retry next tick", "task_id", taskID, "error", err)
			continue
		}

		ids := make([]int64, len(rows))
		for i, row := range rows {
			ids[i] = row.ID
		}
		if err := s.SubTasks.MarkRecordedOnChain(ctx, ids, txHash); err != nil {
			slog.ErrorContext(ctx, "subtask_retry: confirmed on-chain but failed to mark locally", "task_id", taskID, "tx_hash", txHash, "error", err)
		}
	}
}
