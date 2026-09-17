package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/horizonlabs/pulsarfi-backend/src/contracts"
	dbmodel "github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
)

func init() {
	// *orchestratorTurn — the pointer, matching NewGraph[*orchestratorTurn,
	// *orchestratorTurn] exactly. Registering the bare value type here (as
	// this used to) silently mismatches what actually flows through the
	// graph, and gob restores a nil pointer from the checkpoint instead of
	// the real turn — confirmed live via a panic at the very first field
	// access after a needs_input resume, not theory.
	schema.RegisterName[*orchestratorTurn]("orchestrator_turn")
	schema.RegisterName[routeDecision]("orchestrator_route_decision")
	schema.RegisterName[NovaVerdict]("orchestrator_nova_verdict")
	schema.RegisterName[toolCallResult]("orchestrator_tool_call_result")
	schema.RegisterName[contracts.CardContract]("card_contract")
	schema.RegisterName[contracts.IntakeField]("intake_field")
}

// Orchestrator compiles and runs the Quasar/Nova/Comet coordination graph:
//
//	START -> route -> [needs_input pause] -> analyze -> reply -> [arm/execute pause] -> execute -> END
//
// Both pauses use Eino's native interrupt/resume (github.com/cloudwego/eino
// v0.9.15, compose/interrupt.go + compose/resume.go): `route` interrupts
// itself manually via compose.StatefulInterrupt when required trade
// parameters are missing; `execute` is gated by an engine-level
// compose.WithInterruptBeforeNodes compile option, since a real trade can
// never fire before the wallet arms and approves an allowance on a later,
// separate HTTP call. See docs/plans/quasar-clean-routing-scalp-refactor.md
// §3.6 for the full design.
type Orchestrator struct {
	Tasks        *repository.AgentTaskRepository
	SubTasks     *repository.AgentSubTaskRepository
	ChatMessages *repository.AgentChatMessageRepository
	Chain        OrchestratorChainClient

	RouteModel model.BaseChatModel
	ReplyModel model.BaseChatModel
	Analyzer   adk.Agent
	Executor   adk.Agent

	RouteModelName    string
	ReplyModelName    string
	AnalyzerModelName string
	ExecutorModelName string

	// Stocks is kept only for constructor-compatibility with agent_registry.go's
	// existing wiring — orchestrator_workflow_service.go's node functions must
	// never call it directly (docs/plans/quasar-clean-routing-scalp-refactor.md
	// §2 hard requirement). Ticker/catalog lookups belong to Nova's own
	// verify_ticker tool, never to Quasar's routing/orchestration code.
	Stocks StockCatalog

	CheckPointStore compose.CheckPointStore

	runnable compose.Runnable[*orchestratorTurn, *orchestratorTurn]
}

// OrchestratorChainClient defines the narrow on-chain interface required
// by the orchestrator turn.
type OrchestratorChainClient interface {
	CreateTask(ctx context.Context, owner string, isActionable bool, summary string, promptHash [32]byte) (onChainTaskID uint64, err error)
	MarkActionable(ctx context.Context, onChainTaskID uint64) error
	ChainSubTaskRecorder
}

// StockCatalog is the orchestrator's narrow view of the stock catalog. Not
// used by any node function — see the Stocks field comment above.
type StockCatalog interface {
	FindByTickerOrIdxTicker(ctx context.Context, ticker string) (dbmodel.Stock, bool, error)
	FindMarketReady(ctx context.Context) ([]dbmodel.Stock, error)
}

