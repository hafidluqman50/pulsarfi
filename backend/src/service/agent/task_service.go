package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/google/uuid"
	"github.com/horizonlabs/pulsarfi-backend/src/contracts"
	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
)

var (
	ErrWalletMismatch    = errors.New("agent: authenticated wallet does not own this task")
	ErrTaskNotFound      = errors.New("agent: task not found")
	ErrTaskNotActionable = errors.New("agent: task is not actionable")
	ErrTaskNotArmed      = errors.New("agent: task has not been armed yet")
	ErrNothingToRetry    = errors.New("agent: no failed message to retry")
)

type AgentContractClient interface {
	CreateTask(ctx context.Context, owner string, isActionable bool, summary string, promptHash [32]byte) (onChainTaskID uint64, err error)
	GrantTradePermission(ctx context.Context, onChainTaskID uint64, totalBudget string, duration time.Duration, maxAmountPerTrade string, cooldownInterval uint32) error
	RecordSubTasks(ctx context.Context, onChainTaskID uint64, rows []model.AgentSubTask) (onChainSubTaskIDs []uint64, txHash string, err error)
	CancelTask(ctx context.Context, onChainTaskID uint64) error
	TradePermissionRemaining(ctx context.Context, onChainTaskID uint) (string, error)
}

// TaskService is the Task-layer surface HTTP handlers call. It is a thin
// signal/authorization layer only — it never builds an LLM prompt or
// invokes an agent directly. Actually executing a trade (ExecuteTask) does
// nothing but verify authorization/arm state and trigger
// Orchestrator.ResumeExecute, which owns all of that
// (docs/plans/quasar-clean-routing-scalp-refactor.md §2/§3.1).
type TaskService struct {
	Tasks        *repository.AgentTaskRepository
	SubTasks     *repository.AgentSubTaskRepository
	Trades       *repository.AgentTradeRepository
	ChatMessages *repository.AgentChatMessageRepository
	Chats        *repository.AgentChatRepository
	Chain        AgentContractClient
	Executor     adk.Agent
	Orchestrator *Orchestrator
}

type WorkflowCard struct {
	TaskID      int64           `json:"task_id,omitempty"`
	Reply       string          `json:"reply"`
	ContentType string          `json:"content_type"`
	UIComponent *string         `json:"ui_component,omitempty"`
	UIProps     json.RawMessage `json:"ui_props,omitempty"`
}

func (s *TaskService) ListTasks(ctx context.Context, walletAddress string) ([]model.AgentTask, error) {
	return s.Tasks.FindByWallet(ctx, strings.ToLower(walletAddress))
}

// activityFeedLimit caps the Activity Log to the most recent steps across
// every one of the wallet's own Tasks — a live feed, not a paginated
// archive; the full chain for any one Task is still available in full via
// GetReasoningChain.
const activityFeedLimit = 200

func (s *TaskService) GetActivity(ctx context.Context, walletAddress string) ([]model.AgentSubTask, error) {
	return s.SubTasks.FindByOwnerWallet(ctx, strings.ToLower(walletAddress), activityFeedLimit)
}

func (s *TaskService) GetReasoningChain(ctx context.Context, taskID int64) ([]model.AgentSubTask, error) {
	_, found, err := s.Tasks.FindByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrTaskNotFound
	}
	return s.SubTasks.FindByTaskID(ctx, taskID)
}

func (s *TaskService) GetTrades(ctx context.Context, taskID int64, walletAddress string) ([]model.AgentTrade, error) {
	task, found, err := s.Tasks.FindByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrTaskNotFound
	}
	if !strings.EqualFold(task.WalletAddress, walletAddress) {
		return nil, ErrWalletMismatch
	}
	return s.Trades.FindByTaskID(ctx, taskID)
}

func (s *TaskService) recordSubTasksBatch(ctx context.Context, onChainTaskID int64, rows []model.AgentSubTask) error {
	if len(rows) == 0 {
		return nil
	}
	_, txHash, err := s.Chain.RecordSubTasks(ctx, uint64(onChainTaskID), rows)
	if err != nil {
		return err
	}
	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	return s.SubTasks.MarkRecordedOnChain(ctx, ids, txHash)
}

type ArmTaskInput struct {
	TotalBudget       string `json:"total_budget"`
	DurationSec       int64  `json:"duration_sec"`
	TokenAddress      string `json:"token_address"`
	MaxAmountPerTrade string `json:"max_amount_per_trade,omitempty"`
	CooldownInterval  uint32 `json:"cooldown_interval,omitempty"`
	IsRecurring       bool   `json:"is_recurring,omitempty"`
}

type ArmTaskResult struct {
	OnChainTaskID int64  `json:"on_chain_task_id"`
	TokenAddress  string `json:"token_address,omitempty"`
	TotalBudget   string `json:"total_budget,omitempty"`
}

