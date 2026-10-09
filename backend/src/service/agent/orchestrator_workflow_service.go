package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"github.com/horizonlabs/pulsarfi-backend/src/contracts"
	dbmodel "github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
)

func (o *Orchestrator) newRunContext(chatID uuid.UUID, wallet, rawPrompt string, sourceMessageID *int64) *RunContext {
	live := &liveSubTasks{topic: ChatStreamTopic(chatID)}
	return &RunContext{
		Wallet:           wallet,
		ChatID:           chatID,
		RawPrompt:        rawPrompt,
		SourceMessageID:  sourceMessageID,
		OnSubTaskStarted: live.started,
		OnSubTaskFailed:  live.failed,
		live:             live,
	}
}

// restoreFromChat gives a resumed questions turn its Task back: the chat's
// most recent Task, found through the messages it was born from.
func (o *Orchestrator) restoreFromChat(ctx context.Context, rc *RunContext) error {
	messages, err := o.ChatMessages.FindByChatID(ctx, rc.ChatID)
	if err != nil {
		return fmt.Errorf("orchestrator: load chat messages: %w", err)
	}
	messageIDs := make([]int64, len(messages))
	for i, message := range messages {
		messageIDs[i] = message.ID
	}
	task, found, err := o.Tasks.FindByChatMessageIDs(ctx, messageIDs)
	if err != nil {
		return fmt.Errorf("orchestrator: find chat task: %w", err)
	}
	if !found {
		return fmt.Errorf("%w: chat %s is paused but has no task", ErrNoOpenTask, rc.ChatID)
	}
	return o.restoreRunContext(ctx, rc, task)
}

// restoreRunContext rebuilds what RunContext cannot carry across requests
// (it holds a live recorder): the Task identifiers, the recorder continuing
// the same hash chain, and the decision stored on the Task row.
func (o *Orchestrator) restoreRunContext(ctx context.Context, rc *RunContext, task dbmodel.AgentTask) error {
	var onChainID uint64
	if task.OnChainTaskID != nil {
		onChainID = uint64(*task.OnChainTaskID)
	}
	summary := ""
	if task.Summary != nil {
		summary = *task.Summary
	}
	recorder, err := NewSubTaskRecorder(ctx, o.SubTasks, o.Chain, onChainID, task.ID, summary, rc.Wallet)
	if err != nil {
		return fmt.Errorf("orchestrator: rebuild subtask recorder: %w", err)
	}
	recorder.OnRecord = rc.live.done
	rc.TaskID = task.ID
	rc.OnChainTaskID = task.OnChainTaskID
	rc.Recorder = recorder
	rc.Decision = decisionFromTask(task)
	return nil
}

func decisionFromTask(task dbmodel.AgentTask) RouteDecision {
	decision := RouteDecision{IsActionable: task.IsActionable}
	if task.Summary != nil {
		decision.Summary = *task.Summary
		decision.Label = *task.Summary
	}
	if task.TriggerDescription == nil {
		return decision
	}
	var stored struct {
		Shape          string                  `json:"shape"`
		Side           string                  `json:"side"`
		ResolvedTicker string                  `json:"resolved_ticker"`
		BudgetIDRX     string                  `json:"confirmed_budget_idrx"`
		SellAmount     string                  `json:"sell_amount"`
		Card           *contracts.CardContract `json:"card"`
	}
	if err := json.Unmarshal([]byte(*task.TriggerDescription), &stored); err != nil {
		return decision
	}
	decision.Shape = stored.Shape
	decision.Side = stored.Side
	decision.MentionedTicker = stored.ResolvedTicker
	decision.BudgetIDRX = stored.BudgetIDRX
	decision.SellAmount = stored.SellAmount
	decision.Card = stored.Card
	return decision
}

func (o *Orchestrator) cardOfTask(ctx context.Context, taskID int64) *contracts.CardContract {
	if taskID == 0 || o.Tasks == nil {
		return nil
	}
	task, found, err := o.Tasks.FindByID(ctx, taskID)
	if err != nil || !found {
		return nil
	}
	return decisionFromTask(task).Card
}

// CommitTask opens the Task — Postgres row, on-chain createTask, and the
// Sub Task recorder — the first time Quasar recognizes an intent, even while
// parameters are still missing. Every Task is on-chain, so this runs for
// every request except plain conversation. Later rounds only UpdateTask.
func (o *Orchestrator) CommitTask(ctx context.Context, rc *RunContext, decision RouteDecision) error {
	decision = withCardDefaults(decision)

	triggerDescription, err := o.buildDecisionTrigger(ctx, rc.RawPrompt, decision)
	if err != nil {
		return err
	}
	task, err := o.persistTask(ctx, rc, decision, triggerDescription)
	if err != nil {
		return err
	}
	rc.TaskID = task.ID

	onChainTaskID, err := o.armOnChain(ctx, rc, decision, task.ID)
	if err != nil {
		return err
	}
	if err := o.attachRecorder(ctx, rc, decision, onChainTaskID, task.ID); err != nil {
		return err
	}
	rc.Decision = decision

	if recErr := o.recordDecision(ctx, rc, decision, true); recErr != nil {
		slog.WarnContext(ctx, "orchestrator: record understand/route sub tasks failed", "error", recErr)
	}
	return nil
}