// NewOrchestrator compiles the Eino graph and binds all nodes and edges.
func NewOrchestrator(ctx context.Context, o *Orchestrator) (*Orchestrator, error) {
	g := compose.NewGraph[*orchestratorTurn, *orchestratorTurn]()

	// understand turns the raw message into a structured routeDecision (the
	// LLM call + JSON parse, and the needs_input pause when required fields
	// are missing — all in one node, on purpose: a manual
	// compose.StatefulInterrupt must be recovered by the SAME node that
	// raised it via compose.GetInterruptState/GetResumeContext, Eino does
	// not thread turn state across a plain edge into a different node the
	// way it does for an engine-level compile-option pause like execute's
	// below. Direction-deciding ("route") is not a separate lambda node —
	// it is the commitBranch just after commit, Eino's own branch
	// primitive, which cannot itself do work by construction (it only ever
	// returns a node name). commit is the one node that does work: opening
	// the Task, once understand has let a settled decision through
	// (docs/plans/quasar-clean-routing-scalp-refactor.md — understand ->
	// JSON -> object -> thrown around).
	if err := g.AddLambdaNode("understand", compose.InvokableLambda(o.runUnderstand)); err != nil {
		return nil, fmt.Errorf("orchestrator: add understand node: %w", err)
	}
	if err := g.AddLambdaNode("commit", compose.InvokableLambda(o.runCommit)); err != nil {
		return nil, fmt.Errorf("orchestrator: add commit node: %w", err)
	}
	if err := g.AddLambdaNode("analyze", compose.InvokableLambda(o.runAnalyze)); err != nil {
		return nil, fmt.Errorf("orchestrator: add analyze node: %w", err)
	}
	if err := g.AddLambdaNode("execute", compose.InvokableLambda(o.runExecute)); err != nil {
		return nil, fmt.Errorf("orchestrator: add execute node: %w", err)
	}
	if err := g.AddLambdaNode("reply", compose.InvokableLambda(o.runReply)); err != nil {
		return nil, fmt.Errorf("orchestrator: add reply node: %w", err)
	}

	if err := g.AddEdge(compose.START, "understand"); err != nil {
		return nil, fmt.Errorf("orchestrator: add start edge: %w", err)
	}
	if err := g.AddEdge("understand", "commit"); err != nil {
		return nil, fmt.Errorf("orchestrator: add understand->commit edge: %w", err)
	}

	// commit decides where this turn goes after it — executor_only is
	// reachable only for swing/investment shapes with Nova explicitly opted
	// out (instructions.go) — every actionable path (executor_only
	// included, any shape) always passes through "reply" first, so the Arm
	// Card is shown before the arm/execute pause, never after it. Only
	// analyzer_only/analyzer_then_executor detour through "analyze" first;
	// executor_only goes straight to reply since there is no Nova step to
	// wait on.
	commitBranch := compose.NewGraphBranch(func(_ context.Context, in *orchestratorTurn) (string, error) {
		switch in.Decision.Path {
		case pathAnalyzerOnly, pathAnalyzerThenExecutor:
			return "analyze", nil
		default:
			return "reply", nil
		}
	}, map[string]bool{"analyze": true, "reply": true})
	if err := g.AddBranch("commit", commitBranch); err != nil {
		return nil, fmt.Errorf("orchestrator: add commit branch: %w", err)
	}

	if err := g.AddEdge("analyze", "reply"); err != nil {
		return nil, fmt.Errorf("orchestrator: add analyze->reply edge: %w", err)
	}

	// reply always runs before execute for both executable paths, so the
	// Arm Card (and Nova's entry/exit read, for analyzer_then_executor) is
	// on the chat before the pause below ever engages.
	replyBranch := compose.NewGraphBranch(func(_ context.Context, in *orchestratorTurn) (string, error) {
		isExecutablePath := in.Decision.Path == pathAnalyzerThenExecutor || in.Decision.Path == pathExecutorOnly
		if in.Decision.IsActionable && isExecutablePath && in.ExecutorReply == "" {
			return "execute", nil
		}
		return compose.END, nil
	}, map[string]bool{"execute": true, compose.END: true})
	if err := g.AddBranch("reply", replyBranch); err != nil {
		return nil, fmt.Errorf("orchestrator: add reply branch: %w", err)
	}

	if err := g.AddEdge("execute", compose.END); err != nil {
		return nil, fmt.Errorf("orchestrator: add execute->end edge: %w", err)
	}

	var compileOpts []compose.GraphCompileOption
	compileOpts = append(compileOpts, compose.WithInterruptBeforeNodes([]string{"execute"}))
	if o.CheckPointStore != nil {
		compileOpts = append(compileOpts, compose.WithCheckPointStore(o.CheckPointStore))
	}

	runnable, err := g.Compile(ctx, compileOpts...)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: compile graph: %w", err)
	}
	o.runnable = runnable
	return o, nil
}

type AgentEventCallbacks struct {
	OnSubTask        func(dbmodel.AgentSubTask)
	OnSubTaskStarted func(agentName, stepName, label string)
	// OnSubTaskFailed fires when a tool call inside a step is caught
	// non-fatally by WrapToolGraceful — never persisted (same as
	// OnSubTaskStarted), purely an ephemeral live signal so the UI can show
	// a real failed state instead of guessing one from a second
	// OnSubTaskStarted for the same step ever arriving.
	OnSubTaskFailed func(agentName, stepName, reason string)
	OnTextDelta     func(string)
	OnToolCall      func(agentName, toolName, phase string)
	OnThinking      func(agentName, delta string)
	OnFinalizing    func()
}

