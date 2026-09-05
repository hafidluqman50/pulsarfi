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
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"gorm.io/datatypes"
)

var (
	ErrWalletMismatch    = errors.New("agent: authenticated wallet does not own this task")
	ErrTaskNotFound      = errors.New("agent: task not found")
	ErrTaskNotActionable = errors.New("agent: task is not actionable")
	ErrTaskNotArmed      = errors.New("agent: task has not been armed yet")
	ErrChatNotFound      = errors.New("agent: chat not found")
)

// AgentContractClient is TaskService's own narrow view of the contract —
// deliberately not shared with SubTaskRetryService's own ChainClient
// (subtask_retry_service.go), which only ever needs RecordSubTasks. Same
// ISP reasoning already used for executor.StockLookup and the old
// ExecutorLogger.
type AgentContractClient interface {
	CreateTask(ctx context.Context, owner string, isActionable bool, summary string, promptHash [32]byte) (onChainTaskID uint64, err error)
	GrantTradePermission(ctx context.Context, onChainTaskID uint64, totalBudget string, duration time.Duration) error
	RecordSubTasks(ctx context.Context, onChainTaskID uint64, rows []model.AgentSubTask) (txHash string, err error)
	CancelTask(ctx context.Context, onChainTaskID uint64) error
}

// TaskService is the CRUD + run boundary between HTTP and Supervisor. It
// never touches the LLM directly — it builds a RunContext, invokes
// Supervisor, and batches whatever agent_sub_tasks rows a run produced
// into one on-chain recordSubTasks call once the run finishes.
type TaskService struct {
	Tasks        *repository.AgentTaskRepository
	SubTasks     *repository.AgentSubTaskRepository
	Trades       *repository.AgentTradeRepository
	Chats        *repository.AgentChatRepository
	ChatMessages *repository.AgentChatMessageRepository
	Supervisor   adk.Agent
	Chain        AgentContractClient
}

// WorkflowCard is what one HandleChatMessage turn returns to the frontend
// — a plain reply, or a chart (get_portfolio_snapshot was called this
// turn). Plan/Locked/Armed workflow_card rendering is derived by the
// frontend directly from agent_sub_tasks (agent-task-manager-code-implementation.md
// §4.2's PlanCard), not synthesized here.
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

// CreateChat opens a new, empty conversation thread — "+ new chat" in the
// Quasar panel. No Task exists yet; one is only opened later by
// Supervisor's own create_task tool, the first time a message in this
// chat is recognized as a genuinely distinct, data-needing request.
func (s *TaskService) CreateChat(ctx context.Context, walletAddress string, description *string) (model.AgentChat, error) {
	return s.Chats.Create(ctx, strings.ToLower(walletAddress), description)
}

func (s *TaskService) ListChats(ctx context.Context, walletAddress string) ([]model.AgentChat, error) {
	return s.Chats.FindByOwnerWallet(ctx, strings.ToLower(walletAddress))
}

// GetChatMessages returns a chat's full transcript in order — what the
// Quasar panel replays when a user reopens an existing chat.
func (s *TaskService) GetChatMessages(ctx context.Context, chatID int64, walletAddress string) ([]model.AgentChatMessage, error) {
	chat, found, err := s.Chats.FindByID(ctx, chatID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrChatNotFound
	}
	if !strings.EqualFold(chat.OwnerWallet, walletAddress) {
		return nil, ErrWalletMismatch
	}
	return s.ChatMessages.FindByChatID(ctx, chatID)
}

// GetReasoningChain returns a Task's full agent_sub_tasks hash chain in
// step order — public by design (no wallet check): the entire point is
// that anyone can fetch this, recompute the chain from its genesis hash,
// and verify the terminal hash matches AgentTaskManager's on-chain
// reasoningHash, not just the task's own owner.
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

// GetTrades returns a Task's on-chain Trade ledger — zero, one, or many
// fills, each its own row (agent-task-manager-rebuild.md §5 point 6: one
// on-chain Task id, many Trades). Scoped to the task's own owner, unlike
// GetReasoningChain — a Trade carries real fill amounts, not just
// verifiable hashes.
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

