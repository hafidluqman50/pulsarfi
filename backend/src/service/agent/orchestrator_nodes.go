package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/cloudwego/eino/schema"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
)

type tickerNote struct {
	resolved string
	problem  string
}

func (o *Orchestrator) resolveTicker(ctx context.Context, mentioned string, unanswered []string) ([]string, tickerNote) {
	mentioned = strings.ToUpper(strings.TrimSpace(mentioned))
	if mentioned == "" || o.Stocks == nil {
		return unanswered, tickerNote{}
	}

	stock, found, err := o.Stocks.FindByTickerOrIdxTicker(ctx, mentioned)
	if err != nil {
		slog.WarnContext(ctx, "orchestrator: ticker lookup failed, falling back to asking", "ticker", mentioned, "error", err)
		return unanswered, tickerNote{}
	}
	if !found && !strings.HasSuffix(mentioned, "P") {
		stock, found, _ = o.Stocks.FindByTickerOrIdxTicker(ctx, mentioned+"P")
	}
	if !found && strings.HasSuffix(mentioned, "P") {
		stock, found, _ = o.Stocks.FindByTickerOrIdxTicker(ctx, strings.TrimSuffix(mentioned, "P"))
	}

	if found {
		remaining := make([]string, 0, len(unanswered))
		for _, key := range unanswered {
			if key != "ticker" {
				remaining = append(remaining, key)
			}
		}
		return remaining, tickerNote{resolved: stock.Ticker}
	}

	var problem string
	if listed, err := o.Stocks.FindMarketReady(ctx); err == nil && len(listed) > 0 {
		tickers := make([]string, 0, len(listed))
		for _, s := range listed {
			tickers = append(tickers, s.Ticker)
		}
		problem = fmt.Sprintf("Ticker %s is not listed. Listed tokens: %s.", mentioned, strings.Join(tickers, ", "))
	} else {
		problem = fmt.Sprintf("Ticker %s is not listed.", mentioned)
	}

	for _, key := range unanswered {
		if key == "ticker" {
			return unanswered, tickerNote{problem: problem}
		}
	}
	return append([]string{"ticker"}, unanswered...), tickerNote{problem: problem}
}

