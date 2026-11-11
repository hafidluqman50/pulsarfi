package orchestrator

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	agentcore "github.com/horizonlabs/pulsarfi-backend/src/app/core/agent"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/horizonlabs/pulsarfi-backend/src/contracts"
	dbmodel "github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/analyzer"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/executor"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/supervisor"
)

func init() {
	// Interrupt payloads travel inside the Eino checkpoint, so gob must know them.
	schema.RegisterName[agentcore.QuestionsInterrupt]("agent_questions_interrupt")
	schema.RegisterName[agentcore.ArmInterrupt]("agent_arm_interrupt")
}

// Reading map for this file.
//
// A chat message arrives at ChatService.HandleChatMessage, which resolves a
// turnIntent (see chat_service.go) and calls Orchestrator.Send with it. Send
// runs the turn through the graph (buildGraph, below) and, through
// invokeGraph/finish, publishes the live events and records any pause.
//
//   - answerQuestions       -> resumeQuestionsTurn: the message IS the answer.
//   - sideQuestionDuringArm -> runSideTurn: a new turn, the pending Arm pause
//     stays alive.
//   - freshTurn             -> runFreshTurn.
//
// What raises an interrupt is never decided here: Quasar's own tools do it
// (ask_user -> questions, await_arm -> arm). Pauses are Eino interrupts
// persisted in CheckPointStore and resumed in a later HTTP request.
// See docs/plans/agent-workflow-graph.md.
type Orchestrator struct {
	Tasks        *repository.AgentTaskRepository
	SubTasks     *repository.AgentSubTaskRepository
	ChatMessages *repository.AgentChatMessageRepository
	Chain        OrchestratorChainClient

    Quasar     adk.Agent
    Nova       adk.Agent
    Comet      adk.Agent
    ReplyModel model.BaseChatModel

    Stocks StockCatalog

    CheckPointStore agentcore.ExtendedCheckPointStore

    graph        compose.Runnable[*agentcore.TurnState, *agentcore.TurnState]
    quasarRunner *adk.Runner
    novaRunner   *adk.Runner
    cometRunner  *adk.Runner

    quasarNode agentcore.NodeFunc
    novaNode   agentcore.NodeFunc
    cometNode  agentcore.NodeFunc
}

// OrchestratorChainClient defines the narrow on-chain interface required
// by the orchestrator turn.
type OrchestratorChainClient interface {
	CreateTask(ctx context.Context, owner string, isActionable bool, summary string, promptHash [32]byte) (onChainTaskID uint64, err error)
	MarkActionable(ctx context.Context, onChainTaskID uint64) error
	agentcore.ChainSubTaskRecorder
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

    orch.quasarRunner = orch.newRunner(ctx, orch.Quasar)
    orch.novaRunner = orch.newRunner(ctx, orch.Nova)
    orch.cometRunner = orch.newRunner(ctx, orch.Comet)

    orch.quasarNode = supervisor.NewNode(orch.quasarRunner, orch.runAgentStep)
    orch.novaNode = analyzer.NewNode(orch.novaRunner, orch.runAgentStep)
    orch.cometNode = executor.NewNode(orch.cometRunner, orch.Tasks, orch.runAgentStep)

    graph, err := orch.buildGraph(ctx)
    if err != nil {
        return nil, err
    }

    orch.graph = graph
    return orch, nil
}