// ArmTask is the one call that moves a Task on-chain into an executable
// state: grantTradePermission only (createTask already happened the instant
// Quasar committed the Task — see commitTask in
// orchestrator_workflow_service.go). Calling CreateTask again here would
// open a second, orphaned on-chain Task and silently rebind Postgres to it.
func (s *TaskService) ArmTask(ctx context.Context, taskID int64, wallet string, input ArmTaskInput) (ArmTaskResult, error) {
	task, found, err := s.Tasks.FindByID(ctx, taskID)
	if err != nil {
		return ArmTaskResult{}, err
	}
	if !found {
		return ArmTaskResult{}, ErrTaskNotFound
	}
	if !strings.EqualFold(task.WalletAddress, wallet) {
		return ArmTaskResult{}, ErrWalletMismatch
	}
	if task.OnChainTaskID == nil {
		return ArmTaskResult{}, fmt.Errorf("agent: task %d has no on-chain id — every Task must already be created on-chain by the time it reaches Arm, this should never happen", taskID)
	}
	onChainTaskID := uint64(*task.OnChainTaskID)

	if task.IsActionable {
		if err := s.grantAndPersistArm(ctx, taskID, onChainTaskID, input); err != nil {
			return ArmTaskResult{}, err
		}
		if err := s.recordArmed(ctx, task, taskID, onChainTaskID, input.TotalBudget); err != nil {
			slog.ErrorContext(ctx, "agent: record armed sub task failed", "task_id", taskID, "error", err)
		}
	}

	if err := s.recordPendingSubTasks(ctx, taskID, onChainTaskID); err != nil {
		slog.ErrorContext(ctx, "agent: catch-up recordSubTasks failed, leaving for retry", "task_id", taskID, "error", err)
	}

	return ArmTaskResult{OnChainTaskID: int64(onChainTaskID), TokenAddress: input.TokenAddress, TotalBudget: input.TotalBudget}, nil
}

func (s *TaskService) grantAndPersistArm(ctx context.Context, taskID int64, onChainTaskID uint64, input ArmTaskInput) error {
	duration := time.Duration(input.DurationSec) * time.Second
	if err := s.Chain.GrantTradePermission(ctx, onChainTaskID, input.TotalBudget, duration, input.MaxAmountPerTrade, input.CooldownInterval); err != nil {
		return fmt.Errorf("agent: on-chain grantTradePermission: %w", err)
	}

	// Only here, only after the chain call actually succeeded — "armed"
	// means exactly "the on-chain permission exists".
	now := time.Now()
	var nextRunAt *time.Time
	if input.IsRecurring && input.CooldownInterval > 0 {
		t := now.Add(time.Duration(input.CooldownInterval) * time.Second)
		nextRunAt = &t
	}
	var horizonExpiresAt *time.Time
	if input.DurationSec > 0 {
		t := now.Add(duration)
		horizonExpiresAt = &t
	}
	var maxPerTrade int64
	if input.MaxAmountPerTrade != "" {
		if m, ok := new(big.Int).SetString(input.MaxAmountPerTrade, 10); ok {
			maxPerTrade = m.Int64()
		}
	}

	if err := s.Tasks.SetArmedWithGuardrails(ctx, taskID, now, input.IsRecurring, int32(input.CooldownInterval), maxPerTrade, nextRunAt, horizonExpiresAt); err != nil {
		return fmt.Errorf("agent: permission granted on-chain for task %d but persisting armed guardrails failed: %w", taskID, err)
	}
	return nil
}

// recordArmed writes the "armed" Sub Task — the last link in the
// understand -> route -> analyze -> await_confirmation -> armed -> execute
// chain — once GrantTradePermission has actually succeeded on-chain.
func (s *TaskService) recordArmed(ctx context.Context, task model.AgentTask, taskID int64, onChainTaskID uint64, totalBudget string) error {
	summary := ""
	if task.Summary != nil {
		summary = *task.Summary
	}
	recorder, err := NewSubTaskRecorder(ctx, s.SubTasks, s.Chain, onChainTaskID, taskID, summary, task.WalletAddress)
	if err != nil {
		return fmt.Errorf("agent: build subtask recorder for armed step: %w", err)
	}
	_, err = recorder.Record(ctx, "supervisor", "armed", "done", fmt.Sprintf("Armed on-chain with budget %s.", totalBudget), "", nil)
	return err
}

func (s *TaskService) recordPendingSubTasks(ctx context.Context, taskID int64, onChainTaskID uint64) error {
	allRows, err := s.SubTasks.FindByTaskID(ctx, taskID)
	if err != nil {
		return err
	}
	pending := make([]model.AgentSubTask, 0, len(allRows))
	for _, row := range allRows {
		if !row.RecordedOnChain {
			pending = append(pending, row)
		}
	}
	return s.recordSubTasksBatch(ctx, int64(onChainTaskID), pending)
}

