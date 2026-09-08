package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/ethereum/go-ethereum/crypto"
	dbmodel "github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
)

// wibLocation is a fixed UTC+7 offset, not time.LoadLocation("Asia/Jakarta")
// — WIB has no DST, so a fixed offset is both correct and avoids depending
// on the deploy environment having the IANA tzdata package installed at all.
var wibLocation = time.FixedZone("WIB", 7*60*60)

// currentTimeContext gives Quasar (route and reply both) the one thing
// neither call could otherwise ever know on its own: what time it actually
// is right now. Flagged live — asked "hari ini hari apa" with nothing in
// any prompt ever answering it, old design included (GlobalInstructions
// never injected real time either). WIB specifically, since IDX market
// hours and every "today"/"this week" question in this product are
// Indonesia-local, not UTC.
func currentTimeContext() string {
	now := time.Now().In(wibLocation)
	return fmt.Sprintf(
		"Current date and time: %s WIB (UTC+7). Also UTC: %s.",
		now.Format("Monday, 02 January 2006, 15:04"),
		now.UTC().Format(time.RFC3339),
	)
}

// Orchestrator is the single, explicit compose.Graph that wires Quasar,
// Nova, and Comet together — replacing the old pattern where Nova/Comet
// were called from inside a Quasar tool-call closure
// (supervisor/tools_service.go), invisible to any graph. Nova and Comet are
// untouched: still built by analyzer.New/executor.New, still full adk.Agent
// with their own internal tool-use ReAct loop. Quasar is NOT an adk.Agent
// here — it is two plain LLM calls (route, reply) owned directly by this
// graph, specifically so the reply step can call Stream() itself instead of
// going through adk.ChatModelAgent's hardcoded Generate-only ReAct node
// (confirmed by reading eino v0.9.15/v0.9.19/v0.10.0-alpha.31's adk/chatmodel.go
// — none of them ever build their ChatModel node as a StreamableLambda).
//
// Deliberately not wired to task_service.go/agent_registry.go/supervisor/
// yet — this is the graph itself, self-contained, reviewed on its own
// before anything else is cut over to call it.
//
// Explicitly out of scope for this first pass, not silently dropped:
//   - needs_input / multi-turn Task continuation (every call opens a fresh
//     Task; rebinding to an existing "waiting for an answer" Task is not
//     handled here yet)
//   - quick/deep Analyzer model tiering (one Analyzer instance, not two)
//   - chart/news content_type classification for the HTTP response layer
//   - retrying a failed on-chain CreateTask itself (only agent_sub_tasks
//     batches have a retry path today, via SubTaskRetryService)
type Orchestrator struct {
	Tasks    *repository.AgentTaskRepository
	SubTasks *repository.AgentSubTaskRepository
	Chain    OrchestratorChainClient

	RouteModel model.BaseChatModel
	ReplyModel model.BaseChatModel
	Analyzer   adk.Agent
	Executor   adk.Agent

	runnable compose.Runnable[*orchestratorTurn, *orchestratorTurn]
}