func (orch *Orchestrator) newRunner(ctx context.Context, agent adk.Agent) *adk.Runner {
    if agent == nil {
        return nil
    }

    return adk.NewRunner(ctx, adk.RunnerConfig{
        Agent:           agent,
        EnableStreaming: true,
        CheckPointStore: orch.CheckPointStore,
    })
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

func (orch *Orchestrator) interrupts() *agentcore.InterruptStore {
	return agentcore.NewInterruptStore(orch.CheckPointStore)
}

// Send runs one turn for a chat message, routed by the intent ChatService
// already resolved — including which pause it answers, so the pending marker
// is read exactly once, in ChatService.resolveTurn. A tool-budget cap is not a
// failure: the turn is finalized from whatever it produced.
func (orch *Orchestrator) Send(ctx context.Context, in OrchestratorInput, intent agentcore.TurnIntent) (OrchestratorResult, error) {
    run := orch.newRunContext(in.ChatID, in.Wallet, in.RawPrompt, in.SourceMessageID)
    scope := &turnScope{run: run, outcome: &turnOutcome{}}
    ctx = withTurnScope(agentcore.WithRunContext(ctx, run), scope)

    result, err := orch.dispatchTurn(ctx, scope, in, intent)
    if errors.Is(err, errToolBudgetExhausted) {
        orch.finishOnBudget(ctx, in, &result)
        return result, nil
    }

    return result, err
}

func (orch *Orchestrator) dispatchTurn(ctx context.Context, scope *turnScope, in OrchestratorInput, intent agentcore.TurnIntent) (OrchestratorResult, error) {
    switch intent.Kind {
    case agentcore.AnswerQuestions:
        return orch.resumeQuestionsTurn(ctx, scope, in, intent.InterruptID)
    case agentcore.SideQuestionDuringArm:
        return orch.runSideTurn(ctx, scope, in)
    default:
        return orch.runFreshTurn(ctx, scope, in)
    }
}

func (orch *Orchestrator) resumeQuestionsTurn(ctx context.Context, scope *turnScope, in OrchestratorInput, interruptID string) (OrchestratorResult, error) {
    if err := orch.restoreFromChat(ctx, scope.run); err != nil {
        return OrchestratorResult{}, err
    }

    return orch.invokeGraph(compose.ResumeWithData(ctx, interruptID, in.RawPrompt), scope, nil, true, false)
}

func (orch *Orchestrator) runFreshTurn(ctx context.Context, scope *turnScope, in OrchestratorInput) (OrchestratorResult, error) {
    state := &agentcore.TurnState{ChatID: in.ChatID, Wallet: in.Wallet, Messages: in.Messages}
    return orch.invokeGraph(ctx, scope, state, true, false)
}

// A side question while the chat is paused on the Arm Card starts a new turn
// with its own agent checkpoints and no graph checkpoint, so the pending Arm
// pause stays untouched. A pause raised by the side turn itself is dropped.
func (orch *Orchestrator) runSideTurn(ctx context.Context, scope *turnScope, in OrchestratorInput) (OrchestratorResult, error) {
    state := &agentcore.TurnState{ChatID: in.ChatID, Wallet: in.Wallet, Scope: "side-" + uuid.NewString(), Messages: in.Messages}
    return orch.invokeGraph(ctx, scope, state, false, true)
}

// ctx: context carrying the turn scope, already holding the resume data when resuming.
// scope: run context and outcome of this request.
// state: graph input; nil when resuming a pause.
// checkpointed: whether the graph saves its pause under the chat id.
// keepCheckpoint: true for a side turn, which must not clear or replace the pending pause.
func (orch *Orchestrator) invokeGraph(ctx context.Context, scope *turnScope, state *agentcore.TurnState, checkpointed bool, keepCheckpoint bool) (OrchestratorResult, error) {
    var options []compose.Option
    if checkpointed && orch.CheckPointStore != nil {
        options = append(options, compose.WithCheckPointID(scope.run.ChatID.String()))
    }

    _, err := orch.graph.Invoke(ctx, state, options...)
    if err != nil {
        info, interrupted := compose.ExtractInterruptInfo(err)
        if !interrupted {
            if isToolBudgetError(err) {
                return orch.finish(ctx, scope.run, scope.outcome, keepCheckpoint), errToolBudgetExhausted
            }

            return OrchestratorResult{}, err
        }

        if !keepCheckpoint {
            scope.outcome.interrupt = interruptFrom(info.InterruptContexts)
        }
    }

    return orch.finish(ctx, scope.run, scope.outcome, keepCheckpoint), nil
}

// ResumeExecute resumes the Arm pause once the Task layer has confirmed
// on-chain arm and allowance. It carries only a bare "armed" signal: what to
// execute already lives in the checkpointed turn.
func (orch *Orchestrator) ResumeExecute(ctx context.Context, taskID int64, wallet string) (agentcore.ExecuteTaskResult, error) {
    task, found, err := orch.Tasks.FindByID(ctx, taskID)
    if err != nil || !found {
        return agentcore.ExecuteTaskResult{}, fmt.Errorf("orchestrator: task not found: %d", taskID)
    }

    chatID := orch.chatIDForTask(ctx, task)
    pending, hasPending := orch.interrupts().Pending(ctx, chatID)
    if chatID == uuid.Nil || !hasPending || pending.Kind != agentcore.InterruptArm {
        return agentcore.ExecuteTaskResult{}, fmt.Errorf("%w: task %d", agentcore.ErrNoPendingExecution, taskID)
    }

    run := orch.newRunContext(chatID, wallet, "", task.SourceMessageID)
    if err := orch.restoreRunContext(ctx, run, task); err != nil {
        return agentcore.ExecuteTaskResult{}, err
    }

    scope := &turnScope{run: run, outcome: &turnOutcome{}}
    ctx = withTurnScope(agentcore.WithRunContext(ctx, run), scope)

    result, err := orch.invokeGraph(compose.ResumeWithData(ctx, pending.ID, "armed"), scope, nil, true, false)
    if errors.Is(err, errToolBudgetExhausted) {
        return agentcore.ExecuteTaskResult{TaskID: taskID, Status: "executed", Reply: result.Reply}, nil
    }

    if err != nil {
        return agentcore.ExecuteTaskResult{}, fmt.Errorf("orchestrator: resume execute failed: %w", err)
    }

    return agentcore.ExecuteTaskResult{TaskID: taskID, Status: "executed", Reply: result.Reply}, nil
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

	msgs := append([]*schema.Message{schema.SystemMessage(systemPrompt)}, agentcore.ChatHistory(recent)...)
	resp, err := orch.ReplyModel.Generate(ctx, msgs)
	if err == nil && resp != nil && strings.TrimSpace(resp.Content) != "" {
		return strings.TrimSpace(resp.Content)
	}
	return ""
}

const maxGraphSteps = 20

func (orchestrator *Orchestrator) buildGraph(ctx context.Context) (compose.Runnable[*agentcore.TurnState, *agentcore.TurnState], error) {
    graph := compose.NewGraph[*agentcore.TurnState, *agentcore.TurnState]()
    nodes := map[string]func(context.Context, *agentcore.TurnState) (*agentcore.TurnState, error){
        "quasar": orchestrator.quasarNode,
        "nova":   orchestrator.novaNode,
        "comet":  orchestrator.cometNode,
    }
    for name, node := range nodes {
        if err := graph.AddLambdaNode(name, compose.InvokableLambda(node)); err != nil {
            return nil, fmt.Errorf("orchestrator: add node %s: %w", name, err)
        }
    }

    if err := graph.AddEdge(compose.START, "quasar"); err != nil {
        return nil, fmt.Errorf("orchestrator: add start edge: %w", err)
    }

    branch := compose.NewGraphBranch(func(_ context.Context, state *agentcore.TurnState) (string, error) {
        if state.NeedsNova() {
            return "nova", nil
        }

        if state.NeedsComet() {
            return "comet", nil
        }

        return compose.END, nil
    }, map[string]bool{"nova": true, "comet": true, compose.END: true})
    if err := graph.AddBranch("quasar", branch); err != nil {
        return nil, fmt.Errorf("orchestrator: add quasar branch: %w", err)
    }

    if err := graph.AddEdge("nova", "quasar"); err != nil {
        return nil, fmt.Errorf("orchestrator: add nova edge: %w", err)
    }

    if err := graph.AddEdge("comet", "quasar"); err != nil {
        return nil, fmt.Errorf("orchestrator: add comet edge: %w", err)
    }

    options := []compose.GraphCompileOption{compose.WithMaxRunSteps(maxGraphSteps)}
    if orchestrator.CheckPointStore != nil {
        options = append(options, compose.WithCheckPointStore(orchestrator.CheckPointStore))
    }

    return graph.Compile(ctx, options...)
}

type turnScope struct {
    run     *agentcore.RunContext
    outcome *turnOutcome
}

type turnScopeKey struct{}

func withTurnScope(ctx context.Context, scope *turnScope) context.Context {
    return context.WithValue(ctx, turnScopeKey{}, scope)
}

func turnScopeFrom(ctx context.Context) (*turnScope, bool) {
    scope, ok := ctx.Value(turnScopeKey{}).(*turnScope)
    return scope, ok
}

var nodeAgentLabels = map[string]string{"quasar": "supervisor", "nova": "analyzer", "comet": "executor"}

// ctx: node context carrying the turn scope.
// runner: the runner of the agent this node runs.
// node: node name, part of the agent's own checkpoint id.
// state: graph state; nil when the node is resumed after a pause.
// messages: input for a fresh run; ignored when resuming.
func (orchestrator *Orchestrator) runAgentStep(ctx context.Context, runner *adk.Runner, node string, state *agentcore.TurnState, messages []adk.Message) (*agentcore.TurnState, string, error) {
    scope, ok := turnScopeFrom(ctx)
    if !ok {
        return state, "", errors.New("orchestrator: no turn scope bound to this node")
    }

    step := agentcore.AgentStep{Runner: runner, Node: node, Run: scope.run, Checkpoints: orchestrator.CheckPointStore, Read: orchestrator.eventReader(scope)}
    result, err := step.Execute(ctx, state, messages)
    return result, scope.outcome.finalOf(nodeAgentLabels[node]), err
}

func (orchestrator *Orchestrator) eventReader(scope *turnScope) agentcore.EventReader {
    topic := ChatStreamTopic(scope.run.ChatID)
    return func(events *adk.AsyncIterator[*adk.AgentEvent]) (*agentcore.PausedAt, error) {
        interrupt, err := orchestrator.readEvents(topic, events, scope.outcome)
        if err != nil || interrupt == nil {
            return nil, err
        }

        return &agentcore.PausedAt{InterruptID: interrupt.ID, Payload: interrupt.payload}, nil
    }
}
