package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"regexp"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/horizonlabs/pulsarfi-backend/src/contracts"
	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
)

// This file holds ONLY the graph's node functions (runUnderstand, runAnalyze,
// runExecute, runReply) and their pure parsing/dispatch helpers — no
// repository or database call of any kind belongs here beyond Quasar's own
// Task/Sub Task lifecycle writes (commitTask), which are orchestration
// bookkeeping, not domain lookups. Ticker/price/portfolio data enters this
// file exclusively as a typed Go struct already parsed from an LLM's JSON
// output or a sub-agent's own tool result (docs/plans/quasar-clean-routing-scalp-refactor.md
// §2 hard requirement).

// runUnderstand is the graph's first real step: it turns the user's raw
// message (plus conversation history) into the structured routeDecision
// object everything downstream operates on — the LLM call and JSON parsing
// live here, nowhere else. mentioned_ticker and side are taken verbatim
// from the LLM's own output (per the routeDecision schema's own contract —
// mentioned_ticker is documented as "the ticker symbol the user actually
// typed, verbatim"); there is no Go-side string normalization anywhere in
// this file. Canonicalizing a ticker against the real catalog (matching
// "BRPT" to "BRPTP", case-insensitively) is a database lookup — Nova's
// verify_ticker tool's job (via LIKE/ILIKE at the query layer), never
// application-code guesswork performed before the database is ever
// consulted.
func (o *Orchestrator) runUnderstand(ctx context.Context, turn *orchestratorTurn) (*orchestratorTurn, error) {
	// This node is the one that calls compose.StatefulInterrupt below, so it
	// is also the only node allowed to recover state on resume — but the
	// node's own `turn` parameter is NOT reliable for that: confirmed via an
	// isolated reproduction test (backend/test/service/agent) that Eino
	// hands a manually-interrupted node a nil input on the resumed call,
	// even when it is the very same node that raised the interrupt. The
	// only reliable channel back is GetInterruptState, recovering exactly
	// what was passed to StatefulInterrupt — which must therefore be the
	// whole turn, not just the decision, or everything else on it
	// (Wallet, ChatID, RawPrompt...) would be lost on resume too.
	recoveredFromInterrupt := false
	if wasInterrupted, hasState, prevTurn := compose.GetInterruptState[*orchestratorTurn](ctx); wasInterrupted && hasState && prevTurn != nil {
		turn = prevTurn
		recoveredFromInterrupt = true
		// gob strips func fields on decode, so the recovered turn's
		// callbacks are always nil — re-attach the CURRENT call's, carried
		// via context since (unlike the turn parameter) ctx is reliably
		// threaded through to a resumed node.
		if cb, ok := turnCallbacksFrom(ctx); ok {
			turn.bindCallbacks(cb.AgentEventCallbacks)
			turn.onLatestSnapshot = cb.onLatestSnapshot
		}
	}
	if turn == nil {
		return nil, fmt.Errorf("orchestrator: understand has no usable turn (neither a fresh input nor recovered interrupt state)")
	}

	// A Task now opens as soon as an intent is recognized (possibly still
	// needs_input), so TaskID != 0 alone no longer means "already fully
	// understood" — a needs_input resume recovers a turn whose TaskID was
	// set back in round one, and still needs decideRouteWithAnswer below.
	// Only a genuinely fresh invoke with a caller-preset TaskID and no
	// recovered interrupt state — the arm/execute pause, which always sets
	// TaskID before invoking via Orchestrator.ResumeExecute — should skip
	// straight through.
	if turn.TaskID != 0 && !recoveredFromInterrupt {
		return turn, nil
	}

	isResume, hasData, rawAnswer := compose.GetResumeContext[any](ctx)

	var decision routeDecision
	var err error

	switch {
	case isResume && hasData:
		answerStr, _ := rawAnswer.(string)
		decision, err = o.decideRouteWithAnswer(ctx, turn.Decision, answerStr)
	case isResume:
		decision = turn.Decision
	default:
		decision, err = o.decideRoute(ctx, turn)
	}

	// Captured before turn.Decision is overwritten below — updateTask needs
	// the PREVIOUS round's actionability to detect a false->true
	// transition; comparing against turn.Decision after the overwrite would
	// just compare decision against itself.
	wasActionable := turn.Decision.IsActionable
	turn.Decision = decision
	// Clear whatever a PRIOR needs_input round left behind — recovered via
	// GetInterruptState, turn may still be carrying the previous cycle's
	// now-answered questions. Left uncleared, classifyReply would keep
	// telling the frontend to render the (stale, already-answered)
	// clarifying-questions form on every later turn, including the one
	// where the Task is actually ready to arm.
	turn.PendingQuestions = nil
	if decision.MentionedTicker != "" {
		turn.ResolvedTicker = decision.MentionedTicker
	}
	if err != nil || decision.Path == pathNone {
		return turn, err
	}

	// A permanent, on-chain-auditable Task opens the instant Quasar
	// recognizes an intent, not only once every field is finally settled
	// — the first round opens it (Task + the "understand_request" and
	// "route_decision" Sub Tasks below), every later resumed round just
	// updates the same row and adds a fresh "route_decision" Sub Task,
	// never a second Task/on-chain createTask.
	if turn.TaskID == 0 {
		opened, cerr := o.commitTask(ctx, turn, decision)
		if cerr != nil {
			return opened, cerr
		}
		turn = opened
		if recErr := o.recordUnderstandAndRoute(ctx, turn, true); recErr != nil {
			slog.WarnContext(ctx, "orchestrator: record understand/route sub tasks failed", "error", recErr)
		}
	} else {
		if err := o.ensureRunContext(ctx, turn); err != nil {
			return turn, err
		}
		if err := o.updateTask(ctx, turn, decision, wasActionable); err != nil {
			return turn, err
		}
		if recErr := o.recordUnderstandAndRoute(ctx, turn, false); recErr != nil {
			slog.WarnContext(ctx, "orchestrator: record route sub task failed", "error", recErr)
		}
	}

	// Pause ONLY for genuinely missing input — never merely because a path
	// happens to be non-actionable. analyzer_only (a pure chart/news/portfolio
	// question) is a complete, settled decision on its own; it must flow
	// straight through to Nova, not sit waiting for an answer nobody owes.
	if decision.Path == pathNeedsInput || len(decision.Unanswered) > 0 {
		turn.PendingQuestions = decision.Questions
		// This interrupt short-circuits the graph before "reply" (the node)
		// would ever run — the user still needs Quasar's short framing
		// sentence before the clarifying-question card, not silence, so
		// compose it here directly, reusing the exact same logic "reply"
		// itself uses. A failure here is logged, not fatal: the card's own
		// questions still communicate what's needed even without the prose.
		if err := o.composeReply(ctx, turn); err != nil {
			slog.WarnContext(ctx, "orchestrator: compose needs_input framing reply failed", "error", err)
		}
		return turn, compose.StatefulInterrupt(ctx, decision.Questions, turn)
	}

	if decision.IsActionable && turn.runCtx != nil && turn.runCtx.Recorder != nil {
		if turn.onSubTaskStarted != nil {
			turn.onSubTaskStarted("supervisor", "await_confirmation", decision.Label)
		}
		if _, err := turn.runCtx.Recorder.Record(ctx, "supervisor", "await_confirmation", "done", "Arm Card presented, awaiting the user's on-chain confirmation.", decision.Label, nil); err != nil {
			slog.WarnContext(ctx, "orchestrator: record await_confirmation failed", "error", err)
		}
	}

	return turn, nil
}

