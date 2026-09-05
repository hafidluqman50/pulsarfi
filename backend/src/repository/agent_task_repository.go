package repository

import (
	"context"
	"errors"

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
	WalletAddress   string
	SourceMessageID *int64
	RawPrompt       *string
	IsActionable    bool
	Summary         string
}

func (r *AgentTaskRepository) Create(ctx context.Context, input AgentTaskCreateInput) (model.AgentTask, error) {
	status := "pending"
	if !input.IsActionable {
		status = "answered"
	}
	summary := input.Summary
	task := model.AgentTask{
		WalletAddress:   input.WalletAddress,
		SourceMessageID: input.SourceMessageID,
		RawPrompt:       input.RawPrompt,
		IsActionable:    input.IsActionable,
		Status:          status,
		Summary:         &summary,
	}
	return task, r.DB.WithContext(ctx).Create(&task).Error
}

// SetOnChainTaskID persists the id ArmTask received back from the
// contract's createTask call — the link this row needs before any
// recordSubTasks batch or executeTrade check can target it.
func (r *AgentTaskRepository) SetOnChainTaskID(ctx context.Context, id int64, onChainTaskID int64) error {
	return r.DB.WithContext(ctx).Model(&model.AgentTask{}).
		Where("id = ?", id).
		Update("on_chain_task_id", onChainTaskID).Error
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