// OrchestratorChainClient is deliberately narrow (ISP) — just enough for one
// chat turn to open a Task on-chain the instant it's recognized and batch
// its Sub Task rows. GrantTradePermission/CancelTask/pause/resume stay
// separate, human-triggered Task-lifecycle actions outside a single turn's
// concern, not this graph's job.
type OrchestratorChainClient interface {
	CreateTask(ctx context.Context, owner string, isActionable bool, summary string, promptHash [32]byte) (onChainTaskID uint64, err error)
	RecordSubTasks(ctx context.Context, onChainTaskID uint64, rows []dbmodel.AgentSubTask) (txHash string, err error)
}

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

	// Quasar's own routing decision, structural now, not an LLM tool call
	// it happens to make — this branch IS the "handoff" this graph exists
	// to make real, matching how a LangGraph supervisor's handoff tool
	// becomes a real Command(goto:...) node transition rather than a bare
	// function call (see docs/plans/agent-orchestration-graph-rebuild.md
	// §4, validated against CATAT/SERVER's own createSupervisor usage).
	routeBranch := compose.NewGraphBranch(func(_ context.Context, in *orchestratorTurn) (string, error) {
		switch in.Decision.Path {
		case pathAnalyzerOnly, pathAnalyzerThenExecutor:
			return "analyze", nil
		case pathExecutorOnly:
			return "execute", nil
		default:
			return "reply", nil
		}
	}, map[string]bool{"analyze": true, "execute": true, "reply": true})
	if err := g.AddBranch("route", routeBranch); err != nil {
		return nil, fmt.Errorf("orchestrator: add route branch: %w", err)
	}

	// The analyzer-then-executor gate — "never call Executor when Analyzer's
	// conclusion states the condition was not met" — enforced here as a real
	// branch condition instead of trusting an LLM's own judgment not to call
	// a tool.
	gateBranch := compose.NewGraphBranch(func(_ context.Context, in *orchestratorTurn) (string, error) {
		if in.Decision.Path == pathAnalyzerThenExecutor && analyzerConfirmed(in.AnalyzerReply) {
			return "execute", nil
		}
		return "reply", nil
	}, map[string]bool{"execute": true, "reply": true})
	if err := g.AddBranch("analyze", gateBranch); err != nil {
		return nil, fmt.Errorf("orchestrator: add gate branch: %w", err)
	}

	if err := g.AddEdge("execute", "reply"); err != nil {
		return nil, fmt.Errorf("orchestrator: add execute->reply edge: %w", err)
	}
	if err := g.AddEdge("reply", compose.END); err != nil {
		return nil, fmt.Errorf("orchestrator: add reply->end edge: %w", err)
	}

	runnable, err := g.Compile(ctx)
	if err != nil {
		return nil, fmt.Errorf("orchestrator: compile graph: %w", err)
	}
	o.runnable = runnable
	return o, nil
}

// OrchestratorInput is one chat turn's worth of context — deliberately
// plain data, no framework types, so a caller (eventually replacing
// task_service.go's runForMessage) can build this from whatever it already
// has without depending on RunContext directly.
// AgentEventCallbacks bundles every live-progress hook one chat turn can
// fire — grouped into one struct (rather than four separate positional
// function parameters threaded through HandleChatMessage/RetryLastMessage/
// runForMessage/runAgentOverSocket) once a fourth callback (OnToolCall,
// v2.8) made the positional-parameter list unwieldy.
type AgentEventCallbacks struct {
	OnSubTask        func(dbmodel.AgentSubTask)
	OnSubTaskStarted func(agentName, stepName, label string)
	OnTextDelta      func(string)
	// OnToolCall fires live around each individual tool call *inside*
	// Nova's/Comet's own ReAct loop (web_search starting, get_stock_chart
	// finishing, etc.) — real progress, not the generic "Nova sedang
	// memproses langkah ini..." placeholder that was all the UI had before
	// this (flagged live: "MANA ISI PROSES THINKING SI NOVA?"). phase is
	// "start" or "end". Distinct from OnSubTaskStarted, which only fires
	// once per Sub Task (the whole gather_evidence/decide step), not once
	// per tool call within it.
	OnToolCall func(agentName, toolName, phase string)
}

type OrchestratorInput struct {
	Wallet          string
	RawPrompt       string
	Messages        []*schema.Message // full chat history, including the current message
	SourceMessageID *int64

	AgentEventCallbacks
}

type OrchestratorResult struct {
	TaskID        int64
	OnChainTaskID *int64
	Reply         string
	ContentType   string // "text" | "chart" | "news"
	UIProps       json.RawMessage
}