// recordUnderstandAndRoute writes Quasar's own reasoning as real, permanent
// Sub Tasks tied to the Task opened above — "understand_request" fires
// exactly once, the first time an intent is recognized; "route_decision"
// fires every round (including every resumed answer), so the on-chain
// trail shows exactly how the routing evolved as the user answered.
func (o *Orchestrator) recordUnderstandAndRoute(ctx context.Context, turn *orchestratorTurn, isFirst bool) error {
	if turn.runCtx == nil || turn.runCtx.Recorder == nil {
		return nil
	}
	if isFirst {
		if turn.onSubTaskStarted != nil {
			turn.onSubTaskStarted("supervisor", "understand_request", turn.Decision.Label)
		}
		if _, err := turn.runCtx.Recorder.Record(ctx, "supervisor", "understand_request", "done", turn.Decision.Summary, turn.Decision.Label, nil); err != nil {
			return fmt.Errorf("orchestrator: record understand_request: %w", err)
		}
	}

	if turn.onSubTaskStarted != nil {
		turn.onSubTaskStarted("supervisor", "route_decision", turn.Decision.Label)
	}
	if _, err := turn.runCtx.Recorder.Record(ctx, "supervisor", "route_decision", "done", routeDecisionReasoning(turn.Decision), turn.Decision.Label, nil); err != nil {
		return fmt.Errorf("orchestrator: record route_decision: %w", err)
	}
	return nil
}

func routeDecisionReasoning(decision routeDecision) string {
	if decision.Path == pathNeedsInput || len(decision.Unanswered) > 0 {
		return fmt.Sprintf("Path: needs_input — waiting on: %s", strings.Join(decision.Unanswered, ", "))
	}
	return fmt.Sprintf("Path: %s", decision.Path)
}

// updateTask patches an already-open Task's settled fields once a resumed
// answer completes them — never a second on-chain createTask, only the
// Postgres summary/trigger_description/is_actionable are refreshed to
// match the latest decision.
func (o *Orchestrator) updateTask(ctx context.Context, turn *orchestratorTurn, decision routeDecision, wasActionable bool) error {
	if o.Tasks == nil {
		return nil
	}
	card := decision.Card
	if card == nil {
		card = &contracts.CardContract{}
	}
	side := decision.Side
	if decision.MentionedTicker != "" {
		turn.ResolvedTicker = decision.MentionedTicker
	}
	if decision.Summary == "" {
		decision.Summary = defaultSummary(side, decision, turn.ResolvedTicker)
	}

	stockContractAddress := o.resolveStockContractAddress(ctx, side, turn.ResolvedTicker)
	triggerDescription, err := buildTriggerDescription(turn, decision, card, side, stockContractAddress)
	if err != nil {
		return fmt.Errorf("orchestrator: marshal updated trigger description: %w", err)
	}

	// The Task's on-chain isActionable flag is set once, at createTask, and
	// has no other setter besides markActionable — flip it the first time a
	// round settles into an actionable decision, never again afterward.
	if decision.IsActionable && !wasActionable && o.Chain != nil && turn.OnChainTaskID != nil {
		if err := o.Chain.MarkActionable(ctx, uint64(*turn.OnChainTaskID)); err != nil {
			return fmt.Errorf("orchestrator: on-chain markActionable: %w", err)
		}
	}

	if err := o.Tasks.UpdateDecision(ctx, turn.TaskID, decision.Summary, triggerDescription, decision.IsActionable); err != nil {
		return fmt.Errorf("orchestrator: update task decision: %w", err)
	}
	turn.Decision = decision
	return nil
}