// runRoute is Quasar's first call: read the prompt, decide whether this is
// pure conversation ("none") or a real Task, and if it's a Task, open it —
// both in Postgres and on-chain, immediately, regardless of is_actionable.
func (o *Orchestrator) runRoute(ctx context.Context, turn *orchestratorTurn) (*orchestratorTurn, error) {
	slog.InfoContext(ctx, "orchestrator: llm call", "step", "route", "model", o.RouteModelName)

	routeSystemMsgs := []*schema.Message{
		schema.SystemMessage(quasarRouteInstructions),
		schema.SystemMessage(currentTimeContext()),
	}

	// If an actionable task is currently pending and awaiting allowance approval, inject context so LLM knows
	if o.Tasks != nil && turn.Wallet != "" {
		if tasks, err := o.Tasks.FindByWallet(ctx, turn.Wallet); err == nil && len(tasks) > 0 {
			latest := tasks[0]
			if latest.IsActionable && latest.ArmedAt == nil && latest.Status == "pending" {
				routeSystemMsgs = append(routeSystemMsgs, schema.SystemMessage(fmt.Sprintf(
					"Active Unarmed Task Notice: Task T-%d is currently pending and awaiting user on-chain allowance approval. If the user's message is asking or demanding to execute or proceed with this trade now, do NOT create a new task (set path: none, is_actionable: false). In your reply, explain in the user's language that they must approve the permission and allowance on the Arm card above first.",
					latest.ID,
				)))
			}
		}
	}

	messages := append(routeSystemMsgs, turn.Messages...)
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

	// Step 1: Stock Validation Gate & Ticker Normalization
	rawMention := strings.TrimSpace(decision.MentionedTicker)
	unanswered, tickerNote := o.resolveTicker(ctx, rawMention, decision.Unanswered)
	if tickerNote.resolved != "" {
		turn.ResolvedTicker = tickerNote.resolved
		decision.MentionedTicker = tickerNote.resolved
		if rawMention != "" && rawMention != tickerNote.resolved {
			decision.Summary = strings.ReplaceAll(decision.Summary, rawMention, tickerNote.resolved)
			decision.RequestForAnalyzer = strings.ReplaceAll(decision.RequestForAnalyzer, rawMention, tickerNote.resolved)
			decision.RequestForExecutor = strings.ReplaceAll(decision.RequestForExecutor, rawMention, tickerNote.resolved)
		}
	}
	if tickerNote.problem != "" {
		turn.TickerProblem = tickerNote.problem
	}

	// Step 1 Gate: If this is an actionable trade path, validate stock exists
	if (decision.Path == pathExecutorOnly || decision.Path == pathAnalyzerThenExecutor) && turn.ResolvedTicker == "" {
		slog.InfoContext(ctx, "orchestrator: Step 1 gate: rejecting actionable task with unlisted ticker", "raw", rawMention, "problem", turn.TickerProblem)
		decision.Path = pathNeedsInput
		decision.IsActionable = false
		decision.Unanswered = unanswered
		turn.Decision = decision
	} else {
		decision.Unanswered = unanswered
		turn.Decision = decision
	}

	// If the router still needs input, compile the question set and divert to reply
	if decision.Path == pathNeedsInput {
		if len(decision.Questions) > 0 {
			turn.PendingQuestions = decision.Questions
		} else {
			shape := ParseTradeShape(decision.Shape)
			intake := IntakeContext{Shape: shape, Side: ParseSide(decision.Side)}
			fieldSets := [][]IntakeField{GetShapeFieldSet()}
			if o.AnalyzerIntake != nil {
				fieldSets = append(fieldSets, o.AnalyzerIntake(intake))
			}
			if o.ExecutorIntake != nil {
				fieldSets = append(fieldSets, o.ExecutorIntake(intake))
			}
			turn.PendingQuestions = CompileIntake(decision.Unanswered, fieldSets...)
		}
		return turn, nil
	}

	// Confirmation Gate: An actionable trade MUST have zero unanswered intake questions
	if decision.IsActionable {
		if len(decision.Unanswered) > 0 {
			slog.InfoContext(ctx, "orchestrator: confirmation gate: actionable trade has unanswered questions, diverting to HITL card",
				"count", len(decision.Unanswered))
			if len(decision.Questions) > 0 {
				turn.PendingQuestions = decision.Questions
			} else {
				shape := ParseTradeShape(decision.Shape)
				intake := IntakeContext{Shape: shape, Side: ParseSide(decision.Side)}
				fieldSets := [][]IntakeField{GetShapeFieldSet()}
				if o.AnalyzerIntake != nil {
					fieldSets = append(fieldSets, o.AnalyzerIntake(intake))
				}
				if o.ExecutorIntake != nil {
					fieldSets = append(fieldSets, o.ExecutorIntake(intake))
				}
				turn.PendingQuestions = CompileIntake(decision.Unanswered, fieldSets...)
			}
			decision.Path = pathNeedsInput
			decision.IsActionable = false
			turn.Decision = decision
			return turn, nil
		}
	}

	card := decision.Card
	triggerMap := map[string]any{
		"prompt": turn.RawPrompt,
		"card":   card,
	}
	if decision.BudgetIDRX != "" {
		triggerMap["confirmed_budget_idrx"] = decision.BudgetIDRX
		triggerMap["budget"] = decision.BudgetIDRX
	}
	if turn.ResolvedTicker != "" {
		triggerMap["resolved_ticker"] = turn.ResolvedTicker
	}
	triggerBytes, _ := json.Marshal(triggerMap)
	triggerDescription := string(triggerBytes)

	// Step 1: Open the Task in Postgres
	var task model.AgentTask
	if o.Tasks != nil {
		var err error
		rawPrompt := turn.RawPrompt
		task, err = o.Tasks.Create(ctx, repository.AgentTaskCreateInput{
			WalletAddress:      turn.Wallet,
			RawPrompt:          &rawPrompt,
			Summary:            decision.Summary,
			TriggerDescription: &triggerDescription,
			IsActionable:       decision.IsActionable,
			SourceMessageID:    turn.SourceMessageID,
		})
		if err != nil {
			return turn, fmt.Errorf("orchestrator: create task in db: %w", err)
		}
		turn.TaskID = task.ID
	}

	// Step 2: Open the Task on-chain via Quasar's operator wallet
	var onChainTaskID uint64
	if o.Chain != nil {
		promptHash := crypto.Keccak256Hash([]byte(turn.RawPrompt))
		id, err := o.Chain.CreateTask(ctx, turn.Wallet, decision.IsActionable, decision.Summary, promptHash)
		if err != nil {
			return turn, fmt.Errorf("orchestrator: on-chain createTask: %w", err)
		}
		onChainTaskID = id
		if o.Tasks != nil {
			if err := o.Tasks.SetOnChainTaskID(ctx, task.ID, int64(id)); err != nil {
				return turn, fmt.Errorf("orchestrator: persist onChainTaskID: %w", err)
			}
		}
	}

	// Step 3: Build the RunContext
	recorder, err := NewSubTaskRecorder(ctx, o.SubTasks, o.Chain, onChainTaskID, task.ID, decision.Summary, turn.Wallet)
	if err != nil {
		return turn, fmt.Errorf("orchestrator: build subtask recorder: %w", err)
	}
	if turn.OnSubTask != nil {
		recorder.OnRecord = turn.OnSubTask
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
		OnSubTaskStarted: turn.OnSubTaskStarted,
	}

	return turn, nil
}