// Run executes one full turn: route -> (analyze and/or execute) -> reply,
// then batches this turn's own Sub Task rows on-chain in one call if the
// Task already has an on-chain id by the time the turn finishes.
func (o *Orchestrator) Run(ctx context.Context, in OrchestratorInput) (OrchestratorResult, error) {
	turn := &orchestratorTurn{
		Wallet:              in.Wallet,
		RawPrompt:           in.RawPrompt,
		Messages:            in.Messages,
		SourceMessageID:     in.SourceMessageID,
		AgentEventCallbacks: in.AgentEventCallbacks,
	}

	out, err := o.runnable.Invoke(ctx, turn)
	if err != nil {
		return OrchestratorResult{}, err
	}

	if out.runCtx != nil && out.runCtx.OnChainTaskID != nil {
		rows := out.runCtx.Recorder.RowsSince(0)
		if len(rows) > 0 {
			onChainTaskID := uint64(*out.runCtx.OnChainTaskID)
			txHash, chainErr := o.Chain.RecordSubTasks(ctx, onChainTaskID, rows)
			if chainErr != nil {
				slog.ErrorContext(ctx, "orchestrator: recordSubTasks batch failed, leaving for retry", "task_id", out.TaskID, "error", chainErr)
			} else {
				ids := make([]int64, len(rows))
				for i, row := range rows {
					ids[i] = row.ID
				}
				if err := o.SubTasks.MarkRecordedOnChain(ctx, ids, txHash); err != nil {
					slog.ErrorContext(ctx, "orchestrator: confirmed on-chain but failed to mark locally", "task_id", out.TaskID, "error", err)
				}
			}
		}
	}

	contentType, uiProps := classifyReply(out)

	return OrchestratorResult{
		TaskID:        out.TaskID,
		OnChainTaskID: out.OnChainTaskID,
		Reply:         out.FinalReply,
		ContentType:   contentType,
		UIProps:       uiProps,
	}, nil
}

// toolCallResult mirrors one tool call Nova (or Comet) made internally —
// captured by runRoleAgent's own event drain, never by a merged
// "NestedToolCalls" concept the old supervisor-tool-call design needed
// (Nova is a graph node here, not a tool something else calls, so there is
// only ever one layer to look at).
type toolCallResult struct {
	ToolName string
	Result   string
}

// classifyReply restores the chart/news content_type classification that
// task_service.go's old buildWorkflowCard did before the v2.2 wiring
// deleted it (a real, user-visible regression — a chart/news request
// after that point rendered as a plain, uninformative text-only reply,
// found live: "gua nunggu chart gak ada ini"). Only ever looks at Nova's
// own tool calls — Comet's own tools (get_portfolio_holdings, submit_trade)
// never produce chart-shaped or news-shaped output.
func classifyReply(turn *orchestratorTurn) (contentType string, uiProps json.RawMessage) {
	var chartPayloads []json.RawMessage
	for _, tc := range turn.AnalyzerToolCalls {
		if tc.ToolName == "get_portfolio_snapshot" || tc.ToolName == "get_stock_chart" {
			chartPayloads = append(chartPayloads, json.RawMessage(tc.Result))
		}
	}
	if len(chartPayloads) == 1 {
		return "chart", chartPayloads[0]
	}
	if len(chartPayloads) > 1 {
		if payload, err := json.Marshal(chartPayloads); err == nil {
			return "chart", payload
		}
	}

	if evidence := extractNewsEvidence(turn.AnalyzerReply); len(evidence) > 0 {
		if payload, err := json.Marshal(evidence); err == nil {
			return "news", payload
		}
	}

	return "text", nil
}

// newsEvidenceItem mirrors one item of Nova's own "evidence" array (its own
// JSON contract, analyzer/instructions.go) — re-parsed here purely to
// surface it as its own citation card, never to re-derive or alter what
// Nova actually concluded.
type newsEvidenceItem struct {
	Source      string `json:"source"`
	URL         string `json:"url,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
	Excerpt     string `json:"excerpt,omitempty"`
	ImageURL    string `json:"image_url,omitempty"`
}

func extractNewsEvidence(raw string) []newsEvidenceItem {
	var parsed struct {
		Evidence []newsEvidenceItem `json:"evidence"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil
	}
	return parsed.Evidence
}