type OrchestratorInput struct {
	ChatID          uuid.UUID
	Wallet          string
	RawPrompt       string
	Messages        []*schema.Message
	SourceMessageID *int64

	AgentEventCallbacks
}

type OrchestratorResult struct {
	TaskID           int64
	OnChainTaskID    *int64
	Reply            string
	ContentType      string // "text" | "chart" | "news" | "workflow_card"
	UIComponent      string
	UIProps          json.RawMessage
	Decision         routeDecision
	PendingQuestions []contracts.IntakeField
	ResolvedTicker   string
	Locale           string
}

// Run executes one turn of the graph from the start (a brand-new message,
// or a message answering a needs_input question — see runUnderstand, which
// detects and handles the resume case itself via compose.GetResumeContext).
func (o *Orchestrator) Run(ctx context.Context, in OrchestratorInput) (OrchestratorResult, error) {
	turn := &orchestratorTurn{
		ChatID:          in.ChatID,
		Wallet:          in.Wallet,
		RawPrompt:       in.RawPrompt,
		Messages:        in.Messages,
		SourceMessageID: in.SourceMessageID,
	}
	turn.bindCallbacks(in.AgentEventCallbacks)
	var snapshot *orchestratorTurn
	turn.onLatestSnapshot = func(t *orchestratorTurn) { snapshot = t }
	ctx = withTurnCallbacks(ctx, turnCallbacks{AgentEventCallbacks: in.AgentEventCallbacks, onLatestSnapshot: turn.onLatestSnapshot})

	var invokeOpts []compose.Option
	if in.ChatID != uuid.Nil {
		invokeOpts = append(invokeOpts, compose.WithCheckPointID(in.ChatID.String()))
	}

	out, err := o.runnable.Invoke(ctx, turn, invokeOpts...)
	if err != nil {
		return o.handleInvokeErr(ctx, latestTurn(snapshot, turn), in.ChatID, err)
	}
	return o.formatResult(out), nil
}

// latestTurn prefers the snapshot runReply captured (the real, up-to-date
// state) over the turn originally passed into Invoke, which a resumed
// node may never have actually touched (see onLatestSnapshot's comment).
func latestTurn(snapshot, original *orchestratorTurn) *orchestratorTurn {
	if snapshot != nil {
		return snapshot
	}
	return original
}

// ResumeRoute resumes a graph run that paused on needs_input, feeding the
// user's next chat message in as the answer. Task-layer/HTTP code never
// builds prompts or calls agents itself — that stays inside runUnderstand /
// decideRouteWithAnswer, which this only triggers.
func (o *Orchestrator) ResumeRoute(ctx context.Context, chatID uuid.UUID, interruptID string, answer string, wallet string, sourceMessageID *int64, events AgentEventCallbacks) (OrchestratorResult, error) {
	turn := &orchestratorTurn{
		ChatID:          chatID,
		Wallet:          wallet,
		RawPrompt:       answer,
		SourceMessageID: sourceMessageID,
	}
	turn.bindCallbacks(events)
	var snapshot *orchestratorTurn
	turn.onLatestSnapshot = func(t *orchestratorTurn) { snapshot = t }

	rCtx := compose.ResumeWithData(ctx, interruptID, answer)
	rCtx = withTurnCallbacks(rCtx, turnCallbacks{AgentEventCallbacks: events, onLatestSnapshot: turn.onLatestSnapshot})
	out, err := o.runnable.Invoke(rCtx, turn, compose.WithCheckPointID(chatID.String()))
	if err != nil {
		return o.handleInvokeErr(ctx, latestTurn(snapshot, turn), chatID, err)
	}
	return o.formatResult(out), nil
}

