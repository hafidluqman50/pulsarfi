package agent

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/horizonlabs/pulsarfi-backend/src/contracts"
	dbmodel "github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
)

func init() {
	// Interrupt payloads travel inside the Eino checkpoint, so gob must know them.
	schema.RegisterName[QuestionsInterrupt]("agent_questions_interrupt")
	schema.RegisterName[ArmInterrupt]("agent_arm_interrupt")
}

var (
	ErrNoOpenTask         = errors.New("agent: no open task, call open_task first")
	ErrNoPendingExecution = errors.New("agent: task has no pending arm pause to resume")
)

// Orchestrator runs one Quasar agent (Nova and Comet are called by it) over
// an adk.Runner. Pauses are Eino interrupts raised by Quasar's tools: the
// questions card (ask_user) and the Arm Card (await_arm). Both are persisted
// in CheckPointStore and resumed in a later HTTP request.
type Orchestrator struct {
	Tasks        *repository.AgentTaskRepository
	SubTasks     *repository.AgentSubTaskRepository
	ChatMessages *repository.AgentChatMessageRepository
	Chain        OrchestratorChainClient

	Quasar     adk.Agent
	ReplyModel model.BaseChatModel

	Stocks StockCatalog

	CheckPointStore ExtendedCheckPointStore

	runner *adk.Runner
}

// OrchestratorChainClient defines the narrow on-chain interface required
// by the orchestrator turn.
type OrchestratorChainClient interface {
	CreateTask(ctx context.Context, owner string, isActionable bool, summary string, promptHash [32]byte) (onChainTaskID uint64, err error)
	MarkActionable(ctx context.Context, onChainTaskID uint64) error
	ChainSubTaskRecorder
}

// StockCatalog is the orchestrator's narrow view of the stock catalog, used
// only to resolve a sell Task's token address when the Task is written.
type StockCatalog interface {
	FindByTickerOrIdxTicker(ctx context.Context, ticker string) (dbmodel.Stock, bool, error)
	FindMarketReady(ctx context.Context) ([]dbmodel.Stock, error)
}

func NewOrchestrator(ctx context.Context, o *Orchestrator) (*Orchestrator, error) {
	if o.Quasar == nil {
		return nil, fmt.Errorf("orchestrator: Quasar agent is required")
	}
	o.runner = adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           o.Quasar,
		EnableStreaming: true,
		CheckPointStore: o.CheckPointStore,
	})
	return o, nil
}

type OrchestratorInput struct {
	ChatID          uuid.UUID
	Wallet          string
	RawPrompt       string
	Messages        []*schema.Message
	SourceMessageID *int64
}

type OrchestratorResult struct {
	TaskID           int64
	OnChainTaskID    *int64
	Reply            string
	ContentType      string // "text" | "chart" | "news" | "workflow_card"
	UIComponent      string
	UIProps          json.RawMessage
	PendingQuestions []contracts.IntakeField
}

// RouteDecision is the argument of Quasar's open_task and update_task tools:
// what Quasar understood about the request. It is also what the Task row's
// trigger description is built from.
type RouteDecision struct {
	IsActionable    bool                    `json:"is_actionable" jsonschema_description:"true only if this request could ever result in an on-chain trade; false for anything purely informational (chart, portfolio, news) and while trade parameters are still missing"`
	Summary         string                  `json:"summary" jsonschema_description:"short one-line summary of the request in the exact language or dialect of the user's message, suitable for on-chain storage"`
	Label           string                  `json:"label" jsonschema_description:"short human-readable title for this step, in the exact language or dialect of the user's message"`
	Shape           string                  `json:"shape,omitempty" jsonschema_description:"scalp, swing, investment, or empty when the user has not chosen one"`
	Side            string                  `json:"side,omitempty" jsonschema_description:"buy or sell (canonical English value), or empty when unknown"`
	MentionedTicker string                  `json:"mentioned_ticker,omitempty" jsonschema_description:"canonical PulsarFi ticker for the stock the user named: ALL-CAPS with the P suffix (BRPT resolves to BRPTP), or empty when none was named"`
	BudgetIDRX      string                  `json:"budget_idrx,omitempty" jsonschema_description:"confirmed IDRX budget as plain digits, or empty when not applicable or unknown"`
	SellAmount      string                  `json:"sell_amount,omitempty" jsonschema_description:"for sell: confirmed quantity of stock tokens as plain digits, or a percentage such as 50%, or empty when unknown"`
	Card            *contracts.CardContract `json:"card,omitempty" jsonschema_description:"mandatory for any trade Task, authored entirely in the user's language; see the card contract in your instructions"`
}