// orchestratorTurn is the payload threaded through every node — the same
// type in and out of every node, so the graph's edges/branches all trivially
// type-match. Real inter-node "state" lives in this struct's own fields,
// mutated node by node, rather than in a separate compose local-state
// object — simpler generics, same effect for a single-turn, non-concurrent
// run.
type orchestratorTurn struct {
	Wallet          string
	RawPrompt       string
	Messages        []*schema.Message
	SourceMessageID *int64

	Decision routeDecision

	TaskID        int64
	OnChainTaskID *int64
	runCtx        *RunContext // built once a Task is opened; nil for path "none"

	AnalyzerReply     string
	AnalyzerToolCalls []toolCallResult
	ExecutorReply     string
	FinalReply        string

	AgentEventCallbacks
}

type routePath string

const (
	pathNone                 routePath = "none"
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
}

// runRoute is Quasar's first call: read the prompt, decide whether this is
// pure conversation ("none") or a real Task, and if it's a Task, open it —
// both in Postgres and on-chain, immediately, regardless of is_actionable.
// createTask is bookkeeping only (no funds, no owner signature required, per
// agent-task-manager-rebuild.md §2/§3), so it must never wait on a human
// "Arm" action the way grantTradePermission legitimately does — that was
// the real bug in the old Arm-gated flow, not a missing feature.
func (o *Orchestrator) runRoute(ctx context.Context, turn *orchestratorTurn) (*orchestratorTurn, error) {
	messages := append([]*schema.Message{schema.SystemMessage(quasarRouteInstructions), schema.SystemMessage(currentTimeContext())}, turn.Messages...)
	response, err := o.RouteModel.Generate(ctx, messages)
	if err != nil {
		return turn, fmt.Errorf("orchestrator: route generate: %w", err)
	}

	decision, ok := parseRouteDecision(response.Content)
	if !ok {
		slog.WarnContext(ctx, "orchestrator: route decision did not parse, falling back to reply-only", "raw", response.Content)
		decision = routeDecision{Path: pathNone}
	}
	turn.Decision = decision

	if decision.Path == pathNone {
		return turn, nil
	}

	// On-chain is the source of truth — attempted BEFORE any Postgres write.
	// If this fails, the whole turn aborts right here: no agent_tasks row,
	// no recorder, nothing partially created. A Task existing in Postgres
	// with no on-chain counterpart is not a degraded state to tolerate and
	// retry later, it is not allowed to happen at all.
	onChainTaskID, chainErr := o.Chain.CreateTask(ctx, turn.Wallet, decision.IsActionable, decision.Summary, crypto.Keccak256Hash([]byte(turn.RawPrompt)))
	if chainErr != nil {
		return turn, fmt.Errorf("orchestrator: on-chain createTask failed, aborting turn (on-chain is the source of truth, no off-chain-only task is allowed): %w", chainErr)
	}
	onChainID := int64(onChainTaskID)

	task, err := o.Tasks.Create(ctx, repository.AgentTaskCreateInput{
		WalletAddress:   turn.Wallet,
		SourceMessageID: turn.SourceMessageID,
		IsActionable:    decision.IsActionable,
		RawPrompt:       &turn.RawPrompt,
		Summary:         decision.Summary,
	})
	if err != nil {
		return turn, fmt.Errorf("orchestrator: on-chain task %d created but Postgres insert failed: %w", onChainID, err)
	}
	turn.TaskID = task.ID
	turn.OnChainTaskID = &onChainID

	if err := o.Tasks.SetOnChainTaskID(ctx, task.ID, onChainID); err != nil {
		return turn, fmt.Errorf("orchestrator: persist on_chain_task_id: %w", err)
	}

	recorder, err := NewSubTaskRecorder(ctx, o.SubTasks, task.ID, decision.Summary, turn.Wallet)
	if err != nil {
		return turn, fmt.Errorf("orchestrator: build recorder: %w", err)
	}
	recorder.OnRecord = turn.OnSubTask

	runCtx := &RunContext{
		OnChainTaskID:    &onChainID,
		Wallet:           turn.Wallet,
		Recorder:         recorder,
		OnSubTaskStarted: turn.OnSubTaskStarted,
	}
	turn.runCtx = runCtx

	if turn.OnSubTaskStarted != nil {
		turn.OnSubTaskStarted("supervisor", "recognize_request", decision.Summary)
	}
	if _, err := recorder.Record(ctx, "supervisor", "recognize_request", "done", decision.Summary, decision.Summary, map[string]any{"is_actionable": decision.IsActionable}); err != nil {
		return turn, fmt.Errorf("orchestrator: record recognize_request: %w", err)
	}

	return turn, nil
}