// runCommit is the work understand itself must never do: opening the Task
// (Postgres row, on-chain createTask, Sub Task recorder) once understand has
// settled on a decision that needs one. Every AI action gets a permanent,
// on-chain-auditable Task — not just trades: analyzer_only (Nova doing
// real work: tool calls, research, answering) commits exactly like an
// actionable request. The only decision that never commits is "none" —
// pure small talk that never invokes Nova or Comet at all, nothing to
// audit. is_actionable still separately gates whether a Task can ever
// result in an on-chain trade; it no longer gates whether a Task exists.
func (o *Orchestrator) runCommit(ctx context.Context, turn *orchestratorTurn) (*orchestratorTurn, error) {
	if turn.TaskID != 0 || turn.Decision.Path == pathNone {
		return turn, nil
	}
	return o.commitTask(ctx, turn, turn.Decision)
}

func (o *Orchestrator) decideRoute(ctx context.Context, turn *orchestratorTurn) (routeDecision, error) {
	slog.InfoContext(ctx, "orchestrator: llm call", "step", "route", "model", o.RouteModelName)

	messages := []*schema.Message{
		schema.SystemMessage(quasarRouteInstructions),
		schema.SystemMessage(currentTimeContext()),
	}
	messages = append(messages, turn.Messages...)

	response, err := o.RouteModel.Generate(ctx, messages)
	if err != nil {
		return routeDecision{}, fmt.Errorf("orchestrator: route generate: %w", err)
	}

	decision, ok := parseRouteDecision(response.Content)
	if !ok {
		slog.WarnContext(ctx, "orchestrator: route decision did not parse, falling back to reply-only", "raw", response.Content)
		return routeDecision{Path: pathNone}, nil
	}
	return decision, nil
}

// decideRouteWithAnswer re-invokes Quasar's routing model with the user's
// clarifying-question answer merged into the previously-paused decision
// state, instead of re-reading the whole conversation from scratch. The
// instructional text lives in instructions.go
// (quasarRouteResumeInstructions); this function only builds the message
// list, no prompt text of its own.
func (o *Orchestrator) decideRouteWithAnswer(ctx context.Context, prevDecision routeDecision, answer string) (routeDecision, error) {
	slog.InfoContext(ctx, "orchestrator: llm call", "step", "route_resume", "model", o.RouteModelName)

	prevBytes, err := json.Marshal(prevDecision)
	if err != nil {
		return routeDecision{}, fmt.Errorf("orchestrator: marshal previous decision: %w", err)
	}

	messages := []*schema.Message{
		schema.SystemMessage(quasarRouteInstructions),
		schema.SystemMessage(quasarRouteResumeInstructions),
		schema.SystemMessage(currentTimeContext()),
		schema.SystemMessage("Previous route decision state:\n" + string(prevBytes)),
		schema.UserMessage(answer),
	}

	response, err := o.RouteModel.Generate(ctx, messages)
	if err != nil {
		return routeDecision{}, fmt.Errorf("orchestrator: route resume generate: %w", err)
	}

	decision, ok := parseRouteDecision(response.Content)
	if !ok {
		slog.WarnContext(ctx, "orchestrator: route resume decision did not parse", "raw", response.Content)
		return prevDecision, nil
	}
	return decision, nil
}

// commitTask opens the Task — Postgres row, on-chain createTask, and the
// Sub Task recorder — from an already-settled route decision. It never
// touches the stock catalog: ticker resolution/verification is entirely
// Nova's job (verify_ticker), not Quasar's.
func (o *Orchestrator) commitTask(ctx context.Context, turn *orchestratorTurn, decision routeDecision) (*orchestratorTurn, error) {
	card := decision.Card
	if card == nil {
		card = &contracts.CardContract{}
	}
	if card.TaskBadge == "" {
		card.TaskBadge = "Task T-{id}"
	}

	// side stays exactly what Quasar's own routing decision said — the
	// schema itself mandates the canonical English value ("buy"/"sell"),
	// whatever language the user actually typed in, so there is no
	// Go-side conversion needed downstream.
	side := decision.Side
	if decision.MentionedTicker != "" {
		turn.ResolvedTicker = decision.MentionedTicker
	}

	if decision.Summary == "" {
		decision.Summary = defaultSummary(side, decision, turn.ResolvedTicker)
	}

	stockContractAddress := o.resolveStockContractAddress(ctx, side, turn.ResolvedTicker)
	triggerDescription, err := buildTriggerDescription(turn, decision, card, side, stockContractAddress)
	if err != nil {
		return turn, fmt.Errorf("orchestrator: marshal trigger description: %w", err)
	}

	task, err := o.persistTask(ctx, turn, decision, triggerDescription)
	if err != nil {
		return turn, err
	}
	turn.TaskID = task.ID

	onChainTaskID, err := o.armOnChain(ctx, turn, decision, task.ID)
	if err != nil {
		return turn, err
	}

	if err := o.attachRunContext(ctx, turn, decision, onChainTaskID, task.ID); err != nil {
		return turn, err
	}

	turn.Decision = decision
	return turn, nil
}

func defaultSummary(side string, decision routeDecision, ticker string) string {
	if side == "sell" {
		return fmt.Sprintf("Sell %s %s", decision.SellAmount, ticker)
	}
	return fmt.Sprintf("Buy %s IDRX of %s", decision.BudgetIDRX, ticker)
}

// resolveStockContractAddress returns the on-chain ERC-20 address for a sell
// task's stock token. It is called by commitTask and updateTask — both already
// hold a context — so the single catalog read stays co-located with the rest
// of the trigger-description build and does not leak into node functions.
// Returns "" (non-fatal) when: Stocks is nil, ticker is empty, the ticker is
// not in the catalog, or the stock has no deployed contract yet. The caller
// writes the result into trigger_description only when non-empty.
func (o *Orchestrator) resolveStockContractAddress(ctx context.Context, side, ticker string) string {
	if side != "sell" || ticker == "" || o.Stocks == nil {
		return ""
	}
	stock, found, err := o.Stocks.FindByTickerOrIdxTicker(ctx, ticker)
	if err != nil || !found || stock.ContractAddress == nil || *stock.ContractAddress == "" {
		return ""
	}
	return *stock.ContractAddress
}