// QuestionsInterrupt is the payload of the ask_user pause. Message is the
// sentence that introduces the card; it travels with the pause so the user
// never sees a card under an empty bubble.
type QuestionsInterrupt struct {
	Questions []contracts.IntakeField
	Message   string
}

// ArmInterrupt is the payload of the await_arm pause. Message introduces the
// Arm Card, same reason as above.
type ArmInterrupt struct {
	TaskID  int64
	Message string
}

const (
	interruptQuestions = "questions"
	interruptArm       = "arm"
)

type pendingInterrupt struct {
	kind string
	id   string
}

// Send runs one turn for a chat message. If the chat is paused on a
// questions card, the message is the answer and the paused turn resumes;
// the Arm pause is never resumed by a chat message, only by ResumeExecute.
func (o *Orchestrator) Send(ctx context.Context, in OrchestratorInput) (OrchestratorResult, error) {
	rc := o.newRunContext(in.ChatID, in.Wallet, in.RawPrompt, in.SourceMessageID)
	ctx = WithRunContext(ctx, rc)

	result, err := o.runTurn(ctx, rc, in)
	if errors.Is(err, errToolBudgetExhausted) {
		o.finishOnBudget(ctx, in, &result)
		return result, nil
	}
	return result, err
}

// runTurn is the turn's dispatch: resume a pending questions card, or run a
// fresh turn (keeping the Arm pause alive when one is pending).
func (o *Orchestrator) runTurn(ctx context.Context, rc *RunContext, in OrchestratorInput) (OrchestratorResult, error) {
	pending, hasPending := o.pending(ctx, in.ChatID)
	if hasPending && pending.kind == interruptQuestions {
		if err := o.restoreFromChat(ctx, rc); err != nil {
			return OrchestratorResult{}, err
		}
		iter, err := o.runner.ResumeWithParams(ctx, in.ChatID.String(), &adk.ResumeParams{Targets: map[string]any{pending.id: in.RawPrompt}})
		if err != nil {
			return OrchestratorResult{}, fmt.Errorf("orchestrator: resume questions pause: %w", err)
		}
		return o.consume(ctx, rc, iter, false)
	}

	iter := o.runner.Run(ctx, in.Messages, adk.WithCheckPointID(in.ChatID.String()))
	return o.consume(ctx, rc, iter, hasPending && pending.kind == interruptArm)
}

