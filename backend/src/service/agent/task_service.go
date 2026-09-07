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
	Supervisor   adk.Agent
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

func (s *TaskService) HandleChatMessage(ctx context.Context, chatID uuid.UUID, wallet, message string, onSubTask func(model.AgentSubTask), onSubTaskStarted func(agentName, stepName, label string), onTextDelta func(string)) (WorkflowCard, error) {
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

	return s.runForMessage(ctx, chatID, userMessage, append(existingMessages, userMessage), strings.ToLower(wallet), onSubTask, onSubTaskStarted, onTextDelta)
}

func (s *TaskService) RetryLastMessage(ctx context.Context, chatID uuid.UUID, wallet string, onSubTask func(model.AgentSubTask), onSubTaskStarted func(agentName, stepName, label string), onTextDelta func(string)) (WorkflowCard, error) {
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

	return s.runForMessage(ctx, chatID, lastMessage, existingMessages, strings.ToLower(wallet), onSubTask, onSubTaskStarted, onTextDelta)
}

func (s *TaskService) runForMessage(ctx context.Context, chatID uuid.UUID, userMessage model.AgentChatMessage, allMessages []model.AgentChatMessage, wallet string, onSubTask func(model.AgentSubTask), onSubTaskStarted func(agentName, stepName, label string), onTextDelta func(string)) (WorkflowCard, error) {
	messageIDs := make([]int64, len(allMessages))
	for i, m := range allMessages {
		messageIDs[i] = m.ID
	}

	sourceMessageID := userMessage.ID
	runCtx := &RunContext{Wallet: wallet, TriggerDescription: userMessage.Content, SourceMessageID: &sourceMessageID, OnSubTask: onSubTask, OnSubTaskStarted: onSubTaskStarted, OnTextDelta: onTextDelta}

	existingTask, taskFound, err := s.Tasks.FindByChatMessageIDs(ctx, messageIDs)
	if err != nil {
		return WorkflowCard{}, err
	}
	if taskFound {
		lastSubTask, found, err := s.SubTasks.LastForTask(ctx, existingTask.ID)
		if err != nil {
			return WorkflowCard{}, err
		}
		if !found || lastSubTask.Status != "needs_input" {
			taskFound = false
		}
	}
	if taskFound {
		recorder, err := NewSubTaskRecorder(ctx, s.SubTasks, existingTask.ID, userMessage.Content, wallet)
		if err != nil {
			return WorkflowCard{}, fmt.Errorf("agent: build recorder for task %d: %w", existingTask.ID, err)
		}
		recorder.OnRecord = onSubTask
		runCtx.TaskID = existingTask.ID
		runCtx.OnChainTaskID = existingTask.OnChainTaskID
		runCtx.Recorder = recorder
	}

	rowsBefore := 0
	if runCtx.Recorder != nil {
		rowsBefore = runCtx.Recorder.RowCount()
	}

	history := make([]*schema.Message, len(allMessages))
	for i, m := range allMessages {
		if m.Sender == "user" {
			history[i] = schema.UserMessage(m.Content)
		} else {
			history[i] = schema.AssistantMessage(m.Content, nil)
		}
	}

	reply, toolCalls, err := RunAgentWithHistory(WithRunContext(ctx, runCtx), s.Supervisor, history, nil, "")
	if err != nil {
		return WorkflowCard{}, fmt.Errorf("agent: supervisor run failed: %w", err)
	}

	if runCtx.Recorder != nil && runCtx.OnChainTaskID != nil {
		newRows := runCtx.Recorder.RowsSince(rowsBefore)
		if err := s.recordSubTasksBatch(ctx, *runCtx.OnChainTaskID, newRows); err != nil {
			slog.ErrorContext(ctx, "agent: recordSubTasks batch failed, leaving for retry", "task_id", runCtx.TaskID, "error", err)
		}
	}

	allToolCalls := append(toolCalls, runCtx.NestedToolCalls...)
	card := buildWorkflowCard(runCtx.TaskID, reply, allToolCalls)

	var uiProps datatypes.JSON
	if len(card.UIProps) > 0 {
		uiProps = datatypes.JSON(card.UIProps)
	}
	var refTaskID *int64
	if runCtx.TaskID != 0 {
		refTaskID = &runCtx.TaskID
	}
	if _, err := s.ChatMessages.Create(ctx, repository.AgentChatMessageCreateInput{
		ChatID:      chatID,
		Sender:      "supervisor",
		ContentType: card.ContentType,
		Content:     card.Reply,
		UIComponent: card.UIComponent,
		UIProps:     uiProps,
		UIRefTaskID: refTaskID,
	}); err != nil {
		slog.ErrorContext(ctx, "agent: persist supervisor reply failed", "error", err)
	}

	return card, nil
}