// runAnalyze wraps Nova's own, unmodified ReAct run. gather_evidence is
// recorded here, by the caller, because analyzer/chart_service.go's own
// tools never call rc.Recorder.Record for it themselves (only Executor's
// submit_trade records its own steps internally) — matching exactly where
// the old supervisor/tools_service.go's newAnalyzerTool used to do it.
func (o *Orchestrator) runAnalyze(ctx context.Context, turn *orchestratorTurn) (*orchestratorTurn, error) {
	if turn.runCtx == nil {
		return turn, fmt.Errorf("orchestrator: analyze reached with no open task")
	}

	if turn.OnSubTaskStarted != nil {
		turn.OnSubTaskStarted("supervisor", "route_to_analyzer", turn.Decision.Label)
	}
	if _, err := turn.runCtx.Recorder.Record(ctx, "supervisor", "route_to_analyzer", "done", "Routed to Nova for evaluation.", turn.Decision.Label, nil); err != nil {
		return turn, fmt.Errorf("orchestrator: record route_to_analyzer: %w", err)
	}

	request := turn.Decision.RequestForAnalyzer
	if request == "" {
		request = turn.RawPrompt
	}

	if turn.OnSubTaskStarted != nil {
		turn.OnSubTaskStarted("analyzer", "gather_evidence", turn.Decision.Label)
	}
	reply, toolCalls, err := runRoleAgent(WithRunContext(ctx, turn.runCtx), o.Analyzer, request, func(toolName, phase string) {
		if turn.OnToolCall != nil {
			turn.OnToolCall("analyzer", toolName, phase)
		}
	})
	if err != nil {
		return turn, fmt.Errorf("orchestrator: analyzer run: %w", err)
	}
	turn.AnalyzerReply = reply
	turn.AnalyzerToolCalls = toolCalls

	if _, err := turn.runCtx.Recorder.Record(ctx, "analyzer", "gather_evidence", "done", reply, turn.Decision.Label, nil); err != nil {
		return turn, fmt.Errorf("orchestrator: record gather_evidence: %w", err)
	}
	return turn, nil
}

// runExecute wraps Comet's own, unmodified ReAct run. Unlike Analyzer,
// Executor's own submit_trade tool (executor/tools_service.go) already
// records "decide"/"execute" itself via the same rc.Recorder — this only
// needs to record the routing hop, and detect a hold (Comet replied without
// ever calling submit_trade) by comparing the chain's tip before/after,
// exactly like the old supervisor/tools_service.go's newExecutorTool did.
func (o *Orchestrator) runExecute(ctx context.Context, turn *orchestratorTurn) (*orchestratorTurn, error) {
	if turn.runCtx == nil {
		return turn, fmt.Errorf("orchestrator: execute reached with no open task")
	}

	if turn.OnSubTaskStarted != nil {
		turn.OnSubTaskStarted("supervisor", "route_to_executor", turn.Decision.Label)
	}
	if _, err := turn.runCtx.Recorder.Record(ctx, "supervisor", "route_to_executor", "done", "Forwarded to Comet for action.", turn.Decision.Label, nil); err != nil {
		return turn, fmt.Errorf("orchestrator: record route_to_executor: %w", err)
	}

	request := turn.Decision.RequestForExecutor
	if request == "" {
		request = turn.AnalyzerReply
	}
	if request == "" {
		request = turn.RawPrompt
	}

	tipBefore := turn.runCtx.Recorder.TerminalHash()
	reply, _, err := runRoleAgent(WithRunContext(ctx, turn.runCtx), o.Executor, request, func(toolName, phase string) {
		if turn.OnToolCall != nil {
			turn.OnToolCall("executor", toolName, phase)
		}
	})
	if err != nil {
		return turn, fmt.Errorf("orchestrator: executor run: %w", err)
	}
	turn.ExecutorReply = reply

	if turn.runCtx.Recorder.TerminalHash() == tipBefore {
		if _, err := turn.runCtx.Recorder.Record(ctx, "executor", "decide", "done", reply, turn.Decision.Label, map[string]any{"action": "hold"}); err != nil {
			return turn, fmt.Errorf("orchestrator: record hold decision: %w", err)
		}
	}
	return turn, nil
}