// buildTriggerDescription maps the settled decision into the JSON blob
// persisted on the Task row and read back by the frontend card — pure
// struct-to-JSON mapping, no lookups.
// stockContractAddress is non-empty only for sell tasks; the caller resolves
// it via o.Stocks before calling here so this function stays IO-free.
func buildTriggerDescription(turn *orchestratorTurn, decision routeDecision, card *contracts.CardContract, side string, stockContractAddress string) (string, error) {
	cleanShape := strings.ToLower(strings.TrimSpace(decision.Shape))

	triggerMap := map[string]any{
		"prompt": turn.RawPrompt,
		"card":   card,
		"shape":  cleanShape,
		"side":   side,
	}
	switch {
	case cleanShape == string(contracts.ShapeInvestment) || cleanShape == "dca":
		triggerMap["is_recurring"] = true
	case cleanShape == string(contracts.ShapeSwing):
		triggerMap["is_swing"] = true
	}
	if decision.BudgetIDRX != "" {
		triggerMap["confirmed_budget_idrx"] = decision.BudgetIDRX
		triggerMap["budget"] = decision.BudgetIDRX
	}
	if decision.SellAmount != "" {
		triggerMap["sell_amount"] = decision.SellAmount
		triggerMap["budget"] = decision.SellAmount
	}
	if turn.ResolvedTicker != "" {
		triggerMap["resolved_ticker"] = turn.ResolvedTicker
		triggerMap["stock_ticker"] = turn.ResolvedTicker
		if side == "sell" {
			triggerMap["token_symbol"] = turn.ResolvedTicker
		} else {
			triggerMap["token_symbol"] = "IDRX"
		}
	}
	// For sell tasks, write the stock token's contract address so the frontend
	// knows which ERC-20 to approve (ArmPanel reads "stock_contract_address").
	// Buy tasks never need this — IDRX address is hardcoded in the frontend env.
	if side == "sell" && stockContractAddress != "" {
		triggerMap["stock_contract_address"] = stockContractAddress
	}

	triggerBytes, err := json.Marshal(triggerMap)
	if err != nil {
		return "", err
	}
	return string(triggerBytes), nil
}

func (o *Orchestrator) persistTask(ctx context.Context, turn *orchestratorTurn, decision routeDecision, triggerDescription string) (model.AgentTask, error) {
	if o.Tasks == nil {
		return model.AgentTask{}, nil
	}
	rawPrompt := turn.RawPrompt
	task, err := o.Tasks.Create(ctx, repository.AgentTaskCreateInput{
		WalletAddress:      turn.Wallet,
		RawPrompt:          &rawPrompt,
		Summary:            decision.Summary,
		TriggerDescription: &triggerDescription,
		IsActionable:       decision.IsActionable,
		SourceMessageID:    turn.SourceMessageID,
	})
	if err != nil {
		return model.AgentTask{}, fmt.Errorf("orchestrator: create task in db: %w", err)
	}
	return task, nil
}

func (o *Orchestrator) armOnChain(ctx context.Context, turn *orchestratorTurn, decision routeDecision, taskID int64) (uint64, error) {
	if o.Chain == nil {
		return 0, nil
	}
	promptHash := crypto.Keccak256Hash([]byte(turn.RawPrompt))
	onChainTaskID, err := o.Chain.CreateTask(ctx, turn.Wallet, decision.IsActionable, decision.Summary, promptHash)
	if err != nil {
		return 0, fmt.Errorf("orchestrator: on-chain createTask: %w", err)
	}
	if o.Tasks != nil {
		if err := o.Tasks.SetOnChainTaskID(ctx, taskID, int64(onChainTaskID)); err != nil {
			return 0, fmt.Errorf("orchestrator: persist onChainTaskID: %w", err)
		}
	}
	return onChainTaskID, nil
}

func (o *Orchestrator) attachRunContext(ctx context.Context, turn *orchestratorTurn, decision routeDecision, onChainTaskID uint64, taskID int64) error {
	recorder, err := NewSubTaskRecorder(ctx, o.SubTasks, o.Chain, onChainTaskID, taskID, decision.Summary, turn.Wallet)
	if err != nil {
		return fmt.Errorf("orchestrator: build subtask recorder: %w", err)
	}
	if turn.onSubTask != nil {
		recorder.OnRecord = turn.onSubTask
	}

	var onChainIDPtr *int64
	if onChainTaskID != 0 {
		v := int64(onChainTaskID)
		onChainIDPtr = &v
		turn.OnChainTaskID = onChainIDPtr
	}
	turn.runCtx = &RunContext{
		OnChainTaskID:    onChainIDPtr,
		Wallet:           turn.Wallet,
		Recorder:         recorder,
		OnSubTaskStarted: turn.onSubTaskStarted,
		OnSubTaskFailed:  turn.onSubTaskFailed,
	}
	return nil
}