// runAnalyze delegates market intelligence gathering to Nova
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
	slog.InfoContext(ctx, "orchestrator: llm call", "step", "gather_evidence", "agent", "analyzer", "model", o.AnalyzerModelName, "task_id", turn.TaskID)
	reply, toolCalls, err := runRoleAgent(WithRunContext(ctx, turn.runCtx), o.Analyzer, request,
		func(toolName, phase string) {
			if turn.OnToolCall != nil {
				turn.OnToolCall("analyzer", toolName, phase)
			}
		},
		func(delta string) {
			if turn.OnThinking != nil {
				turn.OnThinking("analyzer", delta)
			}
		},
	)
	if err != nil {
		if _, recErr := turn.runCtx.Recorder.Record(ctx, "analyzer", "gather_evidence", "failed", err.Error(), turn.Decision.Label, nil); recErr != nil {
			slog.ErrorContext(ctx, "orchestrator: record gather_evidence failed step", "error", recErr)
		}
		return turn, fmt.Errorf("orchestrator: analyzer agent run: %w", err)
	}
	if _, err := turn.runCtx.Recorder.Record(ctx, "analyzer", "gather_evidence", "done", reply, turn.Decision.Label, nil); err != nil {
		return turn, fmt.Errorf("orchestrator: record gather_evidence: %w", err)
	}

	turn.AnalyzerReply = reply
	turn.AnalyzerToolCalls = toolCalls
	return turn, nil
}

// runExecute wraps Comet's on-chain execution
func (o *Orchestrator) runExecute(ctx context.Context, turn *orchestratorTurn) (*orchestratorTurn, error) {
	if turn.runCtx == nil {
		return turn, fmt.Errorf("orchestrator: execute reached with no open task")
	}

	if turn.OnSubTaskStarted != nil {
		turn.OnSubTaskStarted("supervisor", "route_to_executor", turn.Decision.Label)
	}
	if _, err := turn.runCtx.Recorder.Record(ctx, "supervisor", "route_to_executor", "done", "Routed to Comet for execution.", turn.Decision.Label, nil); err != nil {
		return turn, fmt.Errorf("orchestrator: record route_to_executor: %w", err)
	}

	request := turn.Decision.RequestForExecutor
	if request == "" {
		request = turn.RawPrompt
	}
	if turn.AnalyzerReply != "" {
		request = fmt.Sprintf("User instruction:\n%s\n\nNova's findings:\n%s\n\nBased on Nova's findings above, decide and execute the trade.", request, turn.AnalyzerReply)
	}

	tipBefore := turn.runCtx.Recorder.TerminalHash()
	slog.InfoContext(ctx, "orchestrator: llm call", "step", "decide", "agent", "executor", "model", o.ExecutorModelName, "task_id", turn.TaskID)
	reply, _, err := runRoleAgent(WithRunContext(ctx, turn.runCtx), o.Executor, request,
		func(toolName, phase string) {
			if turn.OnToolCall != nil {
				turn.OnToolCall("executor", toolName, phase)
			}
		},
		func(delta string) {
			if turn.OnThinking != nil {
				turn.OnThinking("executor", delta)
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

// runReply generates Quasar's closing message to the user.
func (o *Orchestrator) runReply(ctx context.Context, turn *orchestratorTurn) (*orchestratorTurn, error) {
	var extra strings.Builder

	if turn.Decision.IsActionable && turn.TaskID > 0 {
		extra.WriteString(fmt.Sprintf("Arm Card for Task T-%d is displayed below your message. In a single brief sentence in the user's language, direct the user to review the budget and click Arm on the card below. Do not include market analysis or trading advice.\n\n", turn.TaskID))
	} else {
		if turn.AnalyzerReply != "" {
			extra.WriteString("Nova findings:\n" + turn.AnalyzerReply + "\n\n")
		}
		if turn.ExecutorReply != "" {
			extra.WriteString("Comet decision:\n" + turn.ExecutorReply + "\n\n")
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

	messages := append([]*schema.Message{schema.SystemMessage(quasarReplyInstructions), schema.SystemMessage(currentTimeContext())}, turn.Messages...)
	if extra.Len() > 0 {
		messages = append(messages, schema.SystemMessage(extra.String()))
	}

	slog.InfoContext(ctx, "orchestrator: llm call", "step", "reply", "model", o.ReplyModelName, "task_id", turn.TaskID)
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
