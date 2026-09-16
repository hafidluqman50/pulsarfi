package repository

import (
	"context"
	"errors"
	"time"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"gorm.io/gorm"
)

type AgentTaskRepository struct {
	DB *gorm.DB
}

func (r *AgentTaskRepository) FindByID(ctx context.Context, id int64) (model.AgentTask, bool, error) {
	var task model.AgentTask
	err := r.DB.WithContext(ctx).First(&task, id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.AgentTask{}, false, nil
	}
	return task, err == nil, err
}

func (r *AgentTaskRepository) FindByWallet(ctx context.Context, walletAddress string) ([]model.AgentTask, error) {
	var tasks []model.AgentTask
	err := r.DB.WithContext(ctx).
		Where("wallet_address = ?", walletAddress).
		Order("created_at DESC").
		Find(&tasks).Error
	return tasks, err
}

// FindByChatMessageIDs finds any Task born from one of this chat's own
// messages — the real link is agent_tasks.source_message_id ->
// agent_chat_messages.id, there is no direct chat_id column on Task.
func (r *AgentTaskRepository) FindByChatMessageIDs(ctx context.Context, messageIDs []int64) (model.AgentTask, bool, error) {
	if len(messageIDs) == 0 {
		return model.AgentTask{}, false, nil
	}
	var task model.AgentTask
	err := r.DB.WithContext(ctx).
		Where("source_message_id IN ?", messageIDs).
		Order("created_at DESC").
		First(&task).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return model.AgentTask{}, false, nil
	}
	return task, err == nil, err
}

type AgentTaskCreateInput struct {
	WalletAddress      string
	SourceMessageID    *int64
	RawPrompt          *string
	IsActionable       bool
	Summary            string
	TriggerDescription *string
}

func (r *AgentTaskRepository) Create(ctx context.Context, input AgentTaskCreateInput) (model.AgentTask, error) {
	status := "pending"
	if !input.IsActionable {
		status = "answered"
	}
	summary := input.Summary
	task := model.AgentTask{
		WalletAddress:      input.WalletAddress,
		SourceMessageID:    input.SourceMessageID,
		RawPrompt:          input.RawPrompt,
		IsActionable:       input.IsActionable,
		Status:             status,
		Summary:            &summary,
		TriggerDescription: input.TriggerDescription,
	}
	return task, r.DB.WithContext(ctx).Create(&task).Error
}

// UpdateDecision patches a Task already committed early (understand-time,
// before every field was known) with the latest settled summary/trigger
// description once the user finishes answering clarifying questions —
// never a second Create, the on-chain Task id and row both stay the same.
// Status flips pending/answered the same way Create's own IsActionable
// branch does, since a Task can go from not-yet-actionable to actionable
// as answers come in.
func (r *AgentTaskRepository) UpdateDecision(ctx context.Context, id int64, summary, triggerDescription string, isActionable bool) error {
	status := "pending"
	if !isActionable {
		status = "answered"
	}
	return r.DB.WithContext(ctx).Model(&model.AgentTask{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"summary":             summary,
			"trigger_description": triggerDescription,
			"is_actionable":       isActionable,
			"status":              status,
		}).Error
}

// SetOnChainTaskID persists the id ArmTask received back from the
// contract's createTask call — the link this row needs before any
// recordSubTasks batch or executeTrade check can target it.
func (r *AgentTaskRepository) SetOnChainTaskID(ctx context.Context, id int64, onChainTaskID int64) error {
	return r.DB.WithContext(ctx).Model(&model.AgentTask{}).
		Where("id = ?", id).
		Update("on_chain_task_id", onChainTaskID).Error
}

// SetArmedAt records the moment GrantTradePermission actually succeeded.
// Called only on that success path — a failed arm attempt must leave this
// null, since "armed" is exactly "the on-chain permission exists".
func (r *AgentTaskRepository) SetArmedAt(ctx context.Context, id int64, armedAt time.Time) error {
	return r.DB.WithContext(ctx).Model(&model.AgentTask{}).
		Where("id = ?", id).
		Update("armed_at", armedAt).Error
}