// runAnalyze delegates ticker verification and market analysis to Nova.
// Only reached on analyzer_only/analyzer_then_executor — a request routed
// executor_only (the user explicitly opted out of Nova via "consult_nova",
// any shape) skips this node entirely. Nova's structured verdict becomes
// part of `turn` here and survives the arm/execute pause via the graph's
// own checkpoint, no persistence of its own; the verdict is advisory only,
// see runExecute.
func (o *Orchestrator) runAnalyze(ctx context.Context, turn *orchestratorTurn) (*orchestratorTurn, error) {
	// A purely informational request (analyzer_only — "what's the news on
	// IHSG today") never opens a Task (runCommit skips it by design, no
	// trade means nothing to audit on-chain). Nova still needs a
	// wallet-scoped RunContext for tools like get_portfolio_snapshot, but
	// there is no Sub Task chain to record against.
	if turn.runCtx == nil {
		turn.runCtx = &RunContext{Wallet: turn.Wallet}
	}
	recording := turn.runCtx.Recorder != nil

	// The live "in progress" UI ping is ephemeral (never persisted) and
	// must always fire so the chat visibly shows Quasar handing off to
	// Nova, whether or not this request has a real on-chain Task behind
	// it. Only the durable Sub Task chain write below is conditional on
	// having one.
	if turn.onSubTaskStarted != nil {
		turn.onSubTaskStarted("supervisor", "route_to_analyzer", turn.Decision.Label)
	}
	if recording {
		if _, err := turn.runCtx.Recorder.Record(ctx, "supervisor", "route_to_analyzer", "done", "Routed to Nova for evaluation.", turn.Decision.Label, nil); err != nil {
			return turn, fmt.Errorf("orchestrator: record route_to_analyzer: %w", err)
		}
	} else {
		// No Task to persist a Sub Task row against — but the frontend
		// already rendered this step from the started ping above and needs
		// a matching completion with real content, or it shows "done" with
		// nothing. Fire the same completion shape, just not written to
		// Postgres/on-chain.
		notifySyntheticSubTask(turn, "supervisor", "route_to_analyzer", "Routed to Nova for evaluation.")
	}

	request := turn.Decision.RequestForAnalyzer
	if request == "" {
		request = turn.RawPrompt
	}
	request = buildAnalyzerRequest(request, turn.Decision.Shape, turn.ResolvedTicker, turn.Decision.Side, turn.Decision.IsActionable)

	if turn.onSubTaskStarted != nil {
		turn.onSubTaskStarted("analyzer", "gather_evidence", turn.Decision.Label)
	}
	turn.runCtx.CurrentAgent = "analyzer"
	turn.runCtx.CurrentStepName = "gather_evidence"
	slog.InfoContext(ctx, "orchestrator: llm call", "step", "gather_evidence", "agent", "analyzer", "model", o.AnalyzerModelName, "task_id", turn.TaskID)
	reply, toolCalls, err := runRoleAgent(WithRunContext(ctx, turn.runCtx), o.Analyzer, request, true,
		func(toolName, phase string) {
			if turn.onToolCall != nil {
				turn.onToolCall("analyzer", toolName, phase)
			}
		},
		func(delta string) {
			if turn.onThinking != nil {
				turn.onThinking("analyzer", delta)
			}
		},
	)
	if err != nil {
		if recording {
			if _, recErr := turn.runCtx.Recorder.Record(ctx, "analyzer", "gather_evidence", "failed", err.Error(), turn.Decision.Label, nil); recErr != nil {
				slog.ErrorContext(ctx, "orchestrator: record gather_evidence failed step", "error", recErr)
			}
		}
		return turn, fmt.Errorf("orchestrator: analyzer agent run: %w", err)
	}

	// A tradeable/entry_price verdict is only meaningful for an actionable
	// request — for analyzer_only there is no trade to verdict on, so
	// nothing is parsed as one; Nova's whole reply is just its answer.
	cleanProse := reply
	if turn.Decision.IsActionable {
		verdict, prose := parseNovaVerdict(reply)
		turn.NovaVerdict = verdict
		cleanProse = prose
	}
	turn.AnalyzerReply = cleanProse
	turn.AnalyzerToolCalls = toolCalls

	if recording {
		if _, err := turn.runCtx.Recorder.Record(ctx, "analyzer", "gather_evidence", "done", cleanProse, turn.Decision.Label, nil); err != nil {
			return turn, fmt.Errorf("orchestrator: record gather_evidence: %w", err)
		}
	} else {
		notifySyntheticSubTask(turn, "analyzer", "gather_evidence", cleanProse)
	}

	return turn, nil
}

// notifySyntheticSubTask fires the same OnSubTask completion shape the
// frontend gets for a real, persisted Sub Task row — for a request with no
// Task behind it (informational, analyzer_only), so a step already shown as
// started (turn.onSubTaskStarted) still gets a real, non-empty completion
// instead of being left to show "done" with nothing. Never written to
// Postgres or on-chain — id 0, not a real row.
func notifySyntheticSubTask(turn *orchestratorTurn, agentName, stepName, reasoning string) {
	if turn.onSubTask == nil {
		return
	}
	var labelPtr *string
	if turn.Decision.Label != "" {
		label := turn.Decision.Label
		labelPtr = &label
	}
	turn.onSubTask(model.AgentSubTask{
		Agent:     agentName,
		StepName:  stepName,
		Label:     labelPtr,
		Status:    "done",
		Reasoning: reasoning,
	})
}

func buildAnalyzerRequest(userInstruction, shape, ticker, side string, isActionable bool) string {
	if !isActionable {
		return fmt.Sprintf("User instruction: %s\n\nAnswer using your available tools. This is an informational request only — no trade is being evaluated, do not verify tradeability or output a trade verdict.", userInstruction)
	}
	return fmt.Sprintf(
		"Trading shape: %s\nTicker: %s\nSide: %s\nUser instruction: %s\n\n"+
			"Verify whether %s is tokenized and tradeable on PulsarFi using verify_ticker. "+
			"Gather relevant market evidence, live pool price, and technical indicators for %s matching trading shape %s.\n"+
			"Conclude your reply with a fenced ```json block stating whether tradeable is true or false, entry_price, confidence, and reasoning.",
		shape, ticker, side, userInstruction, ticker, ticker, shape,
	)
}