// runReply is Quasar's second call — the only place in this whole graph
// that streams. Deliberately a plain ReplyModel.Stream() call, not an
// adk.Agent, precisely so it can stream at all (see this file's own top
// comment for why adk.ChatModelAgent can't).
func (o *Orchestrator) runReply(ctx context.Context, turn *orchestratorTurn) (*orchestratorTurn, error) {
	var extra strings.Builder
	if turn.AnalyzerReply != "" {
		extra.WriteString("Nova's findings:\n")
		extra.WriteString(turn.AnalyzerReply)
		extra.WriteString("\n\n")
	}
	if turn.ExecutorReply != "" {
		extra.WriteString("Comet's decision:\n")
		extra.WriteString(turn.ExecutorReply)
		extra.WriteString("\n\n")
	}

	messages := append([]*schema.Message{schema.SystemMessage(quasarReplyInstructions), schema.SystemMessage(currentTimeContext())}, turn.Messages...)
	if extra.Len() > 0 {
		messages = append(messages, schema.SystemMessage(extra.String()))
	}

	stream, err := o.ReplyModel.Stream(ctx, messages)
	if err != nil {
		return turn, fmt.Errorf("orchestrator: reply stream: %w", err)
	}
	defer stream.Close()

	var full strings.Builder
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return turn, fmt.Errorf("orchestrator: reply stream recv: %w", err)
		}
		if chunk.Content == "" {
			continue
		}
		full.WriteString(chunk.Content)
		if turn.OnTextDelta != nil {
			turn.OnTextDelta(chunk.Content)
		}
	}
	turn.FinalReply = full.String()
	return turn, nil
}

// runRoleAgent runs Nova or Comet's own, unmodified adk.Agent to
// completion and returns its final reply text — a fresh, local replacement
// for llm_service.go's RunAgentWithTrace, kept independent of that file on
// purpose. Prefers the last non-empty Assistant message over the literal
// last one, same fix as llm_service.go's own (a tool-calling turn's last
// Assistant event can be the empty tool-call-request one, not the model's
// real closing synthesis).
// onToolCall, if set, fires "start"/"end" around each individual tool call
// inside a's own ReAct loop — real, per-tool-call visibility
// (docs/plans/agent-orchestration-graph-rebuild.md v2.8), not just
// "this whole step started/finished". Confirmed against eino source: every
// tool invocation inside compose's ToolsNode goes through
// callbacks.ReuseHandlers, which fires whatever handlers adk.WithCallbacks
// attached to this specific Run call — this reaches into Nova's/Comet's own
// tool-use loop without either package needing any change.
func runRoleAgent(ctx context.Context, a adk.Agent, prompt string, onToolCall func(toolName, phase string)) (string, []toolCallResult, error) {
	var runOpts []adk.AgentRunOption
	if onToolCall != nil {
		handler := callbacks.NewHandlerBuilder().
			OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
				if info.Component == components.ComponentOfTool {
					onToolCall(info.Name, "start")
				}
				return ctx
			}).
			OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
				if info.Component == components.ComponentOfTool {
					onToolCall(info.Name, "end")
				}
				return ctx
			}).
			Build()
		runOpts = append(runOpts, adk.WithCallbacks(handler))
	}

	iterator := a.Run(ctx, &adk.AgentInput{Messages: []*schema.Message{schema.UserMessage(prompt)}}, runOpts...)

	var lastAssistant, lastNonEmptyAssistant *schema.Message
	var toolCalls []toolCallResult
	for {
		event, ok := iterator.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return "", nil, event.Err
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		out := event.Output.MessageOutput

		var msg *schema.Message
		if out.IsStreaming {
			concatenated, err := schema.ConcatMessageStream(out.MessageStream)
			if err != nil {
				return "", nil, fmt.Errorf("orchestrator: concat message stream: %w", err)
			}
			msg = concatenated
		} else {
			msg = out.Message
		}
		if msg == nil {
			continue
		}

		switch out.Role {
		case schema.Assistant:
			lastAssistant = msg
			if strings.TrimSpace(msg.Content) != "" {
				lastNonEmptyAssistant = msg
			}
		case schema.Tool:
			toolCalls = append(toolCalls, toolCallResult{ToolName: msg.ToolName, Result: msg.Content})
		}
	}

	final := lastNonEmptyAssistant
	if final == nil {
		final = lastAssistant
	}
	if final == nil {
		return "", nil, fmt.Errorf("orchestrator: no final response from agent")
	}
	return final.Content, toolCalls, nil
}

