package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
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
)

type AgentContractClient interface {
	CreateTask(ctx context.Context, owner string, isActionable bool, summary string, promptHash [32]byte) (onChainTaskID uint64, err error)
	GrantTradePermission(ctx context.Context, onChainTaskID uint64, totalBudget string, duration time.Duration) error
	RecordSubTasks(ctx context.Context, onChainTaskID uint64, rows []model.AgentSubTask) (onChainSubTaskIDs []uint64, txHash string, err error)
	CancelTask(ctx context.Context, onChainTaskID uint64) error
	TradePermissionRemaining(ctx context.Context, onChainTaskID uint) (string, error)
}

type TaskService struct {
	Tasks        *repository.AgentTaskRepository
	SubTasks     *repository.AgentSubTaskRepository
	Trades       *repository.AgentTradeRepository
	ChatMessages *repository.AgentChatMessageRepository
	Chats        *repository.AgentChatRepository
	Chain        AgentContractClient
	Executor     adk.Agent
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

var ErrNothingToRetry = errors.New("agent: no failed message to retry")

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
	TotalBudget  string
	DurationSec  int64
	TokenAddress string
}

type ArmTaskResult struct {
	OnChainTaskID int64  `json:"on_chain_task_id"`
	TokenAddress  string `json:"token_address,omitempty"`
	TotalBudget   string `json:"total_budget,omitempty"`
}

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

	// createTask already happened the instant this Task was recognized
	// (orchestrator_service.go's runRoute, docs/plans/agent-orchestration-graph-rebuild.md
	// v2.1) — every Task reaching Arm already has an on-chain id. Calling
	// CreateTask again here (found while implementing agent-trade-execution.md,
	// a leftover from before that rule existed, when this endpoint was the
	// only place createTask ever ran) would open a second, orphaned on-chain
	// Task and silently rebind Postgres to it, disconnecting any Sub Task
	// already recorded against the real one.
	if task.OnChainTaskID == nil {
		return ArmTaskResult{}, fmt.Errorf("agent: task %d has no on-chain id — every Task must already be created on-chain by the time it reaches Arm, this should never happen", taskID)
	}
	onChainTaskID := uint64(*task.OnChainTaskID)

	if task.IsActionable {
		duration := time.Duration(input.DurationSec) * time.Second
		if err := s.Chain.GrantTradePermission(ctx, onChainTaskID, input.TotalBudget, duration); err != nil {
			return ArmTaskResult{}, fmt.Errorf("agent: on-chain grantTradePermission: %w", err)
		}
		// Only here, only after the chain call actually succeeded — a failed
		// arm must leave armed_at null, since "armed" means exactly "the
		// on-chain permission exists" (Defect B, docs/plans/fix-comet-trade-execution-blockers.md).
		if err := s.Tasks.SetArmedAt(ctx, taskID, time.Now()); err != nil {
			return ArmTaskResult{}, fmt.Errorf("agent: permission granted on-chain for task %d but persisting armed_at failed: %w", taskID, err)
		}
	}

	allRows, err := s.SubTasks.FindByTaskID(ctx, taskID)
	if err != nil {
		return ArmTaskResult{}, err
	}
	pending := make([]model.AgentSubTask, 0, len(allRows))
	for _, row := range allRows {
		if !row.RecordedOnChain {
			pending = append(pending, row)
		}
	}
	if err := s.recordSubTasksBatch(ctx, int64(onChainTaskID), pending); err != nil {
		slog.ErrorContext(ctx, "agent: catch-up recordSubTasks failed, leaving for retry", "task_id", taskID, "error", err)
	}

	return ArmTaskResult{OnChainTaskID: int64(onChainTaskID), TokenAddress: input.TokenAddress, TotalBudget: input.TotalBudget}, nil
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
	summary := ""
	if task.Summary != nil {
		summary = *task.Summary
	}
	prompt := ""
	if task.RawPrompt != nil {
		prompt = *task.RawPrompt
	}

	var card *contracts.CardContract
	if task.TriggerDescription != nil && *task.TriggerDescription != "" {
		var triggerData struct {
			Card *contracts.CardContract `json:"card"`
		}
		if err := json.Unmarshal([]byte(*task.TriggerDescription), &triggerData); err == nil && triggerData.Card != nil {
			card = triggerData.Card
		}
	}

	if task.Status == "executed" {
		trades, _ := s.Trades.FindByTaskID(ctx, taskID)
		reply := "This task was already executed."
		if card != nil && card.Ledger.ExecutedDesc != "" {
			reply = card.Ledger.ExecutedDesc
		}
		return ExecuteTaskResult{
			TaskID: taskID,
			Status: task.Status,
			Reply:  reply,
			Trades: trades,
		}, nil
	}
	if s.Executor == nil {
		return ExecuteTaskResult{}, errors.New("agent: executor agent not initialized")
	}

	onChainTaskID := uint64(*task.OnChainTaskID)

	// Check if on-chain budget permission is already exhausted before firing a doomed transaction
	if remaining, err := s.Chain.TradePermissionRemaining(ctx, uint(onChainTaskID)); err == nil && remaining == "0" {
		trades, _ := s.Trades.FindByTaskID(ctx, taskID)
		if len(trades) > 0 {
			_ = s.Tasks.SetStatus(ctx, taskID, "executed")
			reply := "Task trade order was executed on-chain."
			if card != nil && card.Footnotes.ExecutedSuccess != "" {
				reply = card.Footnotes.ExecutedSuccess
			}
			return ExecuteTaskResult{
				TaskID: taskID,
				Status: "executed",
				Reply:  reply,
				Trades: trades,
			}, nil
		}
	}

	recorder, err := NewSubTaskRecorder(ctx, s.SubTasks, s.Chain, onChainTaskID, task.ID, summary, task.WalletAddress)
	if err != nil {
		return ExecuteTaskResult{}, fmt.Errorf("agent: build recorder: %w", err)
	}

	runCtx := &RunContext{
		OnChainTaskID: task.OnChainTaskID,
		Wallet:        task.WalletAddress,
		Recorder:      recorder,
	}

	subtasks, _ := s.SubTasks.FindByTaskID(ctx, taskID)
	var novaFindings strings.Builder
	for _, st := range subtasks {
		if strings.EqualFold(st.Agent, "analyzer") {
			if st.Reasoning != "" {
				novaFindings.WriteString(st.Reasoning)
				novaFindings.WriteString("\n")
			}
			if st.Output != nil && *st.Output != "" {
				novaFindings.WriteString(*st.Output)
				novaFindings.WriteString("\n")
			}
		}
	}

	var request string
	if novaFindings.Len() > 0 {
		request = fmt.Sprintf("User instruction:\n%s\n\nNova's market findings:\n%s\n\nTask summary: %s\nThe task has been armed and approved on-chain. Based on Nova's findings above, check spot price and IDRX balance, and execute the trade on-chain using submit_trade.\n\n[CRITICAL LANGUAGE MANDATE]: Your entire final reply, explanation, and reasoning MUST 100%% match the language of the User instruction above (e.g. Indonesian if the user writes in Indonesian, English if in English, Turkish if in Turkish, Javanese if in Javanese, Banjar if in Banjar, etc.). Never use any other language!", prompt, novaFindings.String(), summary)
	} else {
		request = fmt.Sprintf("User instruction:\n%s\n\nTask summary: %s\nThe task has been armed and approved on-chain. Check spot price and IDRX balance, and execute the trade on-chain using submit_trade.\n\n[CRITICAL LANGUAGE MANDATE]: Your entire final reply, explanation, and reasoning MUST 100%% match the language of the User instruction above (e.g. Indonesian if the user writes in Indonesian, English if in English, Turkish if in Turkish, Javanese if in Javanese, Banjar if in Banjar, etc.). Never use any other language!", prompt, summary)
	}

	tradesBefore, _ := s.Trades.FindByTaskID(ctx, taskID)
	countBefore := len(tradesBefore)

	tipBefore := recorder.TerminalHash()
	slog.InfoContext(ctx, "agent: executing task with comet", "task_id", taskID, "on_chain_id", onChainTaskID)

	reply, _, err := runRoleAgent(WithRunContext(ctx, runCtx), s.Executor, request, nil, nil)
	if err != nil {
		_, _ = recorder.Record(ctx, "executor", "decide", "failed", err.Error(), summary, nil)
		s.persistExecutionMessage(ctx, &task, fmt.Sprintf("Gagal menjalankan eksekusi on-chain: %s", err.Error()))
		return ExecuteTaskResult{}, fmt.Errorf("agent: executor agent run: %w", err)
	}

	cleanReply := cleanExecutionReply(reply)
	if recorder.TerminalHash() == tipBefore {
		if _, err := recorder.Record(ctx, "executor", "decide", "done", cleanReply, summary, map[string]any{"action": "hold"}); err != nil {
			slog.ErrorContext(ctx, "agent: record hold decision", "error", err)
		}
	}

	tradesAfter, err := s.Trades.FindByTaskID(ctx, taskID)
	status := task.Status
	var executedTrades []model.AgentTrade
	if err == nil && len(tradesAfter) > countBefore {
		status = "executed"
		_ = s.Tasks.SetStatus(ctx, taskID, "executed")
		_ = s.Tasks.SetExecutedAt(ctx, taskID, time.Now())
		executedTrades = tradesAfter[countBefore:]
	}

	// Persist Comet's conversational response into agent_chat_messages
	if strings.TrimSpace(cleanReply) != "" {
		s.persistExecutionMessage(ctx, &task, cleanReply)
	}

	return ExecuteTaskResult{
		TaskID: taskID,
		Status: status,
		Reply:  cleanReply,
		Trades: executedTrades,
	}, nil
}

