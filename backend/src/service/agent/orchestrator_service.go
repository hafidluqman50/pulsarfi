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

// Reading map for this file.
//
// A chat message arrives at ChatService.HandleChatMessage, which resolves a
// turnIntent (see chat_service.go) and calls Orchestrator.Send with it. Send
// is the router: it runs the turn and, through consume/finish, publishes the
// live events and records any pause.
//
//   - answerQuestions       -> resumeQuestionsTurn: the message IS the answer.
//   - sideQuestionDuringArm -> runFreshTurn(keepCheckpoint=true): a new turn,
//     the pending Arm pause stays alive.
//   - freshTurn             -> runFreshTurn(keepCheckpoint=false).
//
// What raises an interrupt is never decided here: Quasar's own tools do it
// (ask_user -> questions, await_arm -> arm). Pauses are Eino interrupts
// persisted in CheckPointStore and resumed in a later HTTP request.
// See docs/plans/agent-turn-pipeline-refactor.md.
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

func NewOrchestrator(ctx context.Context, orch *Orchestrator) (*Orchestrator, error) {
	if orch.Quasar == nil {
		return nil, fmt.Errorf("orchestrator: Quasar agent is required")
	}
	orch.runner = adk.NewRunner(ctx, adk.RunnerConfig{
		Agent:           orch.Quasar,
		EnableStreaming: true,
		CheckPointStore: orch.CheckPointStore,
	})
	return orch, nil
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

// Send runs one turn for a chat message, routed by the intent ChatService
// already resolved — including which pause it answers, so the pending marker
// is read exactly once, in ChatService.resolveTurn. A tool-budget cap is not a
// failure: the turn is finalized from whatever it produced.
func (orch *Orchestrator) Send(ctx context.Context, in OrchestratorInput, intent turnIntent) (OrchestratorResult, error) {
	run := orch.newRunContext(in.ChatID, in.Wallet, in.RawPrompt, in.SourceMessageID)
	ctx = WithRunContext(ctx, run)

	result, err := orch.dispatchTurn(ctx, run, in, intent)
	if errors.Is(err, errToolBudgetExhausted) {
		orch.finishOnBudget(ctx, in, &result)
		return result, nil
	}
	return result, err
}

func (orch *Orchestrator) dispatchTurn(ctx context.Context, run *RunContext, in OrchestratorInput, intent turnIntent) (OrchestratorResult, error) {
	switch intent.kind {
	case answerQuestions:
		return orch.resumeQuestionsTurn(ctx, run, in, intent.interruptID)
	case sideQuestionDuringArm:
		// The message starts a new turn; the pending Arm pause stays alive.
		return orch.runFreshTurn(ctx, run, in, true)
	default:
		return orch.runFreshTurn(ctx, run, in, false)
	}
}

// resumeQuestionsTurn answers a pending questions card: the user's message is
// the suspended ask_user tool's result.
func (orch *Orchestrator) resumeQuestionsTurn(ctx context.Context, run *RunContext, in OrchestratorInput, interruptID string) (OrchestratorResult, error) {
	if err := orch.restoreFromChat(ctx, run); err != nil {
		return OrchestratorResult{}, err
	}
	events, err := orch.runner.ResumeWithParams(ctx, run.ChatID.String(), &adk.ResumeParams{
		Targets: map[string]any{interruptID: in.RawPrompt},
	})
	if err != nil {
		return OrchestratorResult{}, fmt.Errorf("orchestrator: resume questions pause: %w", err)
	}
	return orch.consume(ctx, run, events, false)
}

// runFreshTurn starts a new turn. keepCheckpoint is true only when the chat is
// paused on the Arm card and this message is a side question: the Arm pause
// must survive the new turn.
func (orch *Orchestrator) runFreshTurn(ctx context.Context, run *RunContext, in OrchestratorInput, keepCheckpoint bool) (OrchestratorResult, error) {
	events := orch.runner.Run(ctx, in.Messages, adk.WithCheckPointID(run.ChatID.String()))
	return orch.consume(ctx, run, events, keepCheckpoint)
}

// ResumeExecute resumes the Arm pause once the Task layer has confirmed
// on-chain arm and allowance. It carries only a bare "armed" signal: what to
// execute already lives in the checkpointed conversation.
func (orch *Orchestrator) ResumeExecute(ctx context.Context, taskID int64, wallet string) (ExecuteTaskResult, error) {
	task, found, err := orch.Tasks.FindByID(ctx, taskID)
	if err != nil || !found {
		return ExecuteTaskResult{}, fmt.Errorf("orchestrator: task not found: %d", taskID)
	}

	chatID := orch.chatIDForTask(ctx, task)
	pending, hasPending := orch.interrupts().pending(ctx, chatID)
	if chatID == uuid.Nil || !hasPending || pending.kind != interruptArm {
		return ExecuteTaskResult{}, fmt.Errorf("%w: task %d", ErrNoPendingExecution, taskID)
	}

	run := orch.newRunContext(chatID, wallet, "", task.SourceMessageID)
	if err := orch.restoreRunContext(ctx, run, task); err != nil {
		return ExecuteTaskResult{}, err
	}
	ctx = WithRunContext(ctx, run)

	events, err := orch.runner.ResumeWithParams(ctx, chatID.String(), &adk.ResumeParams{Targets: map[string]any{pending.id: "armed"}})
	if err != nil {
		return ExecuteTaskResult{}, fmt.Errorf("orchestrator: resume arm pause: %w", err)
	}
	result, err := orch.consume(ctx, run, events, false)
	if errors.Is(err, errToolBudgetExhausted) {
		// The budget is not a failure: return what the turn produced.
		return ExecuteTaskResult{TaskID: taskID, Status: "executed", Reply: result.Reply}, nil
	}
	if err != nil {
		return ExecuteTaskResult{}, fmt.Errorf("orchestrator: resume execute failed: %w", err)
	}
	return ExecuteTaskResult{TaskID: taskID, Status: "executed", Reply: result.Reply}, nil
}

// chatIDForTask finds the chat a Task belongs to, needed to look up its
// checkpoint/interrupt ID — via the Task's own source message first, falling
// back to any chat message that references this Task's card.
func (orch *Orchestrator) chatIDForTask(ctx context.Context, task dbmodel.AgentTask) uuid.UUID {
	if task.SourceMessageID != nil {
		if msg, found, err := orch.ChatMessages.FindByID(ctx, *task.SourceMessageID); err == nil && found {
			return msg.ChatID
		}
	}
	if msg, found, err := orch.ChatMessages.FindByUIRefTaskID(ctx, task.ID); err == nil && found {
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
func (orch *Orchestrator) finishOnBudget(ctx context.Context, in OrchestratorInput, result *OrchestratorResult) {
	if strings.TrimSpace(result.Reply) != "" || orch.ReplyModel == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, finalAnswerTimeout)
	defer cancel()

	messages := append([]*schema.Message{schema.SystemMessage(toolBudgetInstructions)}, in.Messages...)
	resp, err := orch.ReplyModel.Generate(ctx, messages)
	if err != nil || resp == nil || strings.TrimSpace(resp.Content) == "" {
		return
	}
	result.Reply = strings.TrimSpace(resp.Content)
	PublishChatEvent(in.ChatID, "reply_delta", map[string]any{"delta": result.Reply})
}

// CancellationReply writes the sentence that confirms a cancelled trade plan,
// in the user's own language. It lives here, next to the turn it belongs to,
// because the Orchestrator owns ReplyModel — ChatService never builds a prompt.
func (orch *Orchestrator) CancellationReply(ctx context.Context, ticker string, history []dbmodel.AgentChatMessage) string {
	if orch.ReplyModel == nil {
		return ""
	}

	recent := history
	if len(recent) > 10 {
		recent = recent[len(recent)-10:]
	}

	target := ticker
	if target == "" {
		target = "the asset"
	}

	systemPrompt := fmt.Sprintf(`You are Quasar, PulsarFi's AI trading assistant.
The user has explicitly cancelled the pending trade confirmation for %s.

CRITICAL USER-DRIVEN LANGUAGE POLICY:
Detect and mirror 100%% the user's active language from the chat history (Indonesian, English, Javanese, Turkish, Japanese, etc.).
NEVER hardcode or assume any single language. Do NOT use em dashes.
Produce a single friendly, professional confirmation sentence stating that the trade plan for %s has been cancelled and no orders or on-chain transactions were executed.
Return ONLY the confirmation sentence, with no markdown fences, greetings, or extra commentary.`, target, target)

	msgs := append([]*schema.Message{schema.SystemMessage(systemPrompt)}, chatHistory(recent)...)
	resp, err := orch.ReplyModel.Generate(ctx, msgs)
	if err == nil && resp != nil && strings.TrimSpace(resp.Content) != "" {
		return strings.TrimSpace(resp.Content)
	}
	return ""
}