// ResumeExecute resumes the graph at the execute node once the Task layer
// has confirmed on-chain arm + allowance. It carries only a bare "go"
// signal — no ticker, amount, or verdict content, all of which already
// live on the checkpointed turn from the earlier route/analyze/reply
// portion of this same run (docs/plans/quasar-clean-routing-scalp-refactor.md
// §3.6). This is the only orchestrator entrypoint TaskService.ExecuteTask
// calls; it never builds a prompt or invokes Comet itself.
func (o *Orchestrator) ResumeExecute(ctx context.Context, taskID int64, wallet string) (ExecuteTaskResult, error) {
	task, found, err := o.Tasks.FindByID(ctx, taskID)
	if err != nil || !found {
		return ExecuteTaskResult{}, fmt.Errorf("orchestrator: task not found: %d", taskID)
	}

	chatID := o.resolveChatID(ctx, task)

	var interruptID string
	if chatID != uuid.Nil {
		if cpStore, ok := o.CheckPointStore.(ExtendedCheckPointStore); ok {
			interruptID, _, _ = cpStore.GetInterruptID(ctx, chatID.String())
		}
	}

	rCtx := ctx
	if interruptID != "" {
		rCtx = compose.ResumeWithData(ctx, interruptID, "go")
	}

	var invokeOpts []compose.Option
	if chatID != uuid.Nil {
		invokeOpts = append(invokeOpts, compose.WithCheckPointID(chatID.String()))
	}

	// A fresh, minimal turn — never nil. Whatever route/analyze/reply
	// already produced earlier in this same checkpointed run (Decision,
	// NovaVerdict, AnalyzerReply, etc.) is restored by the engine; this
	// only needs to carry the identifiers execute itself depends on when
	// runCtx isn't already present (runExecute rebuilds its own recorder
	// in that case).
	resumeTurn := &orchestratorTurn{
		ChatID:          chatID,
		Wallet:          wallet,
		SourceMessageID: task.SourceMessageID,
		TaskID:          task.ID,
		OnChainTaskID:   task.OnChainTaskID,
	}

	out, err := o.runnable.Invoke(rCtx, resumeTurn, invokeOpts...)
	if err != nil {
		return ExecuteTaskResult{}, fmt.Errorf("orchestrator: resume execute failed: %w", err)
	}

	if chatID != uuid.Nil {
		if cpStore, ok := o.CheckPointStore.(ExtendedCheckPointStore); ok {
			_ = cpStore.Delete(ctx, chatID.String())
		}
	}

	reply := ""
	if out != nil {
		reply = out.ExecutorReply
		if reply == "" {
			reply = out.FinalReply
		}
	}

	return ExecuteTaskResult{TaskID: taskID, Status: "executed", Reply: reply}, nil
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

// handleInvokeErr distinguishes an expected pause (needs_input, or the
// arm/execute gate) from a genuine failure. On a pause, it records which
// interrupt to target on the next resume and returns a normal-looking
// result built from whatever route/analyze/reply already produced and
// streamed via callbacks before the pause engaged.
func (o *Orchestrator) handleInvokeErr(ctx context.Context, turn *orchestratorTurn, chatID uuid.UUID, err error) (OrchestratorResult, error) {
	info, isInterrupt := compose.ExtractInterruptInfo(err)
	if !isInterrupt || len(info.InterruptContexts) == 0 {
		return OrchestratorResult{}, err
	}

	if chatID != uuid.Nil {
		if cpStore, ok := o.CheckPointStore.(ExtendedCheckPointStore); ok {
			_ = cpStore.SetInterruptID(ctx, chatID.String(), info.InterruptContexts[0].ID)
		}
	}

	contentType, uiProps := classifyReply(turn)
	uiComponent := ""
	if contentType == "workflow_card" {
		uiComponent = "clarifying_questions"
	}
	return OrchestratorResult{
		TaskID:           turn.TaskID,
		OnChainTaskID:    turn.OnChainTaskID,
		Reply:            turn.FinalReply,
		ContentType:      contentType,
		UIComponent:      uiComponent,
		UIProps:          uiProps,
		Decision:         turn.Decision,
		PendingQuestions: turn.PendingQuestions,
		ResolvedTicker:   turn.ResolvedTicker,
		Locale:           turn.Locale,
	}, nil
}

func (o *Orchestrator) formatResult(out *orchestratorTurn) OrchestratorResult {
	if out == nil {
		return OrchestratorResult{}
	}
	contentType, uiProps := classifyReply(out)
	uiComponent := ""
	if contentType == "workflow_card" {
		uiComponent = "clarifying_questions"
	}
	return OrchestratorResult{
		TaskID:           out.TaskID,
		OnChainTaskID:    out.OnChainTaskID,
		Reply:            out.FinalReply,
		ContentType:      contentType,
		UIComponent:      uiComponent,
		UIProps:          uiProps,
		Decision:         out.Decision,
		PendingQuestions: out.PendingQuestions,
		ResolvedTicker:   out.ResolvedTicker,
		Locale:           out.Locale,
	}
}

// NovaVerdict is Nova's structured, machine-parsed conclusion — the object
// that flows from analyze to execute via the turn itself (normal graph data
// flow, restored by the checkpoint across the arm/execute pause), never via
// ResumeWithData. See parseNovaVerdict in orchestrator_workflow_service.go.
type NovaVerdict struct {
	Tradeable  bool   `json:"tradeable"`
	EntryPrice string `json:"entry_price"`
	ExitPrice  string `json:"exit_price,omitempty"`
	Confidence string `json:"confidence"`
	Reasoning  string `json:"reasoning"`
}

// orchestratorTurn is the single input/output type threaded through every
// node in the graph.
type orchestratorTurn struct {
	ChatID          uuid.UUID
	Wallet          string
	RawPrompt       string
	Messages        []*schema.Message
	SourceMessageID *int64
	Locale          string

	Decision routeDecision

	TaskID        int64
	OnChainTaskID *int64
	runCtx        *RunContext

	NovaVerdict       *NovaVerdict
	AnalyzerReply     string
	AnalyzerToolCalls []toolCallResult
	ExecutorReply     string
	FinalReply        string
	PendingQuestions  []contracts.IntakeField
	ResolvedTicker    string
	TickerProblem     string

	onSubTask        func(dbmodel.AgentSubTask)
	onSubTaskStarted func(agentName, stepName, label string)
	onSubTaskFailed  func(agentName, stepName, reason string)
	onTextDelta      func(string)
	onToolCall       func(agentName, toolName, phase string)
	onThinking       func(agentName, delta string)
	onFinalizing     func()

	// onLatestSnapshot is internal plumbing, not part of AgentEventCallbacks
	// — never set via bindCallbacks. runReply calls it with itself right
	// after finishing, so Run/ResumeRoute can recover the real, fully
	// up-to-date turn (TaskID, FinalReply, etc.) if a LATER node (execute)
	// interrupts afterward in the same Invoke call. This exists because a
	// resumed node's own `turn` parameter is unreliable (see runUnderstand's
	// comment) — a node that recovers state via GetInterruptState works
	// with a *different* pointer than the one the caller originally passed
	// into Invoke, so mutations on it are invisible to the caller unless
	// surfaced through a side channel like this one.
	onLatestSnapshot func(*orchestratorTurn)
}

func (t *orchestratorTurn) bindCallbacks(cb AgentEventCallbacks) {
	t.onSubTask = cb.OnSubTask
	t.onSubTaskStarted = cb.OnSubTaskStarted
	t.onSubTaskFailed = cb.OnSubTaskFailed
	t.onTextDelta = cb.OnTextDelta
	t.onToolCall = cb.OnToolCall
	t.onThinking = cb.OnThinking
	t.onFinalizing = cb.OnFinalizing
}

// turnCallbacks bundles every func-typed field a node must re-attach to a
// turn recovered via GetInterruptState — gob silently drops func fields on
// decode (see schema.RegisterName's own doc: "Functions and channels are
// not supported and will be ignored"), so a recovered turn's callbacks are
// always nil. These are threaded via context instead, alongside ctx itself,
// since ctx (unlike the node's own turn parameter) IS reliably passed
// through to a resumed node.
type turnCallbacks struct {
	AgentEventCallbacks
	onLatestSnapshot func(*orchestratorTurn)
}

type turnCallbacksKey struct{}

func withTurnCallbacks(ctx context.Context, cb turnCallbacks) context.Context {
	return context.WithValue(ctx, turnCallbacksKey{}, cb)
}

func turnCallbacksFrom(ctx context.Context) (turnCallbacks, bool) {
	cb, ok := ctx.Value(turnCallbacksKey{}).(turnCallbacks)
	return cb, ok
}

type routePath string

const (
	pathNone                 routePath = "none"
	pathNeedsInput           routePath = "needs_input"
	pathAnalyzerOnly         routePath = "analyzer_only"
	pathExecutorOnly         routePath = "executor_only"
	pathAnalyzerThenExecutor routePath = "analyzer_then_executor"
)

type routeDecision struct {
	Path               routePath               `json:"path"`
	IsActionable       bool                    `json:"is_actionable"`
	Summary            string                  `json:"summary"`
	Label              string                  `json:"label"`
	RequestForAnalyzer string                  `json:"request_for_analyzer,omitempty"`
	RequestForExecutor string                  `json:"request_for_executor,omitempty"`
	Shape              string                  `json:"shape,omitempty"`
	Side               string                  `json:"side,omitempty"`
	MentionedTicker    string                  `json:"mentioned_ticker,omitempty"`
	Unanswered         []string                `json:"unanswered,omitempty"`
	Questions          []contracts.IntakeField `json:"questions,omitempty"`
	BudgetIDRX         string                  `json:"budget_idrx,omitempty"`
	SellAmount         string                  `json:"sell_amount,omitempty"`
	Card               *contracts.CardContract `json:"card,omitempty"`
}