func (s *TaskService) persistExecutionMessage(ctx context.Context, task *model.AgentTask, content string) {
	if s.ChatMessages == nil || strings.TrimSpace(content) == "" {
		return
	}
	var chatID *uuid.UUID
	if task.SourceMessageID != nil {
		if sourceMsg, found, err := s.ChatMessages.FindByID(ctx, *task.SourceMessageID); err == nil && found {
			cid := sourceMsg.ChatID
			chatID = &cid
		}
	}
	if chatID == nil && s.Chats != nil {
		if chats, err := s.Chats.FindByOwnerWallet(ctx, task.WalletAddress); err == nil && len(chats) > 0 {
			cid := chats[0].ID
			chatID = &cid
		}
	}
	if chatID != nil {
		_, err := s.ChatMessages.Create(ctx, repository.AgentChatMessageCreateInput{
			ChatID:      *chatID,
			Sender:      "supervisor",
			ContentType: "text",
			Content:     content,
			UIRefTaskID: nil, // Do not duplicate PlanCard in chat thread
		})
		if err != nil {
			slog.ErrorContext(ctx, "agent: persist execution reply to chat", "error", err)
		}
	}
}

// Evaluate (the scheduler-driven re-check tick for an armed, actionable
// Task) is removed as of docs/plans/agent-orchestration-graph-rebuild.md
// v2.2 — it had zero callers anywhere (no scheduler was ever wired to it),
// and its own approach (re-running Quasar's full route decision from
// scratch on every tick) was already agreed to be the wrong shape for
// standing-instruction monitoring, which per that same plan (§12) should
// start at Comet calling Nova directly, not restart at Quasar. Tracked
// there as still-undesigned work, not lost.

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}

func cleanExecutionReply(raw string) string {
	trimmed := strings.TrimSpace(raw)
	// Remove markdown code fences if wrapped in ```json ... ``` or ``` ... ```
	if strings.HasPrefix(trimmed, "```") {
		lines := strings.Split(trimmed, "\n")
		if len(lines) >= 2 {
			if strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
				lines = lines[1 : len(lines)-1]
			} else {
				lines = lines[1:]
			}
			trimmed = strings.TrimSpace(strings.Join(lines, "\n"))
		}
	}

	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		var parsed struct {
			Reasoning   string `json:"reasoning"`
			Reply       string `json:"reply"`
			Message     string `json:"message"`
			Explanation string `json:"explanation"`
			Status      string `json:"status"`
		}
		if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
			if parsed.Reasoning != "" {
				return parsed.Reasoning
			}
			if parsed.Reply != "" {
				return parsed.Reply
			}
			if parsed.Message != "" {
				return parsed.Message
			}
			if parsed.Explanation != "" {
				return parsed.Explanation
			}
			if parsed.Status != "" {
				return fmt.Sprintf("Status eksekusi: %s", parsed.Status)
			}
		}
	}
	return trimmed
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