// UpdateTask patches an already-open Task's settled fields once the user's
// answers complete them — never a second on-chain createTask, only the
// Postgres summary, trigger description and is_actionable are refreshed, and
// the on-chain actionable flag is flipped the first time a round settles.
func (o *Orchestrator) UpdateTask(ctx context.Context, rc *RunContext, decision RouteDecision) error {
	if rc.TaskID == 0 {
		return ErrNoOpenTask
	}
	wasActionable := rc.Decision.IsActionable
	if decision.MentionedTicker == "" {
		decision.MentionedTicker = rc.Decision.MentionedTicker
	}
	if decision.Card == nil {
		decision.Card = rc.Decision.Card
	}
	decision = withCardDefaults(decision)

	task, found, err := o.Tasks.FindByID(ctx, rc.TaskID)
	if err != nil || !found {
		return fmt.Errorf("orchestrator: load task %d: %w", rc.TaskID, err)
	}
	rawPrompt := rc.RawPrompt
	if task.RawPrompt != nil {
		rawPrompt = *task.RawPrompt
	}
	triggerDescription, err := o.buildDecisionTrigger(ctx, rawPrompt, decision)
	if err != nil {
		return err
	}

	// The Task's on-chain isActionable flag is set once, at createTask, and
	// has no other setter besides markActionable — flip it the first time a
	// round settles into an actionable decision, never again afterward.
	if decision.IsActionable && !wasActionable && o.Chain != nil && rc.OnChainTaskID != nil {
		if err := o.Chain.MarkActionable(ctx, uint64(*rc.OnChainTaskID)); err != nil {
			return fmt.Errorf("orchestrator: on-chain markActionable: %w", err)
		}
	}
	if err := o.Tasks.UpdateDecision(ctx, rc.TaskID, decision.Summary, triggerDescription, decision.IsActionable); err != nil {
		return fmt.Errorf("orchestrator: update task decision: %w", err)
	}
	rc.Decision = decision

	if recErr := o.recordDecision(ctx, rc, decision, false); recErr != nil {
		slog.WarnContext(ctx, "orchestrator: record route sub task failed", "error", recErr)
	}
	return nil
}

func withCardDefaults(decision RouteDecision) RouteDecision {
	if decision.Card == nil {
		decision.Card = &contracts.CardContract{}
	}
	if decision.Card.TaskBadge == "" {
		decision.Card.TaskBadge = "Task T-{id}"
	}
	return decision
}

func (o *Orchestrator) buildDecisionTrigger(ctx context.Context, rawPrompt string, decision RouteDecision) (string, error) {
	if decision.Summary == "" {
		decision.Summary = defaultSummary(decision.Side, decision, decision.MentionedTicker)
	}
	stockContractAddress := o.resolveStockContractAddress(ctx, decision.Side, decision.MentionedTicker)
	triggerDescription, err := buildTriggerDescription(rawPrompt, decision, stockContractAddress)
	if err != nil {
		return "", fmt.Errorf("orchestrator: marshal trigger description: %w", err)
	}
	return triggerDescription, nil
}

// recordDecision writes Quasar's own reasoning as real, permanent Sub Tasks
// tied to the Task — "understand_request" fires exactly once, the first time
// an intent is recognized; "route_decision" fires every round, so the
// on-chain trail shows how the Task evolved as the user answered.
func (o *Orchestrator) recordDecision(ctx context.Context, rc *RunContext, decision RouteDecision, isFirst bool) error {
	if rc.Recorder == nil {
		return nil
	}
	if isFirst {
		rc.OnSubTaskStarted("supervisor", "understand_request", decision.Label)
		if _, err := rc.Recorder.Record(ctx, "supervisor", "understand_request", "done", decision.Summary, decision.Label, nil); err != nil {
			return fmt.Errorf("orchestrator: record understand_request: %w", err)
		}
	}
	verb := "Task updated"
	if isFirst {
		verb = "Task opened"
	}
	rc.OnSubTaskStarted("supervisor", "route_decision", decision.Label)
	if _, err := rc.Recorder.Record(ctx, "supervisor", "route_decision", "done", fmt.Sprintf("%s. Actionable: %t", verb, decision.IsActionable), decision.Label, nil); err != nil {
		return fmt.Errorf("orchestrator: record route_decision: %w", err)
	}
	return nil
}

func defaultSummary(side string, decision RouteDecision, ticker string) string {
	if side == "sell" {
		return fmt.Sprintf("Sell %s %s", decision.SellAmount, ticker)
	}
	return fmt.Sprintf("Buy %s IDRX of %s", decision.BudgetIDRX, ticker)
}