func (s *TaskService) DisarmTask(ctx context.Context, taskID int64, wallet string) error {
	task, found, err := s.Tasks.FindByID(ctx, taskID)
	if err != nil {
		return err
	}
	if !found {
		return ErrTaskNotFound
	}
	if !strings.EqualFold(task.WalletAddress, wallet) {
		return ErrWalletMismatch
	}
	if task.OnChainTaskID == nil {
		return ErrTaskNotArmed
	}
	if err := s.Chain.CancelTask(ctx, uint64(*task.OnChainTaskID)); err != nil {
		return fmt.Errorf("agent: on-chain cancelTask: %w", err)
	}
	return s.Tasks.SetCancelled(ctx, taskID)
}

func (s *TaskService) PauseTask(ctx context.Context, taskID int64, wallet string) error {
	task, found, err := s.Tasks.FindByID(ctx, taskID)
	if err != nil {
		return err
	}
	if !found {
		return ErrTaskNotFound
	}
	if !strings.EqualFold(task.WalletAddress, wallet) {
		return ErrWalletMismatch
	}
	return s.Tasks.SetPaused(ctx, taskID, true)
}

func (s *TaskService) ResumeTask(ctx context.Context, taskID int64, wallet string) error {
	task, found, err := s.Tasks.FindByID(ctx, taskID)
	if err != nil {
		return err
	}
	if !found {
		return ErrTaskNotFound
	}
	if !strings.EqualFold(task.WalletAddress, wallet) {
		return ErrWalletMismatch
	}
	return s.Tasks.SetPaused(ctx, taskID, false)
}

type ExecuteTaskResult struct {
	TaskID int64              `json:"task_id"`
	Status string             `json:"status"`
	Reply  string             `json:"reply"`
	Trades []model.AgentTrade `json:"trades,omitempty"`
}

// ExecuteTask authorizes and triggers execution — nothing else. All LLM
// orchestration (reading Nova's verdict, deciding whether Comet fires,
// sizing, submit_trade) happens inside the graph via
// Orchestrator.ResumeExecute; this function never builds a prompt or calls
// an agent itself (docs/plans/quasar-clean-routing-scalp-refactor.md §2).
func (s *TaskService) ExecuteTask(ctx context.Context, taskID int64, wallet string) (ExecuteTaskResult, error) {
	task, found, err := s.Tasks.FindByID(ctx, taskID)
	if err != nil {
		return ExecuteTaskResult{}, err
	}
	if !found {
		return ExecuteTaskResult{}, ErrTaskNotFound
	}
	if !strings.EqualFold(task.WalletAddress, wallet) {
		return ExecuteTaskResult{}, ErrWalletMismatch
	}
	if !task.IsActionable {
		return ExecuteTaskResult{}, ErrTaskNotActionable
	}
	if task.ArmedAt == nil || task.OnChainTaskID == nil {
		return ExecuteTaskResult{}, ErrTaskNotArmed
	}

	if task.Status == "executed" {
		return s.alreadyExecutedResult(ctx, task)
	}
	if s.Orchestrator == nil {
		return ExecuteTaskResult{}, errors.New("agent: orchestrator not initialized")
	}

	tradesBefore, _ := s.Trades.FindByTaskID(ctx, taskID)
	countBefore := len(tradesBefore)

	slog.InfoContext(ctx, "agent: resuming task execution via orchestrator graph", "task_id", taskID, "on_chain_id", *task.OnChainTaskID)

	res, err := s.Orchestrator.ResumeExecute(ctx, taskID, wallet)
	if err != nil {
		s.persistExecutionMessage(ctx, &task, fmt.Sprintf("On-chain execution failed: %s", err.Error()))
		return ExecuteTaskResult{}, fmt.Errorf("agent: orchestrator resume execute: %w", err)
	}

	status, executedTrades := s.reconcileTradeStatus(ctx, taskID, task.Status, countBefore)

	cleanReply := cleanExecutionReply(res.Reply)
	if strings.TrimSpace(cleanReply) != "" {
		s.persistExecutionMessage(ctx, &task, cleanReply)
	}

	return ExecuteTaskResult{TaskID: taskID, Status: status, Reply: cleanReply, Trades: executedTrades}, nil
}

func (s *TaskService) alreadyExecutedResult(ctx context.Context, task model.AgentTask) (ExecuteTaskResult, error) {
	var card *contracts.CardContract
	if task.TriggerDescription != nil && *task.TriggerDescription != "" {
		var triggerData struct {
			Card *contracts.CardContract `json:"card"`
		}
		if err := json.Unmarshal([]byte(*task.TriggerDescription), &triggerData); err == nil {
			card = triggerData.Card
		}
	}

	trades, _ := s.Trades.FindByTaskID(ctx, task.ID)
	reply := "This task was already executed."
	if card != nil && card.Ledger.ExecutedDesc != "" {
		reply = card.Ledger.ExecutedDesc
	}
	return ExecuteTaskResult{TaskID: task.ID, Status: task.Status, Reply: reply, Trades: trades}, nil
}