// runExecute wraps Comet's on-chain execution. It only ever runs after the
// engine's arm/execute pause resumes (compose.WithInterruptBeforeNodes in
// orchestrator_service.go), i.e. only once the Task layer has confirmed
// on-chain arm + allowance. Nova's verdict is advisory only: Comet is given
// it as context for sizing/timing, but the user's own instruction and
// approved budget is the final authority — a tradeable:false verdict never
// blocks execution, the user is never overridden by Nova's opinion.
func (o *Orchestrator) runExecute(ctx context.Context, turn *orchestratorTurn) (*orchestratorTurn, error) {
	if err := o.ensureRunContext(ctx, turn); err != nil {
		return turn, err
	}

	if turn.onSubTaskStarted != nil {
		turn.onSubTaskStarted("supervisor", "route_to_executor", turn.Decision.Label)
	}
	if _, err := turn.runCtx.Recorder.Record(ctx, "supervisor", "route_to_executor", "done", "Routed to Comet for execution.", turn.Decision.Label, nil); err != nil {
		return turn, fmt.Errorf("orchestrator: record route_to_executor: %w", err)
	}

	request := buildExecutorRequest(turn)

	tipBefore := turn.runCtx.Recorder.TerminalHash()
	turn.runCtx.CurrentAgent = "executor"
	turn.runCtx.CurrentStepName = "decide"
	slog.InfoContext(ctx, "orchestrator: llm call", "step", "decide", "agent", "executor", "model", o.ExecutorModelName, "task_id", turn.TaskID)
	reply, _, err := runRoleAgent(WithRunContext(ctx, turn.runCtx), o.Executor, request, false,
		func(toolName, phase string) {
			if turn.onToolCall != nil {
				turn.onToolCall("executor", toolName, phase)
			}
		},
		func(delta string) {
			if turn.onThinking != nil {
				turn.onThinking("executor", delta)
			}
		},
	)
	if err != nil {
		if _, recErr := turn.runCtx.Recorder.Record(ctx, "executor", "decide", "failed", err.Error(), turn.Decision.Label, nil); recErr != nil {
			slog.ErrorContext(ctx, "orchestrator: record decide failed step", "error", recErr)
		}
		return turn, fmt.Errorf("orchestrator: executor agent run: %w", err)
	}

	if turn.runCtx.Recorder.TerminalHash() == tipBefore {
		if _, err := turn.runCtx.Recorder.Record(ctx, "executor", "decide", "done", reply, turn.Decision.Label, nil); err != nil {
			return turn, fmt.Errorf("orchestrator: record executor decide step: %w", err)
		}
	}

	turn.ExecutorReply = reply
	return turn, nil
}

// ensureRunContext rebuilds the Sub Task recorder if the checkpoint didn't
// carry it forward (RunContext holds a live recorder/closures, which — unlike
// the plain routeDecision/NovaVerdict data on turn — aren't expected to
// survive serialization across the arm/execute pause).
func (o *Orchestrator) ensureRunContext(ctx context.Context, turn *orchestratorTurn) error {
	if turn.runCtx != nil {
		return nil
	}
	if turn.TaskID == 0 {
		return fmt.Errorf("orchestrator: execute reached with no open task")
	}
	var onChainID uint64
	if turn.OnChainTaskID != nil {
		onChainID = uint64(*turn.OnChainTaskID)
	}
	recorder, err := NewSubTaskRecorder(ctx, o.SubTasks, o.Chain, onChainID, turn.TaskID, turn.Decision.Summary, turn.Wallet)
	if err != nil {
		return fmt.Errorf("orchestrator: rebuild subtask recorder: %w", err)
	}
	turn.runCtx = &RunContext{
		OnChainTaskID:    turn.OnChainTaskID,
		Wallet:           turn.Wallet,
		Recorder:         recorder,
		OnSubTaskStarted: turn.onSubTaskStarted,
		OnSubTaskFailed:  turn.onSubTaskFailed,
	}
	return nil
}

func buildExecutorRequest(turn *orchestratorTurn) string {
	request := turn.Decision.RequestForExecutor
	if request == "" {
		request = turn.RawPrompt
	}
	if turn.AnalyzerReply == "" {
		return request
	}

	verdictSummary := ""
	if turn.NovaVerdict != nil {
		verdictSummary = fmt.Sprintf(
			"\n\nNova's structured verdict:\n- Tradeable: %v\n- Entry price: %s\n- Confidence: %s\n- Reasoning: %s",
			turn.NovaVerdict.Tradeable, turn.NovaVerdict.EntryPrice, turn.NovaVerdict.Confidence, turn.NovaVerdict.Reasoning,
		)
	}
	return fmt.Sprintf(
		"User instruction:\n%s\n\nNova's market findings:\n%s%s\n\n"+
			"The task is armed and approved on-chain. Based on Nova's market findings and structured verdict above, "+
			"size within the approved budget and execute the trade on-chain using submit_trade. "+
			"Reflect Nova's market findings in your chat reasoning to explain your decision.",
		request, turn.AnalyzerReply, verdictSummary,
	)
}

// runReply generates Quasar's closing message to the user — the Arm Card
// invitation for an actionable request, or a plain informational reply
// otherwise. Runs before the arm/execute pause for every actionable path
// (see the reply branch in orchestrator_service.go), so the card is always
// visible before the pause engages.
func (o *Orchestrator) runReply(ctx context.Context, turn *orchestratorTurn) (*orchestratorTurn, error) {
	return turn, o.composeReply(ctx, turn)
}

