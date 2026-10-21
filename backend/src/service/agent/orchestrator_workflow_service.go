package agent

import (
	"context"
	"encoding/json"
	"fmt"
	agentcore "github.com/horizonlabs/pulsarfi-backend/src/app/core/agent"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/subtask"
	"log/slog"
	"strings"

	"github.com/ethereum/go-ethereum/crypto"
	"github.com/horizonlabs/pulsarfi-backend/src/contracts"
	dbmodel "github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
)

func decisionFromTask(task dbmodel.AgentTask) agentcore.RouteDecision {
	decision := agentcore.RouteDecision{IsActionable: task.IsActionable}
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

func (orch *Orchestrator) cardOfTask(ctx context.Context, taskID int64) *contracts.CardContract {
	if taskID == 0 || orch.Tasks == nil {
		return nil
	}
	task, found, err := orch.Tasks.FindByID(ctx, taskID)
	if err != nil || !found {
		return nil
	}
	return decisionFromTask(task).Card
}

// CommitTask opens the Task — Postgres row, on-chain createTask, and the
// Sub Task recorder — the first time Quasar recognizes an intent, even while
// parameters are still missing. Every Task is on-chain, so this runs for
// every request except plain conversation. Later rounds only UpdateTask.
func (orch *Orchestrator) CommitTask(ctx context.Context, run *agentcore.RunContext, decision agentcore.RouteDecision) error {
	decision = withCardDefaults(decision)

	triggerDescription, err := orch.buildDecisionTrigger(ctx, run.RawPrompt, decision)
	if err != nil {
		return err
	}
	task, err := orch.persistTask(ctx, run, decision, triggerDescription)
	if err != nil {
		return err
	}
	run.TaskID = task.ID

	onChainTaskID, err := orch.armOnChain(ctx, run, decision, task.ID)
	if err != nil {
		return err
	}
	if err := orch.attachRecorder(ctx, run, decision, onChainTaskID, task.ID); err != nil {
		return err
	}
	run.Decision = decision

	if recErr := orch.recordDecision(ctx, run, decision, true); recErr != nil {
		slog.WarnContext(ctx, "orchestrator: record understand/route sub tasks failed", "error", recErr)
	}
	return nil
}

// UpdateTask patches an already-open Task's settled fields once the user's
// answers complete them — never a second on-chain createTask, only the
// Postgres summary, trigger description and is_actionable are refreshed, and
// the on-chain actionable flag is flipped the first time a round settles.
func (orch *Orchestrator) UpdateTask(ctx context.Context, run *agentcore.RunContext, decision agentcore.RouteDecision) error {
	if run.TaskID == 0 {
		return agentcore.ErrNoOpenTask
	}
	wasActionable := run.Decision.IsActionable
	if decision.MentionedTicker == "" {
		decision.MentionedTicker = run.Decision.MentionedTicker
	}
	if decision.Card == nil {
		decision.Card = run.Decision.Card
	}
	decision = withCardDefaults(decision)

	task, found, err := orch.Tasks.FindByID(ctx, run.TaskID)
	if err != nil || !found {
		return fmt.Errorf("orchestrator: load task %d: %w", run.TaskID, err)
	}
	rawPrompt := run.RawPrompt
	if task.RawPrompt != nil {
		rawPrompt = *task.RawPrompt
	}
	triggerDescription, err := orch.buildDecisionTrigger(ctx, rawPrompt, decision)
	if err != nil {
		return err
	}

	// The Task's on-chain isActionable flag is set once, at createTask, and
	// has no other setter besides markActionable — flip it the first time a
	// round settles into an actionable decision, never again afterward.
	if decision.IsActionable && !wasActionable && orch.Chain != nil && run.OnChainTaskID != nil {
		if err := orch.Chain.MarkActionable(ctx, uint64(*run.OnChainTaskID)); err != nil {
			return fmt.Errorf("orchestrator: on-chain markActionable: %w", err)
		}
	}
	if err := orch.Tasks.UpdateDecision(ctx, run.TaskID, decision.Summary, triggerDescription, decision.IsActionable); err != nil {
		return fmt.Errorf("orchestrator: update task decision: %w", err)
	}
	run.Decision = decision

	if recErr := orch.recordDecision(ctx, run, decision, false); recErr != nil {
		slog.WarnContext(ctx, "orchestrator: record route sub task failed", "error", recErr)
	}
	return nil
}

func withCardDefaults(decision agentcore.RouteDecision) agentcore.RouteDecision {
	if decision.Card == nil {
		decision.Card = &contracts.CardContract{}
	}
	if decision.Card.TaskBadge == "" {
		decision.Card.TaskBadge = "Task T-{id}"
	}
	return decision
}

func (orch *Orchestrator) buildDecisionTrigger(ctx context.Context, rawPrompt string, decision agentcore.RouteDecision) (string, error) {
	if decision.Summary == "" {
		decision.Summary = defaultSummary(decision.Side, decision, decision.MentionedTicker)
	}
	stockContractAddress := orch.resolveStockContractAddress(ctx, decision.Side, decision.MentionedTicker)
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
func (orch *Orchestrator) recordDecision(ctx context.Context, run *agentcore.RunContext, decision agentcore.RouteDecision, isFirst bool) error {
	if run.Recorder == nil {
		return nil
	}
	if isFirst {
		run.Stream.StepStarted("supervisor", "understand_request", decision.Label)
		if _, err := run.Recorder.Record(ctx, "supervisor", "understand_request", "done", decision.Summary, decision.Label, nil); err != nil {
			return fmt.Errorf("orchestrator: record understand_request: %w", err)
		}
	}
	verb := "Task updated"
	if isFirst {
		verb = "Task opened"
	}
	run.Stream.StepStarted("supervisor", "route_decision", decision.Label)
	if _, err := run.Recorder.Record(ctx, "supervisor", "route_decision", "done", fmt.Sprintf("%s. Actionable: %t", verb, decision.IsActionable), decision.Label, nil); err != nil {
		return fmt.Errorf("orchestrator: record route_decision: %w", err)
	}
	return nil
}

func defaultSummary(side string, decision agentcore.RouteDecision, ticker string) string {
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
func (orch *Orchestrator) resolveStockContractAddress(ctx context.Context, side, ticker string) string {
	if side != "sell" || ticker == "" || orch.Stocks == nil {
		return ""
	}
	stock, found, err := orch.Stocks.FindByTickerOrIdxTicker(ctx, ticker)
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
func buildTriggerDescription(rawPrompt string, decision agentcore.RouteDecision, stockContractAddress string) (string, error) {
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

func (orch *Orchestrator) persistTask(ctx context.Context, run *agentcore.RunContext, decision agentcore.RouteDecision, triggerDescription string) (dbmodel.AgentTask, error) {
	if orch.Tasks == nil {
		return dbmodel.AgentTask{}, nil
	}
	rawPrompt := run.RawPrompt
	summary := decision.Summary
	if summary == "" {
		summary = defaultSummary(decision.Side, decision, decision.MentionedTicker)
	}
	task, err := orch.Tasks.Create(ctx, repository.AgentTaskCreateInput{
		WalletAddress:      run.Wallet,
		RawPrompt:          &rawPrompt,
		Summary:            summary,
		TriggerDescription: &triggerDescription,
		IsActionable:       decision.IsActionable,
		SourceMessageID:    run.SourceMessageID,
	})
	if err != nil {
		return dbmodel.AgentTask{}, fmt.Errorf("orchestrator: create task in db: %w", err)
	}
	return task, nil
}

func (orch *Orchestrator) armOnChain(ctx context.Context, run *agentcore.RunContext, decision agentcore.RouteDecision, taskID int64) (uint64, error) {
	if orch.Chain == nil {
		return 0, nil
	}
	promptHash := crypto.Keccak256Hash([]byte(run.RawPrompt))
	summary := decision.Summary
	if summary == "" {
		summary = defaultSummary(decision.Side, decision, decision.MentionedTicker)
	}
	onChainTaskID, err := orch.Chain.CreateTask(ctx, run.Wallet, decision.IsActionable, summary, promptHash)
	if err != nil {
		return 0, fmt.Errorf("orchestrator: on-chain createTask: %w", err)
	}
	if orch.Tasks != nil {
		if err := orch.Tasks.SetOnChainTaskID(ctx, taskID, int64(onChainTaskID)); err != nil {
			return 0, fmt.Errorf("orchestrator: persist onChainTaskID: %w", err)
		}
	}
	return onChainTaskID, nil
}

func (orch *Orchestrator) attachRecorder(ctx context.Context, run *agentcore.RunContext, decision agentcore.RouteDecision, onChainTaskID uint64, taskID int64) error {
	recorder, err := subtask.NewSubTaskRecorder(ctx, orch.SubTasks, orch.Chain, onChainTaskID, taskID, decision.Summary, run.Wallet)
	if err != nil {
		return fmt.Errorf("orchestrator: build subtask recorder: %w", err)
	}
	recorder.OnRecord = run.Stream.StepDone
	if onChainTaskID != 0 {
		onChainID := int64(onChainTaskID)
		run.OnChainTaskID = &onChainID
	}
	run.Recorder = recorder
	return nil
}
