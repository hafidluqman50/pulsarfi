package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"gorm.io/datatypes"
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
	RecordSubTasks(ctx context.Context, onChainTaskID uint64, rows []model.AgentSubTask) (txHash string, err error)
	CancelTask(ctx context.Context, onChainTaskID uint64) error
}

type TaskService struct {
	Tasks        *repository.AgentTaskRepository
	SubTasks     *repository.AgentSubTaskRepository
	Trades       *repository.AgentTradeRepository
	Chats        *repository.AgentChatRepository
	ChatMessages *repository.AgentChatMessageRepository
	Orchestrator *Orchestrator
	Chain        AgentContractClient
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

func (s *TaskService) ListChats(ctx context.Context, walletAddress string) ([]model.AgentChat, error) {
	return s.Chats.FindByOwnerWallet(ctx, strings.ToLower(walletAddress))
}

// activityFeedLimit caps the Activity Log to the most recent steps across
// every one of the wallet's own Tasks — a live feed, not a paginated
// archive; the full chain for any one Task is still available in full via
// GetReasoningChain.
const activityFeedLimit = 200

func (s *TaskService) GetActivity(ctx context.Context, walletAddress string) ([]model.AgentSubTask, error) {
	return s.SubTasks.FindByOwnerWallet(ctx, strings.ToLower(walletAddress), activityFeedLimit)
}

func (s *TaskService) GetChatMessages(ctx context.Context, chatID uuid.UUID, walletAddress string) ([]model.AgentChatMessage, error) {
	chat, found, err := s.Chats.FindByID(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if !found {
		return []model.AgentChatMessage{}, nil
	}
	if !strings.EqualFold(chat.OwnerWallet, walletAddress) {
		return nil, ErrWalletMismatch
	}
	return s.ChatMessages.FindByChatID(ctx, chatID)
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

// QuasarErrorMessage is what the user hears, in-chat, whenever a turn fails
// for any reason — on-chain abort, LLM failure, DB error. Zero fallback
// (docs/plans/agent-orchestration-graph-rebuild.md v2.5) means a failure is
// never hidden or faked, but it must still reach the user as something
// Quasar says, not a bare system string. A static constant, not a second
// LLM call, since the thing that just failed might be the LLM/API itself.
const QuasarErrorMessage = "Waduh, ada kendala pas aku memproses ini. Coba kirim ulang pesannya sebentar lagi ya."

func (s *TaskService) HandleChatMessage(ctx context.Context, chatID uuid.UUID, wallet, message string, events AgentEventCallbacks) (WorkflowCard, error) {
	chat, err := s.Chats.FindOrCreate(ctx, chatID, strings.ToLower(wallet), truncateRunes(message, 50))
	if err != nil {
		return WorkflowCard{}, err
	}
	if !strings.EqualFold(chat.OwnerWallet, wallet) {
		return WorkflowCard{}, ErrWalletMismatch
	}

	existingMessages, err := s.ChatMessages.FindByChatID(ctx, chatID)
	if err != nil {
		return WorkflowCard{}, err
	}

	userMessage, err := s.ChatMessages.Create(ctx, repository.AgentChatMessageCreateInput{
		ChatID: chatID, Sender: "user", ContentType: "text", Content: message,
	})
	if err != nil {
		return WorkflowCard{}, fmt.Errorf("agent: persist user message: %w", err)
	}

	return s.runForMessage(ctx, chatID, userMessage, append(existingMessages, userMessage), strings.ToLower(wallet), events)
}

func (s *TaskService) RetryLastMessage(ctx context.Context, chatID uuid.UUID, wallet string, events AgentEventCallbacks) (WorkflowCard, error) {
	chat, found, err := s.Chats.FindByID(ctx, chatID)
	if err != nil {
		return WorkflowCard{}, err
	}
	if !found {
		return WorkflowCard{}, ErrNothingToRetry
	}
	if !strings.EqualFold(chat.OwnerWallet, wallet) {
		return WorkflowCard{}, ErrWalletMismatch
	}

	existingMessages, err := s.ChatMessages.FindByChatID(ctx, chatID)
	if err != nil {
		return WorkflowCard{}, err
	}
	if len(existingMessages) == 0 {
		return WorkflowCard{}, ErrNothingToRetry
	}
	lastMessage := existingMessages[len(existingMessages)-1]
	if lastMessage.Sender != "user" {
		return WorkflowCard{}, ErrNothingToRetry
	}

	return s.runForMessage(ctx, chatID, lastMessage, existingMessages, strings.ToLower(wallet), events)
}

// runForMessage delegates the whole route/analyze/execute/reply turn to the
// Orchestrator (orchestrator_service.go) — this function is now just the
// chat-message/HTTP-facing glue: build history, call the graph, persist the
// reply. The Orchestrator owns Task creation, hash-chain recording, and the
// on-chain createTask/RecordSubTasks calls itself (see
// docs/plans/agent-orchestration-graph-rebuild.md).
//
// Not carried over from the pre-rebuild version, tracked as open items in
// that same plan, not silently dropped: rebinding to an existing
// needs_input Task (every message now opens a fresh Task), and chart/news
// content_type classification (every reply persists as plain "text" for
// now).
func (s *TaskService) runForMessage(ctx context.Context, chatID uuid.UUID, userMessage model.AgentChatMessage, allMessages []model.AgentChatMessage, wallet string, events AgentEventCallbacks) (WorkflowCard, error) {
	sourceMessageID := userMessage.ID

	history := make([]*schema.Message, len(allMessages))
	for i, m := range allMessages {
		if m.Sender == "user" {
			history[i] = schema.UserMessage(m.Content)
		} else {
			history[i] = schema.AssistantMessage(m.Content, nil)
		}
	}

	result, err := s.Orchestrator.Run(ctx, OrchestratorInput{
		Wallet:              wallet,
		RawPrompt:           userMessage.Content,
		Messages:            history,
		SourceMessageID:     &sourceMessageID,
		AgentEventCallbacks: events,
	})
	if err != nil {
		// Zero fallback (docs/plans/agent-orchestration-graph-rebuild.md v2.5):
		// the failure itself is never hidden or faked, but the user must
		// still hear about it from Quasar, in the chat, not just a generic
		// system string surfaced by the transport layer. Persisted like any
		// other supervisor reply — visible on reload, not just a one-time
		// toast — deliberately a static string, never a second LLM call,
		// since the thing that just failed might be the LLM/API itself.
		if _, persistErr := s.ChatMessages.Create(ctx, repository.AgentChatMessageCreateInput{
			ChatID:      chatID,
			Sender:      "supervisor",
			ContentType: "text",
			Content:     QuasarErrorMessage,
		}); persistErr != nil {
			slog.ErrorContext(ctx, "agent: persist error message failed", "error", persistErr)
		}
		return WorkflowCard{}, fmt.Errorf("agent: orchestrator run failed: %w", err)
	}

	card := WorkflowCard{TaskID: result.TaskID, Reply: result.Reply, ContentType: result.ContentType, UIProps: result.UIProps}

	var uiProps datatypes.JSON
	if len(card.UIProps) > 0 {
		uiProps = datatypes.JSON(card.UIProps)
	}
	var refTaskID *int64
	if result.TaskID != 0 {
		refTaskID = &result.TaskID
	}
	if _, err := s.ChatMessages.Create(ctx, repository.AgentChatMessageCreateInput{
		ChatID:      chatID,
		Sender:      "supervisor",
		ContentType: card.ContentType,
		Content:     card.Reply,
		UIProps:     uiProps,
		UIRefTaskID: refTaskID,
	}); err != nil {
		slog.ErrorContext(ctx, "agent: persist supervisor reply failed", "error", err)
	}

	return card, nil
}

func (s *TaskService) recordSubTasksBatch(ctx context.Context, onChainTaskID int64, rows []model.AgentSubTask) error {
	if len(rows) == 0 {
		return nil
	}
	txHash, err := s.Chain.RecordSubTasks(ctx, uint64(onChainTaskID), rows)
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

	summary := ""
	if task.Summary != nil {
		summary = *task.Summary
	}
	rawPrompt := ""
	if task.RawPrompt != nil {
		rawPrompt = *task.RawPrompt
	}

	onChainTaskID, err := s.Chain.CreateTask(ctx, task.WalletAddress, task.IsActionable, summary, crypto.Keccak256Hash([]byte(rawPrompt)))
	if err != nil {
		return ArmTaskResult{}, fmt.Errorf("agent: on-chain createTask: %w", err)
	}
	if task.IsActionable {
		duration := time.Duration(input.DurationSec) * time.Second
		if err := s.Chain.GrantTradePermission(ctx, onChainTaskID, input.TotalBudget, duration); err != nil {
			return ArmTaskResult{}, fmt.Errorf("agent: on-chain grantTradePermission: %w", err)
		}
	}
	if err := s.Tasks.SetOnChainTaskID(ctx, taskID, int64(onChainTaskID)); err != nil {
		return ArmTaskResult{}, fmt.Errorf("agent: persist on_chain_task_id: %w", err)
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