func parseRouteDecision(raw string) (routeDecision, bool) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var decision routeDecision
	if err := json.Unmarshal([]byte(raw), &decision); err != nil {
		return routeDecision{}, false
	}
	if decision.Path == "" {
		return routeDecision{}, false
	}
	return decision, true
}

// analyzerConfirmed defensively reads Nova's own condition_met field —
// Nova's reply is prose-or-JSON depending on how it chose to answer, never
// enforced strictly by its own tool schema, same defensive-parse pattern
// already established elsewhere in this codebase (task_service.go's
// unwrapReplyJSON).
func analyzerConfirmed(raw string) bool {
	var parsed struct {
		ConditionMet bool `json:"condition_met"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return false
	}
	return parsed.ConditionMet
}

const quasarRouteInstructions = `You are Quasar, PulsarFi's trading assistant, deciding how to handle the latest message in this conversation.

Respond with ONLY a single JSON object, no prose, no markdown fences:

{
  "path": "none" | "analyzer_only" | "executor_only" | "analyzer_then_executor",
  "is_actionable": boolean,
  "summary": "short one-line summary of the request, suitable for on-chain storage",
  "label": "short human-readable title for this step, in the same language as the user's message",
  "request_for_analyzer": "what Nova should gather/conclude, if analyzer_only or analyzer_then_executor",
  "request_for_executor": "what Comet should decide/do, if executor_only or analyzer_then_executor"
}

Path meanings:
- "none": pure conversation, nothing to fetch or act on (a greeting, small talk). No Task is created for this path.
- "analyzer_only": the user wants information only — news, sentiment, portfolio, or chart data. Nova gathers it, you just relay the answer.
- "executor_only": the user already has enough information and is directly instructing an action now.
- "analyzer_then_executor": a trigger condition needs fresh evaluation before anything should happen — Nova evaluates first, Comet only acts if Nova's conclusion genuinely confirms it.

is_actionable is true only if this request could ever result in an on-chain trade (executor_only or analyzer_then_executor), false for anything purely informational (including chart/portfolio lookups).

Every distinct message gets its own new decision — never assume a follow-up silently continues an earlier one.`

const quasarReplyInstructions = `You are Quasar, PulsarFi's trading assistant. You work with two teammates: Nova, who reads news and numbers and answers chart/portfolio questions, and Comet, who decides and is the only one who submits anything on-chain. Never describe yourself or them using technical words like "supervisor", "node", "agent", "system", "tool", or "sub-agent" — to the user, you are Quasar, and if you ever mention them, they are Nova and Comet.

Write the final reply to the user now, in your own voice: professional but warm, plain modern language, confident without being stiff. No em dash character, use a comma or a period instead. Reply in whichever language the user wrote in.

A system message below states the current real date and time in WIB — trust it as fact whenever the user asks anything about today, the current time, or "this week", never guess or say you don't know.

If Nova's findings or Comet's decision are given below as system context, summarize them naturally as your own answer — never repeat them verbatim, never mention that they came from a teammate.`