// HandleChatMessage is the chat-intake entry point. It never inserts an
// agent_tasks row itself — that's Supervisor's own create_task tool call,
// made when its own reasoning recognizes a genuinely new, data-needing
// request. This method's job: persist the user's turn, load whichever
// Task this chat is already attached to (if any, via
// agent_tasks.source_message_id -> agent_chat_messages.id — there is no
// direct chat_id column on Task), run Supervisor, persist its reply, and
// batch whatever new agent_sub_tasks rows this run produced — only if the
// Task is already armed (there is no on-chain Task to batch against
// before that).
func (s *TaskService) HandleChatMessage(ctx context.Context, chatID int64, wallet, message string) (WorkflowCard, error) {
	chat, found, err := s.Chats.FindByID(ctx, chatID)
	if err != nil {
		return WorkflowCard{}, err
	}
	if !found {
		return WorkflowCard{}, ErrChatNotFound
	}
	if !strings.EqualFold(chat.OwnerWallet, wallet) {
		return WorkflowCard{}, ErrWalletMismatch
	}

	existingMessages, err := s.ChatMessages.FindByChatID(ctx, chatID)
	if err != nil {
		return WorkflowCard{}, err
	}
	messageIDs := make([]int64, len(existingMessages))
	for i, m := range existingMessages {
		messageIDs[i] = m.ID
	}

	userMessage, err := s.ChatMessages.Create(ctx, repository.AgentChatMessageCreateInput{
		ChatID: chatID, Sender: "user", ContentType: "text", Content: message,
	})
	if err != nil {
		return WorkflowCard{}, fmt.Errorf("agent: persist user message: %w", err)
	}
	messageIDs = append(messageIDs, userMessage.ID)

	wallet = strings.ToLower(wallet)
	sourceMessageID := userMessage.ID
	runCtx := &RunContext{Wallet: wallet, TriggerDescription: message, SourceMessageID: &sourceMessageID}

	existingTask, taskFound, err := s.Tasks.FindByChatMessageIDs(ctx, messageIDs)
	if err != nil {
		return WorkflowCard{}, err
	}
	if taskFound {
		recorder, err := NewSubTaskRecorder(ctx, s.SubTasks, existingTask.ID, message, wallet)
		if err != nil {
			return WorkflowCard{}, fmt.Errorf("agent: build recorder for task %d: %w", existingTask.ID, err)
		}
		runCtx.TaskID = existingTask.ID
		runCtx.OnChainTaskID = existingTask.OnChainTaskID
		runCtx.Recorder = recorder
	}

	rowsBefore := 0
	if runCtx.Recorder != nil {
		rowsBefore = runCtx.Recorder.RowCount()
	}

	reply, toolCalls, err := RunAgentWithTrace(WithRunContext(ctx, runCtx), s.Supervisor, message)
	if err != nil {
		return WorkflowCard{}, fmt.Errorf("agent: supervisor run failed: %w", err)
	}

	// Only an already-armed Task has an on-chain counterpart to batch
	// against. Rows from a not-yet-armed Task stay recorded_on_chain =
	// false in Postgres on purpose — ArmTask does the catch-up batch.
	if runCtx.Recorder != nil && runCtx.OnChainTaskID != nil {
		newRows := runCtx.Recorder.RowsSince(rowsBefore)
		if err := s.recordSubTasksBatch(ctx, *runCtx.OnChainTaskID, newRows); err != nil {
			slog.ErrorContext(ctx, "agent: recordSubTasks batch failed, leaving for retry", "task_id", runCtx.TaskID, "error", err)
		}
	}

	card := buildWorkflowCard(runCtx.TaskID, reply, toolCalls)

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

// buildWorkflowCard classifies this turn's reply — a chart if
// get_portfolio_snapshot was called (its result is already the
// JSON-marshaled ChartPayload, per eino's tool-result serialization),
// plain text otherwise.
func buildWorkflowCard(taskID int64, reply string, toolCalls []ToolCallTrace) WorkflowCard {
	// Supervisor's own instructions say never to reply with raw JSON, but a
	// cheap/fast-tier model does not always obey that reliably — observed
	// live as {"reply": "..."} coming back instead of plain text. Unwrap it
	// defensively rather than trusting the instruction alone; fall back to
	// the raw string if it isn't actually that shape.
	reply = unwrapReplyJSON(reply)

	for _, tc := range toolCalls {
		if tc.ToolName == "get_portfolio_snapshot" {
			return WorkflowCard{TaskID: taskID, Reply: reply, ContentType: "chart", UIProps: json.RawMessage(tc.Result)}
		}
	}
	return WorkflowCard{TaskID: taskID, Reply: reply, ContentType: "text"}
}

func unwrapReplyJSON(reply string) string {
	var wrapped struct {
		Reply string `json:"reply"`
	}
	if err := json.Unmarshal([]byte(reply), &wrapped); err == nil && wrapped.Reply != "" {
		return wrapped.Reply
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

// ArmTask is the one call that actually moves this Task on-chain:
// createTask always, grantTradePermission only if the Task is actionable.
// Then does a one-time catch-up recordSubTasks batch for everything
// accumulated since Task creation — those rows could not be recorded
// on-chain before this exact moment, since there was no on-chain Task to
// batch against until now.
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

// DisarmTask calls on-chain cancelTask only. approve(0) on the relevant
// token is a separate, direct wallet transaction the frontend fires on
// its own — this method never touches ERC20 allowance.
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

// PauseTask and ResumeTask move no funds and touch no on-chain state — a
// paused Task's tick is skipped outright by the evaluation heartbeat
// (Evaluate below), not partially run.
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

// Evaluate is the scheduler-driven re-evaluation tick for an armed,
// actionable Task — the only place submit_trade can actually fire, since
// only an armed Task has a live TradePermission/allowance to check. A
// not-yet-armed or purely informational Task has nothing to re-evaluate
// (it was answered once, done), so both cases return ErrTaskNotActionable
// rather than silently invoking Supervisor for no reason.
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

	if _, _, err := RunAgentWithTrace(WithRunContext(ctx, runCtx), s.Supervisor, triggerPrompt); err != nil {
		return fmt.Errorf("agent: evaluation run failed for task %d: %w", taskID, err)
	}

	return s.recordSubTasksBatch(ctx, *runCtx.OnChainTaskID, recorder.RowsSince(rowsBefore))
}