func buildWorkflowCard(taskID int64, reply string, toolCalls []ToolCallTrace) WorkflowCard {
	reply = unwrapReplyJSON(reply)

	// get_portfolio_snapshot and get_stock_chart (analyzer/chart_service.go)
	// both return a publicsvc.ChartPayload — either one means this reply has
	// real chart-ready data behind it, not just prose. Collects every match,
	// not just the first: a compound request ("chart-in portofolio ku, dan
	// chart BRPT") makes Analyzer call both tools in the same turn, and
	// returning only the first found silently dropped the second chart even
	// though Analyzer's own reply claimed both had rendered — confirmed live
	// via a request where the model correctly fetched two charts but the
	// user only ever saw one (docs/plans/agent-task-manager-code-implementation.md).
	var chartPayloads []json.RawMessage
	for _, tc := range toolCalls {
		if tc.ToolName == "get_portfolio_snapshot" || tc.ToolName == "get_stock_chart" {
			chartPayloads = append(chartPayloads, json.RawMessage(tc.Result))
		}
	}
	if len(chartPayloads) == 1 {
		return WorkflowCard{TaskID: taskID, Reply: reply, ContentType: "chart", UIProps: chartPayloads[0]}
	}
	if len(chartPayloads) > 1 {
		if payload, err := json.Marshal(chartPayloads); err == nil {
			return WorkflowCard{TaskID: taskID, Reply: reply, ContentType: "chart", UIProps: payload}
		}
	}

	// analyzer_agent's own conclusion (GlobalInstructions' JSON contract —
	// condition_met/confidence/evidence/reasoning) carries real, sourced
	// citations whenever news evidence actually drove the conclusion.
	// Surfaced as its own card so the user sees dated, linked sources, not
	// just Supervisor's prose summary of them.
	for _, tc := range toolCalls {
		if tc.ToolName != "analyzer_agent" {
			continue
		}
		if evidence := extractNewsEvidence(tc.Result); len(evidence) > 0 {
			payload, err := json.Marshal(evidence)
			if err == nil {
				return WorkflowCard{TaskID: taskID, Reply: reply, ContentType: "news", UIProps: payload}
			}
		}
	}
	return WorkflowCard{TaskID: taskID, Reply: reply, ContentType: "text"}
}

// NewsEvidenceItem mirrors one item of analyzer_agent's own "evidence"
// array (its own JSON contract, see GlobalInstructions/analyzer/instructions.go) —
// re-parsed here purely to surface it as its own chart-like card, never to
// re-derive or alter what Analyzer actually concluded.
type NewsEvidenceItem struct {
	Source      string `json:"source"`
	URL         string `json:"url,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
	Excerpt     string `json:"excerpt,omitempty"`
	ImageURL    string `json:"image_url,omitempty"`
}

func extractNewsEvidence(raw string) []NewsEvidenceItem {
	var parsed struct {
		Evidence []NewsEvidenceItem `json:"evidence"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil
	}
	return parsed.Evidence
}

func unwrapReplyJSON(reply string) string {
	var wrapped struct {
		Reply    string `json:"reply"`
		Message  string `json:"message"`
		Response string `json:"response"`
		Answer   string `json:"answer"`
	}
	if err := json.Unmarshal([]byte(reply), &wrapped); err == nil {
		for _, candidate := range []string{wrapped.Reply, wrapped.Message, wrapped.Response, wrapped.Answer} {
			if candidate != "" {
				return candidate
			}
		}
	}
	return reply
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

func (s *TaskService) Evaluate(ctx context.Context, taskID int64) error {
	task, found, err := s.Tasks.FindByID(ctx, taskID)
	if err != nil {
		return err
	}
	if !found {
		return ErrTaskNotFound
	}
	if task.Paused {
		return nil
	}
	if !task.IsActionable || task.OnChainTaskID == nil {
		return ErrTaskNotActionable
	}

	recorder, err := NewSubTaskRecorder(ctx, s.SubTasks, taskID, "", task.WalletAddress)
	if err != nil {
		return err
	}
	runCtx := &RunContext{TaskID: taskID, OnChainTaskID: task.OnChainTaskID, Wallet: task.WalletAddress, Recorder: recorder}
	rowsBefore := recorder.RowCount()

	triggerPrompt := ""
	if task.TriggerDescription != nil {
		triggerPrompt = *task.TriggerDescription
	}

	if _, _, err := RunAgentWithTrace(WithRunContext(ctx, runCtx), s.Supervisor, triggerPrompt, nil, ""); err != nil {
		return fmt.Errorf("agent: evaluation run failed for task %d: %w", taskID, err)
	}

	return s.recordSubTasksBatch(ctx, *runCtx.OnChainTaskID, recorder.RowsSince(rowsBefore))
}

func truncateRunes(s string, max int) string {
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max])
}
