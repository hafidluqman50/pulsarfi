package repository

import (
	"context"
	"time"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"gorm.io/gorm"
)

type AgentSubTaskRepository struct {
	DB *gorm.DB
}

// FindByTaskID returns a Task's full hash chain in step order — what
// GET /agent/tasks/:id/reasoning serves, and what an independent verifier
// re-hashes from the genesis hash forward to check against the on-chain
// reasoningHash.
func (r *AgentSubTaskRepository) FindByTaskID(ctx context.Context, taskID int64) ([]model.AgentSubTask, error) {
	var subTasks []model.AgentSubTask
	err := r.DB.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("step_order ASC").
		Find(&subTasks).Error
	return subTasks, err
}

// LastForTask returns the most recently written row for a Task, i.e. the
// current tip of its hash chain — the next row's prev_decision_hash. Found
// is false only for a Task's very first row, whose prev_decision_hash is
// instead the Task's own genesis hash (see service/agent/hashchain.go).
func (r *AgentSubTaskRepository) LastForTask(ctx context.Context, taskID int64) (model.AgentSubTask, bool, error) {
	var subTask model.AgentSubTask
	err := r.DB.WithContext(ctx).
		Where("task_id = ?", taskID).
		Order("step_order DESC").
		First(&subTask).Error
	if err == gorm.ErrRecordNotFound {
		return model.AgentSubTask{}, false, nil
	}
	return subTask, err == nil, err
}

func (r *AgentSubTaskRepository) Create(ctx context.Context, subTask model.AgentSubTask) (model.AgentSubTask, error) {
	return subTask, r.DB.WithContext(ctx).Create(&subTask).Error
}

// FindUnconfirmed returns rows still recorded_on_chain = false, past a
// grace period, whose parent Task is already armed (on_chain_task_id is
// set) — a not-yet-armed Task's rows are *supposed* to stay unconfirmed
// (there is no on-chain Task to batch them against yet), that is not a
// failure to retry. What subtask_retry_service.go scans.
func (r *AgentSubTaskRepository) FindUnconfirmed(ctx context.Context, olderThan time.Time) ([]model.AgentSubTask, error) {
	var subTasks []model.AgentSubTask
	err := r.DB.WithContext(ctx).
		Where("recorded_on_chain = false AND created_at < ? AND task_id IN (?)",
			olderThan, r.DB.Model(&model.AgentTask{}).Select("id").Where("on_chain_task_id IS NOT NULL")).
		Order("task_id ASC, step_order ASC").
		Find(&subTasks).Error
	return subTasks, err
}

// MarkRecordedOnChain flips a confirmed batch's rows in one update — same
// rows, same order as what was submitted, never re-derived.
func (r *AgentSubTaskRepository) MarkRecordedOnChain(ctx context.Context, ids []int64, txHash string) error {
	return r.DB.WithContext(ctx).Model(&model.AgentSubTask{}).
		Where("id IN ?", ids).
		Updates(map[string]any{"recorded_on_chain": true, "on_chain_tx_hash": txHash}).Error
}