// resolveStockContractAddress returns the on-chain ERC-20 address for a sell
// task's stock token. Returns "" (non-fatal) when: Stocks is nil, ticker is
// empty, the ticker is not in the catalog, or the stock has no deployed
// contract yet. The caller writes the result into trigger_description only
// when non-empty.
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
// struct-to-JSON mapping, no lookups. stockContractAddress is non-empty only
// for sell tasks; the caller resolves it before calling here so this
// function stays IO-free.
func buildTriggerDescription(rawPrompt string, decision RouteDecision, stockContractAddress string) (string, error) {
	cleanShape := strings.ToLower(strings.TrimSpace(decision.Shape))
	side := decision.Side

	triggerMap := map[string]any{
		"prompt": rawPrompt,
		"card":   decision.Card,
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
	if decision.MentionedTicker != "" {
		triggerMap["resolved_ticker"] = decision.MentionedTicker
		triggerMap["stock_ticker"] = decision.MentionedTicker
		if side == "sell" {
			triggerMap["token_symbol"] = decision.MentionedTicker
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

func (o *Orchestrator) persistTask(ctx context.Context, rc *RunContext, decision RouteDecision, triggerDescription string) (dbmodel.AgentTask, error) {
	if o.Tasks == nil {
		return dbmodel.AgentTask{}, nil
	}
	rawPrompt := rc.RawPrompt
	summary := decision.Summary
	if summary == "" {
		summary = defaultSummary(decision.Side, decision, decision.MentionedTicker)
	}
	task, err := o.Tasks.Create(ctx, repository.AgentTaskCreateInput{
		WalletAddress:      rc.Wallet,
		RawPrompt:          &rawPrompt,
		Summary:            summary,
		TriggerDescription: &triggerDescription,
		IsActionable:       decision.IsActionable,
		SourceMessageID:    rc.SourceMessageID,
	})
	if err != nil {
		return dbmodel.AgentTask{}, fmt.Errorf("orchestrator: create task in db: %w", err)
	}
	return task, nil
}

func (o *Orchestrator) armOnChain(ctx context.Context, rc *RunContext, decision RouteDecision, taskID int64) (uint64, error) {
	if o.Chain == nil {
		return 0, nil
	}
	promptHash := crypto.Keccak256Hash([]byte(rc.RawPrompt))
	summary := decision.Summary
	if summary == "" {
		summary = defaultSummary(decision.Side, decision, decision.MentionedTicker)
	}
	onChainTaskID, err := o.Chain.CreateTask(ctx, rc.Wallet, decision.IsActionable, summary, promptHash)
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

func (o *Orchestrator) attachRecorder(ctx context.Context, rc *RunContext, decision RouteDecision, onChainTaskID uint64, taskID int64) error {
	recorder, err := NewSubTaskRecorder(ctx, o.SubTasks, o.Chain, onChainTaskID, taskID, decision.Summary, rc.Wallet)
	if err != nil {
		return fmt.Errorf("orchestrator: build subtask recorder: %w", err)
	}
	recorder.OnRecord = rc.live.done
	if onChainTaskID != 0 {
		onChainID := int64(onChainTaskID)
		rc.OnChainTaskID = &onChainID
	}
	rc.Recorder = recorder
	return nil
}

// NovaVerdict is Nova's structured, machine-parsed conclusion on an
// actionable request. It is advisory only: Comet is given it as context for
// sizing and timing, but the user's own instruction and approved budget are
// the final authority.
type NovaVerdict struct {
	Tradeable  bool   `json:"tradeable"`
	EntryPrice string `json:"entry_price"`
	ExitPrice  string `json:"exit_price,omitempty"`
	Confidence string `json:"confidence"`
	Reasoning  string `json:"reasoning"`
}

// FormatNovaVerdict renders the verdict as plain lines Quasar can pass along
// to Comet in its request, since Comet only ever sees what Quasar writes.
func FormatNovaVerdict(verdict *NovaVerdict) string {
	if verdict == nil {
		return ""
	}
	return fmt.Sprintf(
		"Nova's structured verdict:\n- Tradeable: %v\n- Entry price: %s\n- Confidence: %s\n- Reasoning: %s",
		verdict.Tradeable, verdict.EntryPrice, verdict.Confidence, verdict.Reasoning,
	)
}

// BuildAnalyzerRequest is the request Nova receives: the user's instruction
// plus, for an actionable request, the ticker verification and the verdict
// block Nova must end its reply with.
func BuildAnalyzerRequest(userInstruction, shape, ticker, side string, isActionable bool) string {
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

// ParseNovaVerdict extracts the last fenced ```json block from Nova's reply
// and unmarshals it into a NovaVerdict. Everything before the block is
// Nova's normal prose (shown to the user unchanged). A missing or
// unparseable block fails closed — tradeable:false — never a green light.
func ParseNovaVerdict(reply string) (*NovaVerdict, string) {
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

func stripFence(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	return strings.TrimSpace(raw)
}