// composeReply streams Quasar's closing message into turn.FinalReply. Shared
// by the "reply" node itself and by runUnderstand's needs_input pause — a
// needs_input interrupt short-circuits the graph before "reply" would ever
// run on its own, but the user still needs Quasar's short framing sentence
// ("just need a couple more things") before the clarifying-question card,
// not silence.
func (o *Orchestrator) composeReply(ctx context.Context, turn *orchestratorTurn) error {
	extra := buildReplyContext(turn)

	messages := append([]*schema.Message{schema.SystemMessage(quasarReplyInstructions), schema.SystemMessage(currentTimeContext())}, turn.Messages...)
	if extra != "" {
		messages = append(messages, schema.SystemMessage(extra))
	}

	slog.InfoContext(ctx, "orchestrator: llm call", "step", "reply", "model", o.ReplyModelName, "task_id", turn.TaskID)
	stream, err := o.ReplyModel.Stream(ctx, messages)
	if err != nil {
		return fmt.Errorf("orchestrator: reply stream: %w", err)
	}
	defer stream.Close()

	var full strings.Builder
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("orchestrator: reply stream recv: %w", err)
		}
		if chunk.Content == "" {
			continue
		}
		full.WriteString(chunk.Content)
		if turn.onTextDelta != nil {
			turn.onTextDelta(chunk.Content)
		}
	}
	turn.FinalReply = full.String()
	if turn.onLatestSnapshot != nil {
		turn.onLatestSnapshot(turn)
	}
	return nil
}

func buildReplyContext(turn *orchestratorTurn) string {
	var extra strings.Builder

	if turn.AnalyzerReply != "" {
		extra.WriteString("Nova findings:\n" + turn.AnalyzerReply + "\n\n")
	}
	if turn.ExecutorReply != "" {
		extra.WriteString("Comet decision:\n" + turn.ExecutorReply + "\n\n")
	}

	if turn.Decision.IsActionable && turn.TaskID > 0 {
		if turn.AnalyzerReply != "" {
			extra.WriteString(fmt.Sprintf("Arm Card for Task T-%d is displayed below your message. In the user's active language, summarize Nova's market findings briefly in 1-2 sentences, and direct the user to review the parameters and click Arm on the card below. Strictly NO conversational filler.\n\n", turn.TaskID))
		} else {
			extra.WriteString(fmt.Sprintf("Arm Card for Task T-%d is displayed below your message. In a single brief sentence in the user's language, state the confirmed trade parameters and direct the user to review and click Arm on the card below. Strictly FORBIDDEN from chatting casually, claiming you are preparing the trade, or giving commentary. Only 1 direct sentence directing to the card.\n\n", turn.TaskID))
		}
	}

	if turn.ResolvedTicker != "" && !turn.Decision.IsActionable {
		extra.WriteString(fmt.Sprintf("Ticker resolved to %s from what the user typed.\n\n", turn.ResolvedTicker))
	}
	if turn.TickerProblem != "" {
		extra.WriteString("Ticker issue: " + turn.TickerProblem + ". Inform the user in their language.\n\n")
	}
	if len(turn.PendingQuestions) > 0 {
		extra.WriteString("Clarifying questions are displayed in an interactive card below. Do not repeat the questions in chat prose. Briefly invite the user in their language to answer the card.\n\n")
	}

	return extra.String()
}

// toolCallResult mirrors one tool call Nova (or Comet) made internally.
type toolCallResult struct {
	ToolName string
	Result   string
}

type newsEvidenceItem struct {
	Title       string `json:"title,omitempty"`
	Source      string `json:"source"`
	URL         string `json:"url,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
	Excerpt     string `json:"excerpt,omitempty"`
	ImageURL    string `json:"image_url,omitempty"`
}

func extractHostName(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "News"
	}
	host := strings.TrimPrefix(u.Hostname(), "www.")
	lower := strings.ToLower(host)
	switch {
	case strings.Contains(lower, "kompas.com"):
		return "Kompas.com"
	case strings.Contains(lower, "liputan6.com"):
		return "Liputan6.com"
	case strings.Contains(lower, "bisnis.com"):
		return "Bisnis.com"
	case strings.Contains(lower, "cnbcindonesia.com"):
		return "CNBC Indonesia"
	default:
		if host != "" {
			return host
		}
		return "News"
	}
}

func extractNewsEvidenceFromToolCalls(toolCalls []toolCallResult) []newsEvidenceItem {
	var evidence []newsEvidenceItem
	seenURLs := make(map[string]bool)

	// 1. Prioritize read_article tool calls (carries full article metadata: source, date, image, excerpt)
	for _, tc := range toolCalls {
		if tc.ToolName != "read_article" {
			continue
		}
		var resp struct {
			Articles []struct {
				URL         string `json:"url"`
				Title       string `json:"title"`
				Excerpt     string `json:"excerpt"`
				SiteName    string `json:"site_name"`
				ImageURL    string `json:"image_url"`
				PublishedAt string `json:"published_at"`
			} `json:"articles"`
		}
		if err := json.Unmarshal([]byte(tc.Result), &resp); err == nil {
			for _, a := range resp.Articles {
				if a.URL == "" || seenURLs[a.URL] {
					continue
				}
				seenURLs[a.URL] = true
				source := a.SiteName
				if source == "" {
					source = extractHostName(a.URL)
				}
				evidence = append(evidence, newsEvidenceItem{
					Title:       a.Title,
					Source:      source,
					URL:         a.URL,
					PublishedAt: a.PublishedAt,
					Excerpt:     a.Excerpt,
					ImageURL:    a.ImageURL,
				})
			}
		}
	}

	// 2. Secondary fallback: if read_article wasn't called or yielded no articles, check web_search
	if len(evidence) == 0 {
		for _, tc := range toolCalls {
			if tc.ToolName != "web_search" {
				continue
			}
			var resp struct {
				Results []struct {
					Title    string `json:"title"`
					URL      string `json:"url"`
					Content  string `json:"content"`
					ImageURL string `json:"image_url"`
				} `json:"results"`
				Images []string `json:"images"`
			}
			if err := json.Unmarshal([]byte(tc.Result), &resp); err == nil {
				for i, r := range resp.Results {
					if r.URL == "" || seenURLs[r.URL] {
						continue
					}
					seenURLs[r.URL] = true
					imgURL := r.ImageURL
					if imgURL == "" && i < len(resp.Images) {
						imgURL = resp.Images[i]
					}
					if imgURL == "" {
						reImg := regexp.MustCompile(`!\[.*?\]\((https?://[^\s)]+)\)`)
						if m := reImg.FindStringSubmatch(r.Content); len(m) > 1 {
							imgURL = m[1]
						}
					}
					evidence = append(evidence, newsEvidenceItem{
						Title:    r.Title,
						Source:   extractHostName(r.URL),
						URL:      r.URL,
						Excerpt:  r.Content,
						ImageURL: imgURL,
					})
				}
			}
		}
	}

	return evidence
}