// ResumeExecute resumes the Arm pause once the Task layer has confirmed
// on-chain arm and allowance. It carries only a bare "armed" signal: what to
// execute already lives in the checkpointed conversation.
func (o *Orchestrator) ResumeExecute(ctx context.Context, taskID int64, wallet string) (ExecuteTaskResult, error) {
	task, found, err := o.Tasks.FindByID(ctx, taskID)
	if err != nil || !found {
		return ExecuteTaskResult{}, fmt.Errorf("orchestrator: task not found: %d", taskID)
	}

	chatID := o.resolveChatID(ctx, task)
	pending, hasPending := o.pending(ctx, chatID)
	if chatID == uuid.Nil || !hasPending || pending.kind != interruptArm {
		return ExecuteTaskResult{}, fmt.Errorf("%w: task %d", ErrNoPendingExecution, taskID)
	}

	rc := o.newRunContext(chatID, wallet, "", task.SourceMessageID)
	if err := o.restoreRunContext(ctx, rc, task); err != nil {
		return ExecuteTaskResult{}, err
	}
	ctx = WithRunContext(ctx, rc)

	iter, err := o.runner.ResumeWithParams(ctx, chatID.String(), &adk.ResumeParams{Targets: map[string]any{pending.id: "armed"}})
	if err != nil {
		return ExecuteTaskResult{}, fmt.Errorf("orchestrator: resume arm pause: %w", err)
	}
	result, err := o.consume(ctx, rc, iter, false)
	if errors.Is(err, errToolBudgetExhausted) {
		// The budget is not a failure: return what the turn produced.
		return ExecuteTaskResult{TaskID: taskID, Status: "executed", Reply: result.Reply}, nil
	}
	if err != nil {
		return ExecuteTaskResult{}, fmt.Errorf("orchestrator: resume execute failed: %w", err)
	}
	return ExecuteTaskResult{TaskID: taskID, Status: "executed", Reply: result.Reply}, nil
}

func (o *Orchestrator) pending(ctx context.Context, chatID uuid.UUID) (pendingInterrupt, bool) {
	if chatID == uuid.Nil {
		return pendingInterrupt{}, false
	}
	stored, found, err := o.CheckPointStore.GetInterruptID(ctx, chatID.String())
	if err != nil || !found {
		return pendingInterrupt{}, false
	}
	kind, id, ok := strings.Cut(stored, "|")
	if !ok || id == "" {
		return pendingInterrupt{}, false
	}
	return pendingInterrupt{kind: kind, id: id}, true
}

func (o *Orchestrator) storeInterrupt(ctx context.Context, chatID uuid.UUID, kind, id string) {
	if chatID == uuid.Nil {
		return
	}
	_ = o.CheckPointStore.SetInterruptID(ctx, chatID.String(), kind+"|"+id)
}

// resolveChatID finds the chat a Task belongs to, needed to look up its
// checkpoint/interrupt ID — via the Task's own source message first, falling
// back to any chat message that references this Task's card.
func (o *Orchestrator) resolveChatID(ctx context.Context, task dbmodel.AgentTask) uuid.UUID {
	if task.SourceMessageID != nil {
		if msg, found, err := o.ChatMessages.FindByID(ctx, *task.SourceMessageID); err == nil && found {
			return msg.ChatID
		}
	}
	if msg, found, err := o.ChatMessages.FindByUIRefTaskID(ctx, task.ID); err == nil && found {
		return msg.ChatID
	}
	return uuid.Nil
}

const finalAnswerTimeout = 30 * time.Second

// toolBudgetInstructions names no language: the model mirrors the user's.
const toolBudgetInstructions = `You have reached your tool-call budget for this turn. Answer the user now using only the evidence you already have: do not call any more tools. Be honest about anything you could not verify. Reply in the same language as the user's message.`

// finishOnBudget turns a budget-exhausted turn into a normal one. It keeps the
// agent's own text when there is any; otherwise it asks the model once, with no
// tools, for an answer from the evidence already gathered. No hardcoded reply
// is written here.
func (o *Orchestrator) finishOnBudget(ctx context.Context, in OrchestratorInput, result *OrchestratorResult) {
	if strings.TrimSpace(result.Reply) != "" || o.ReplyModel == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, finalAnswerTimeout)
	defer cancel()

	messages := append([]*schema.Message{schema.SystemMessage(toolBudgetInstructions)}, in.Messages...)
	resp, err := o.ReplyModel.Generate(ctx, messages)
	if err != nil || resp == nil || strings.TrimSpace(resp.Content) == "" {
		return
	}
	result.Reply = strings.TrimSpace(resp.Content)
	PublishChatEvent(in.ChatID, "reply_delta", map[string]any{"delta": result.Reply})
}
