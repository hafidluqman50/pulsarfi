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

// Orchestrator executes the Quasar/Nova/Comet coordination graph:
//
//	START -> route -> (analyze and/or execute) -> reply -> END
//
// Built with CloudWeGo Eino (github.com/cloudwego/eino v0.9.15).
type Orchestrator struct {
	Tasks    *repository.AgentTaskRepository
	SubTasks *repository.AgentSubTaskRepository
	Chain    OrchestratorChainClient

	RouteModel model.BaseChatModel
	ReplyModel model.BaseChatModel
	Analyzer   adk.Agent
	Executor   adk.Agent

	RouteModelName    string
	ReplyModelName    string
	AnalyzerModelName string
	ExecutorModelName string

	AnalyzerIntake func(IntakeContext) []IntakeField
	ExecutorIntake func(IntakeContext) []IntakeField

	Stocks StockCatalog

	CheckPointStore compose.CheckPointStore

	runnable compose.Runnable[*orchestratorTurn, *orchestratorTurn]
}

// OrchestratorChainClient defines the narrow on-chain interface required
// by the orchestrator turn.
type OrchestratorChainClient interface {
	CreateTask(ctx context.Context, owner string, isActionable bool, summary string, promptHash [32]byte) (onChainTaskID uint64, err error)
	ChainSubTaskRecorder
}

// StockCatalog is the orchestrator's narrow view of the stock catalog.
type StockCatalog interface {
	FindByTickerOrIdxTicker(ctx context.Context, ticker string) (dbmodel.Stock, bool, error)
	FindMarketReady(ctx context.Context) ([]dbmodel.Stock, error)
}

// IntakeFor returns both roles' declared requirements for this shape, in a
// fixed order (Executor first: ticker/side/caps are what a user expects to
// be asked first), for CompileIntake to merge and de-duplicate.
func (o *Orchestrator) IntakeFor(in IntakeContext) [][]IntakeField {
	var sets [][]IntakeField
	sets = append(sets, GetShapeFieldSet())
	if o.ExecutorIntake != nil {
		sets = append(sets, o.ExecutorIntake(in))
	}
	if o.AnalyzerIntake != nil {
		sets = append(sets, o.AnalyzerIntake(in))
	}
	return sets
}

// NewOrchestrator compiles the Eino graph and binds all nodes and edges.
func NewOrchestrator(ctx context.Context, o *Orchestrator) (*Orchestrator, error) {
	g := compose.NewGraph[*orchestratorTurn, *orchestratorTurn]()

	if err := g.AddLambdaNode("route", compose.InvokableLambda(o.runRoute)); err != nil {
		return nil, fmt.Errorf("orchestrator: add route node: %w", err)
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

	if err := g.AddEdge(compose.START, "route"); err != nil {
		return nil, fmt.Errorf("orchestrator: add start edge: %w", err)
	}

	routeBranch := compose.NewGraphBranch(func(_ context.Context, in *orchestratorTurn) (string, error) {
		switch in.Decision.Path {
		case pathAnalyzerOnly, pathAnalyzerThenExecutor:
			return "analyze", nil
		case pathExecutorOnly, pathNeedsInput:
			return "reply", nil
		default:
			return "reply", nil
		}
	}, map[string]bool{"analyze": true, "reply": true})
	if err := g.AddBranch("route", routeBranch); err != nil {
		return nil, fmt.Errorf("orchestrator: add route branch: %w", err)
	}

	if err := g.AddEdge("analyze", "reply"); err != nil {
		return nil, fmt.Errorf("orchestrator: add analyze->reply edge: %w", err)
	}

	if err := g.AddEdge("execute", "reply"); err != nil {
		return nil, fmt.Errorf("orchestrator: add execute->reply edge: %w", err)
	}
	if err := g.AddEdge("reply", compose.END); err != nil {
		return nil, fmt.Errorf("orchestrator: add reply->end edge: %w", err)
	}

	var compileOpts []compose.GraphCompileOption
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
	OnTextDelta      func(string)
	OnToolCall       func(agentName, toolName, phase string)
	OnThinking       func(agentName, delta string)
	OnFinalizing     func()
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
	PendingQuestions []IntakeField
	ResolvedTicker   string
	Locale           string
}

// Run executes one full turn: route -> (analyze and/or execute) -> reply.
func (o *Orchestrator) Run(ctx context.Context, in OrchestratorInput) (OrchestratorResult, error) {
	turn := &orchestratorTurn{
		Wallet:              in.Wallet,
		RawPrompt:           in.RawPrompt,
		Messages:            in.Messages,
		SourceMessageID:     in.SourceMessageID,
		AgentEventCallbacks: in.AgentEventCallbacks,
	}

	var invokeOpts []compose.Option
	if in.ChatID != uuid.Nil {
		invokeOpts = append(invokeOpts, compose.WithCheckPointID(in.ChatID.String()))
	}

	out, err := o.runnable.Invoke(ctx, turn, invokeOpts...)
	if err != nil {
		return OrchestratorResult{}, err
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
	}, nil
}

type orchestratorTurn struct {
	Wallet          string
	RawPrompt       string
	Messages        []*schema.Message
	SourceMessageID *int64
	Locale          string

	Decision routeDecision

	TaskID        int64
	OnChainTaskID *int64
	runCtx        *RunContext // built once a Task is opened; nil for path "none"

	AnalyzerReply     string
	AnalyzerToolCalls []toolCallResult
	ExecutorReply     string
	FinalReply        string
	PendingQuestions  []IntakeField
	ResolvedTicker    string
	TickerProblem     string

	DemandedExecutionWithoutAllowance bool
	UnarmedTaskID                     int64

	AgentEventCallbacks
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
	Path               routePath `json:"path"`
	IsActionable       bool      `json:"is_actionable"`
	Summary            string    `json:"summary"`
	Label              string    `json:"label"`
	RequestForAnalyzer string    `json:"request_for_analyzer,omitempty"`
	RequestForExecutor string    `json:"request_for_executor,omitempty"`
	Shape              string                  `json:"shape,omitempty"`
	Side               string                  `json:"side,omitempty"`
	MentionedTicker    string                  `json:"mentioned_ticker,omitempty"`
	Unanswered         []string                `json:"unanswered,omitempty"`
	Questions          []IntakeField           `json:"questions,omitempty"`
	BudgetIDRX         string                  `json:"budget_idrx,omitempty"`
	Card               *contracts.CardContract `json:"card,omitempty"`
}