func classifyReply(turn *orchestratorTurn) (contentType string, uiProps json.RawMessage) {
	if len(turn.PendingQuestions) > 0 {
		card := turn.Decision.Card
		if payload, err := json.Marshal(map[string]any{
			"questions": turn.PendingQuestions,
			"card":      card,
		}); err == nil {
			return "workflow_card", payload
		}
	}

	var chartPayloads []json.RawMessage
	for _, tc := range turn.AnalyzerToolCalls {
		if tc.ToolName == "get_portfolio_snapshot" || tc.ToolName == "get_stock_chart" {
			chartPayloads = append(chartPayloads, json.RawMessage(tc.Result))
		}
	}
	newsEvidence := extractNewsEvidenceFromToolCalls(turn.AnalyzerToolCalls)

	// If both charts and news evidence are present, deliver both via a composite payload
	// using "news" as the database-safe content_type enum
	if len(chartPayloads) > 0 && len(newsEvidence) > 0 {
		var chartData any = chartPayloads
		if len(chartPayloads) == 1 {
			chartData = chartPayloads[0]
		}
		if payload, err := json.Marshal(map[string]any{
			"charts": chartData,
			"news":   newsEvidence,
		}); err == nil {
			return "news", payload
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

	if len(newsEvidence) > 0 {
		if payload, err := json.Marshal(newsEvidence); err == nil {
			return "news", payload
		}
	}

	return "text", nil
}

func parseRouteDecision(raw string) (routeDecision, bool) {
	raw = stripFence(raw)

	var decision routeDecision
	if err := json.Unmarshal([]byte(raw), &decision); err != nil {
		return routeDecision{}, false
	}
	if decision.Path == "" {
		return routeDecision{}, false
	}
	return decision, true
}

func stripFence(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	return strings.TrimSpace(raw)
}

// parseNovaVerdict extracts the last fenced ```json block from Nova's reply
// and unmarshals it into a NovaVerdict. Everything before the block is
// Nova's normal prose (shown to the user unchanged). A missing or
// unparseable block fails closed — tradeable:false — never a green light.
func parseNovaVerdict(reply string) (*NovaVerdict, string) {
	lastIdx := strings.LastIndex(reply, "```json")
	if lastIdx == -1 {
		lastIdx = strings.LastIndex(reply, "```")
	}
	if lastIdx == -1 {
		return &NovaVerdict{
			Tradeable:  false,
			Reasoning:  "Nova reply did not contain a structured JSON verdict block.",
			Confidence: "low",
		}, reply
	}

	prose := strings.TrimSpace(reply[:lastIdx])
	block := stripFence(reply[lastIdx:])

	var verdict NovaVerdict
	if err := json.Unmarshal([]byte(block), &verdict); err != nil {
		slog.Warn("orchestrator: unmarshal nova verdict failed, failing closed", "error", err, "raw", block)
		return &NovaVerdict{
			Tradeable:  false,
			Reasoning:  "Unparseable Nova verdict block.",
			Confidence: "low",
		}, prose
	}

	return &verdict, prose
}

func drainAssistantThinking(stream *schema.StreamReader[*schema.Message], onThinking func(delta string)) (string, error) {
	defer stream.Close()
	var full strings.Builder
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if chunk.Content == "" {
			continue
		}
		full.WriteString(chunk.Content)
		if onThinking != nil {
			onThinking(chunk.Content)
		}
	}
	return full.String(), nil
}

// runRoleAgent drives one full agent turn (Nova or Comet) to completion,
// draining streamed output and surfacing tool-call events via callbacks.
func runRoleAgent(ctx context.Context, a adk.Agent, prompt string, allowIterationRecovery bool, onToolCall func(toolName, phase string), onThinking func(delta string)) (string, []toolCallResult, error) {
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

	iterator := a.Run(ctx, &adk.AgentInput{Messages: []*schema.Message{schema.UserMessage(prompt)}, EnableStreaming: true}, runOpts...)

	var lastAssistant, lastNonEmptyAssistant *schema.Message
	var toolCalls []toolCallResult
	for {
		event, ok := iterator.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			if allowIterationRecovery && strings.Contains(event.Err.Error(), "exceeds max iterations") {
				slog.WarnContext(ctx, "orchestrator: role agent reached max iterations, attempting recovery from partial state", "error", event.Err)
				break
			}
			return "", nil, event.Err
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		out := event.Output.MessageOutput

		var msg *schema.Message
		if out.IsStreaming {
			if out.Role == schema.Assistant {
				content, err := drainAssistantThinking(out.MessageStream, onThinking)
				if err != nil {
					return "", nil, fmt.Errorf("orchestrator: drain assistant thinking stream: %w", err)
				}
				msg = &schema.Message{Role: schema.Assistant, Content: content}
			} else {
				concatenated, err := schema.ConcatMessageStream(out.MessageStream)
				if err != nil {
					return "", nil, fmt.Errorf("orchestrator: concat message stream: %w", err)
				}
				msg = concatenated
			}
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
		return "", toolCalls, fmt.Errorf("orchestrator: agent produced tool calls but no final textual response")
	}
	return final.Content, toolCalls, nil
}