// SetCancelled is unconditional once the caller has already verified
// ownership/authorization — cancel (disarm) is risk-reducing with no
// on-chain state-machine guard beyond the contract's own Active check, so
// there is no local status precondition to enforce here either. Replaces
// the old Cancel, which only matched status = 'pending' and would never
// fire for an already-armed Task.
func (r *AgentTaskRepository) SetCancelled(ctx context.Context, id int64) error {
	return r.DB.WithContext(ctx).Model(&model.AgentTask{}).
		Where("id = ?", id).
		Update("status", "cancelled").Error
}

// SetPaused clears paused_at on resume rather than leaving it stale — the
// paused banner (agent-task-manager-rebuild.md §5 point 8) shows "paused
// at {{ time }}", which a stale timestamp from a prior pause would
// misreport.
func (r *AgentTaskRepository) SetPaused(ctx context.Context, id int64, paused bool) error {
	updates := map[string]any{"paused": paused}
	if paused {
		updates["paused_at"] = gorm.Expr("NOW()")
	} else {
		updates["paused_at"] = nil
	}
	return r.DB.WithContext(ctx).Model(&model.AgentTask{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *AgentTaskRepository) SetStatus(ctx context.Context, id int64, status string) error {
	return r.DB.WithContext(ctx).Model(&model.AgentTask{}).
		Where("id = ?", id).
		Update("status", status).Error
}

func (r *AgentTaskRepository) SetExecutedAt(ctx context.Context, id int64, executedAt time.Time) error {
	return r.DB.WithContext(ctx).Model(&model.AgentTask{}).
		Where("id = ?", id).
		Update("executed_at", executedAt).Error
}

func (r *AgentTaskRepository) SetArmedWithGuardrails(ctx context.Context, id int64, armedAt time.Time, isRecurring bool, cooldownSec int32, maxPerTrade int64, nextRunAt *time.Time, horizonExpiresAt *time.Time) error {
	updates := map[string]any{
		"armed_at":           armedAt,
		"status":             "armed",
		"is_recurring":       isRecurring,
		"cooldown_sec":       cooldownSec,
		"max_per_trade":      maxPerTrade,
		"next_run_at":        nextRunAt,
		"horizon_expires_at": horizonExpiresAt,
	}
	return r.DB.WithContext(ctx).Model(&model.AgentTask{}).
		Where("id = ?", id).
		Updates(updates).Error
}

func (r *AgentTaskRepository) FindDueRecurringTasks(ctx context.Context, limit int) ([]model.AgentTask, error) {
	var tasks []model.AgentTask
	err := r.DB.WithContext(ctx).
		Where("is_recurring = ? AND status = ? AND paused = ? AND next_run_at <= NOW()", true, "armed", false).
		Order("next_run_at ASC").
		Limit(limit).
		Find(&tasks).Error
	return tasks, err
}

func (r *AgentTaskRepository) FindPendingHorizonTasks(ctx context.Context, threshold time.Duration, limit int) ([]model.AgentTask, error) {
	var tasks []model.AgentTask
	cutoff := time.Now().Add(threshold)
	err := r.DB.WithContext(ctx).
		Where("horizon_expires_at IS NOT NULL AND horizon_notified_at IS NULL AND horizon_expires_at <= ?", cutoff).
		Order("horizon_expires_at ASC").
		Limit(limit).
		Find(&tasks).Error
	return tasks, err
}

func (r *AgentTaskRepository) MarkHorizonNotified(ctx context.Context, id int64) error {
	return r.DB.WithContext(ctx).Model(&model.AgentTask{}).
		Where("id = ?", id).
		Update("horizon_notified_at", gorm.Expr("NOW()")).Error
}

func (r *AgentTaskRepository) UpdateNextRunAt(ctx context.Context, id int64, nextRunAt *time.Time) error {
	return r.DB.WithContext(ctx).Model(&model.AgentTask{}).
		Where("id = ?", id).
		Update("next_run_at", nextRunAt).Error
}

func (r *AgentTaskRepository) SetExitPolicy(ctx context.Context, id int64, policy string, status string) error {
	updates := map[string]any{
		"exit_policy": policy,
	}
	if status != "" {
		updates["status"] = status
	}
	return r.DB.WithContext(ctx).Model(&model.AgentTask{}).
		Where("id = ?", id).
		Updates(updates).Error
}