func (s *TaskService) reconcileTradeStatus(ctx context.Context, taskID int64, currentStatus string, countBefore int) (string, []model.AgentTrade) {
	tradesAfter, err := s.Trades.FindByTaskID(ctx, taskID)
	if err != nil {
		return currentStatus, nil
	}
	if len(tradesAfter) > countBefore {
		_ = s.Tasks.SetStatus(ctx, taskID, "executed")
		_ = s.Tasks.SetExecutedAt(ctx, taskID, time.Now())
		return "executed", tradesAfter[countBefore:]
	}
	if len(tradesAfter) > 0 {
		return "executed", tradesAfter
	}
	return currentStatus, nil
}

func (s *TaskService) persistExecutionMessage(ctx context.Context, task *model.AgentTask, content string) {
	if s.ChatMessages == nil || strings.TrimSpace(content) == "" {
		return
	}
	chatID := s.resolveExecutionChatID(ctx, task)
	if chatID == nil {
		return
	}
	if _, err := s.ChatMessages.Create(ctx, repository.AgentChatMessageCreateInput{
		ChatID:      *chatID,
		Sender:      "supervisor",
		ContentType: "text",
		Content:     content,
		UIRefTaskID: nil, // Do not duplicate PlanCard in chat thread
	}); err != nil {
		slog.ErrorContext(ctx, "agent: persist execution reply to chat", "error", err)
	}
}

func (s *TaskService) resolveExecutionChatID(ctx context.Context, task *model.AgentTask) *uuid.UUID {
	if task.SourceMessageID != nil {
		if sourceMsg, found, err := s.ChatMessages.FindByID(ctx, *task.SourceMessageID); err == nil && found {
			cid := sourceMsg.ChatID
			return &cid
		}
	}
	if s.Chats != nil {
		if chats, err := s.Chats.FindByOwnerWallet(ctx, task.WalletAddress); err == nil && len(chats) > 0 {
			cid := chats[0].ID
			return &cid
		}
	}
	return nil
}

// Evaluate (the scheduler-driven re-check tick for an armed, actionable
// Task) is intentionally not implemented — standing-instruction monitoring
// is still undesigned work, tracked separately, not part of this refactor's
// scalp-only scope.

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

func cleanExecutionReply(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = stripCodeFence(trimmed)

	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		if reply, ok := extractReplyField(trimmed); ok {
			return reply
		}
	}
	return trimmed
}

func stripCodeFence(trimmed string) string {
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) < 2 {
		return trimmed
	}
	if strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
		lines = lines[1 : len(lines)-1]
	} else {
		lines = lines[1:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func extractReplyField(trimmed string) (string, bool) {
	var parsed struct {
		Reasoning   string `json:"reasoning"`
		Reply       string `json:"reply"`
		Message     string `json:"message"`
		Explanation string `json:"explanation"`
		Status      string `json:"status"`
	}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return "", false
	}
	switch {
	case parsed.Reasoning != "":
		return parsed.Reasoning, true
	case parsed.Reply != "":
		return parsed.Reply, true
	case parsed.Message != "":
		return parsed.Message, true
	case parsed.Explanation != "":
		return parsed.Explanation, true
	case parsed.Status != "":
		return fmt.Sprintf("Status eksekusi: %s", parsed.Status), true
	default:
		return "", false
	}
}

const (
	recordSubTasksGracePeriod = 2 * time.Minute
	subTaskRetryPollInterval  = 1 * time.Minute
)

func (s *TaskService) RunSubTaskRetry(ctx context.Context) {
	s.runSubTaskRetryOnce(ctx)

	ticker := time.NewTicker(subTaskRetryPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runSubTaskRetryOnce(ctx)
		}
	}
}

func (s *TaskService) runSubTaskRetryOnce(ctx context.Context) {
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

		_, txHash, err := s.Chain.RecordSubTasks(ctx, uint64(*task.OnChainTaskID), rows)
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

func (s *TaskService) SettleHorizonTask(ctx context.Context, taskID int64, wallet string, policy string) error {
	task, found, err := s.Tasks.FindByID(ctx, taskID)
	if err != nil {
		return err
	}
	if !found {
		return ErrTaskNotFound
	}
	if !strings.EqualFold(task.WalletAddress, wallet) {
		return ErrWalletMismatch
	}
	status := "completed"
	if policy == "leave_open" {
		status = "settled_held"
	}
	return s.Tasks.SetExitPolicy(ctx, taskID, policy, status)
}
