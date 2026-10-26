package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	agentcore "github.com/horizonlabs/pulsarfi-backend/src/app/core/agent"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/executor"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/orchestrator"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/subtask"
	"log/slog"
	"math/big"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
	"github.com/horizonlabs/pulsarfi-backend/src/contracts"
	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
)

type AgentContractClient interface {
	CreateTask(ctx context.Context, owner string, isActionable bool, summary string, promptHash [32]byte) (onChainTaskID uint64, err error)
	GrantTradePermission(ctx context.Context, onChainTaskID uint64, totalBudget string, duration time.Duration, maxAmountPerTrade string, cooldownInterval uint32) error
	RecordSubTasks(ctx context.Context, onChainTaskID uint64, rows []model.AgentSubTask) (onChainSubTaskIDs []uint64, txHash string, err error)
	CancelTask(ctx context.Context, onChainTaskID uint64) error
	TradePermissionRemaining(ctx context.Context, onChainTaskID uint) (string, error)
	SubmitTradePair(ctx context.Context, onChainTaskID, subTaskID uint64, token common.Address, ticker string, side contracts.TradeSide, amount, minOut *big.Int, summary string, reasoningHash [32]byte, grantTotalBudget string, grantDurationSec int64, grantMaxPerTrade string, grantCooldown uint32, gasLimit uint64) (armHash, executeHash string, nonce uint64, err error)
	WaitTradeByHash(ctx context.Context, txHash string) (*agentcore.ExecuteTradeOutput, error)
}

// TaskService is the Task-layer surface HTTP handlers call. It is a thin
// signal/authorization layer only — it never builds an LLM prompt or
// invokes an agent directly. Actually executing a trade (ExecuteTask) does
// nothing but verify authorization/arm state and trigger
// Orchestrator.ResumeExecute, which owns all of that
// (docs/plans/quasar-clean-routing-scalp-refactor.md §2/§3.1).
type TaskService struct {
	Tasks        *repository.AgentTaskRepository
	SubTasks     *repository.AgentSubTaskRepository
	Trades       *repository.AgentTradeRepository
	ChatMessages *repository.AgentChatMessageRepository
	Chats        *repository.AgentChatRepository
	Chain        AgentContractClient
	Executor     adk.Agent
	Orchestrator *orchestrator.Orchestrator

	Stocks            executor.StockLookup
	Prices            executor.SpotPriceReader
	StockTransactions *repository.StockTransactionRepository
	ExecutionGasLimit uint64
	InstantExecution  bool
}

func (s *TaskService) ListTasks(ctx context.Context, walletAddress string) ([]model.AgentTask, error) {
	return s.Tasks.FindByWallet(ctx, strings.ToLower(walletAddress))
}

// activityFeedLimit caps the Activity Log to the most recent steps across
// every one of the wallet's own Tasks — a live feed, not a paginated
// archive; the full chain for any one Task is still available in full via
// GetReasoningChain.
const activityFeedLimit = 200

func (s *TaskService) GetActivity(ctx context.Context, walletAddress string) ([]model.AgentSubTask, error) {
	return s.SubTasks.FindByOwnerWallet(ctx, strings.ToLower(walletAddress), activityFeedLimit)
}

func (s *TaskService) GetReasoningChain(ctx context.Context, taskID int64) ([]model.AgentSubTask, error) {
	_, found, err := s.Tasks.FindByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, agentcore.ErrTaskNotFound
	}
	return s.SubTasks.FindByTaskID(ctx, taskID)
}

func (s *TaskService) GetTrades(ctx context.Context, taskID int64, walletAddress string) ([]model.AgentTrade, error) {
	task, found, err := s.Tasks.FindByID(ctx, taskID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, agentcore.ErrTaskNotFound
	}
	if !strings.EqualFold(task.WalletAddress, walletAddress) {
		return nil, agentcore.ErrWalletMismatch
	}
	return s.Trades.FindByTaskID(ctx, taskID)
}

func (s *TaskService) recordSubTasksBatch(ctx context.Context, onChainTaskID int64, rows []model.AgentSubTask) error {
	if len(rows) == 0 {
		return nil
	}
	_, txHash, err := s.Chain.RecordSubTasks(ctx, uint64(onChainTaskID), rows)
	if err != nil {
		return err
	}
	ids := make([]int64, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	return s.SubTasks.MarkRecordedOnChain(ctx, ids, txHash)
}

type ArmTaskInput struct {
	TotalBudget       string `json:"total_budget"`
	DurationSec       int64  `json:"duration_sec"`
	TokenAddress      string `json:"token_address"`
	MaxAmountPerTrade string `json:"max_amount_per_trade,omitempty"`
	CooldownInterval  uint32 `json:"cooldown_interval,omitempty"`
	IsRecurring       bool   `json:"is_recurring,omitempty"`
}

type ArmTaskResult struct {
	OnChainTaskID int64             `json:"on_chain_task_id"`
	TokenAddress  string            `json:"token_address,omitempty"`
	TotalBudget   string            `json:"total_budget,omitempty"`
	Status        string            `json:"status,omitempty"`
	TxHash        string            `json:"tx_hash,omitempty"`
	Trade         *model.AgentTrade `json:"trade,omitempty"`
}

// ArmTask is the one call that moves a Task on-chain into an executable
// state: grantTradePermission only (createTask already happened the instant
// Quasar committed the Task — see commitTask in
// orchestrator_workflow_service.go). Calling CreateTask again here would
// open a second, orphaned on-chain Task and silently rebind Postgres to it.
func (s *TaskService) ArmTask(ctx context.Context, taskID int64, wallet string, input ArmTaskInput) (ArmTaskResult, error) {
	task, found, err := s.Tasks.FindByID(ctx, taskID)
	if err != nil {
		return ArmTaskResult{}, err
	}
	if !found {
		return ArmTaskResult{}, agentcore.ErrTaskNotFound
	}
	if !strings.EqualFold(task.WalletAddress, wallet) {
		return ArmTaskResult{}, agentcore.ErrWalletMismatch
	}
	if task.OnChainTaskID == nil {
		return ArmTaskResult{}, fmt.Errorf("agent: task %d has no on-chain id — every Task must already be created on-chain by the time it reaches Arm, this should never happen", taskID)
	}
	onChainTaskID := uint64(*task.OnChainTaskID)

	// Instant path: the Arm Card's on-chain anchor is already recorded, so
	// this call only claims the Task, signs grant+execute, sends both in one
	// batch, waits for the execute receipt and writes the trade rows. The
	// legacy grant/execute flow below stays for everything else.
	if s.InstantExecution && task.IsActionable && !input.IsRecurring {
		if decide, ok := executionDecideOf(task); ok {
			if task.Status != "pending" {
				return s.existingTradeOrConflict(ctx, taskID, wallet)
			}
			claimed, claimErr := s.Tasks.ClaimForExecution(ctx, taskID, wallet)
			if errors.Is(claimErr, repository.ErrExecutionClaimLost) {
				return s.existingTradeOrConflict(ctx, taskID, wallet)
			}
			if claimErr != nil {
				return ArmTaskResult{}, claimErr
			}
			return s.armAndExecute(ctx, claimed, decide, input)
		}
	}

	if task.IsActionable {
		if err := s.grantAndPersistArm(ctx, taskID, onChainTaskID, input); err != nil {
			return ArmTaskResult{}, err
		}
		if err := s.recordArmed(ctx, task, taskID, onChainTaskID, input.TotalBudget); err != nil {
			slog.ErrorContext(ctx, "agent: record armed sub task failed", "task_id", taskID, "error", err)
		}
	}

	if err := s.recordPendingSubTasks(ctx, taskID, onChainTaskID); err != nil {
		slog.ErrorContext(ctx, "agent: catch-up recordSubTasks failed, leaving for retry", "task_id", taskID, "error", err)
	}

	return ArmTaskResult{OnChainTaskID: int64(onChainTaskID), TokenAddress: input.TokenAddress, TotalBudget: input.TotalBudget}, nil
}

type executionDecide struct {
	SubTaskID     int64
	OnChainID     uint64
	ReasoningHash string
	Side          string
	Ticker        string
	StockID       int64
	Label         string
}

func executionDecideOf(task model.AgentTask) (executionDecide, bool) {
	if task.TriggerDescription == nil {
		return executionDecide{}, false
	}
	var payload struct {
		ExecutionDecide *struct {
			SubTaskID     int64  `json:"sub_task_id"`
			OnChainID     uint64 `json:"on_chain_id"`
			ReasoningHash string `json:"reasoning_hash"`
			StockID       int64  `json:"stock_id"`
		} `json:"execution_decide"`
		Side           string `json:"side"`
		ResolvedTicker string `json:"resolved_ticker"`
		StockTicker    string `json:"stock_ticker"`
	}
	if err := json.Unmarshal([]byte(*task.TriggerDescription), &payload); err != nil {
		return executionDecide{}, false
	}
	if payload.ExecutionDecide == nil || payload.ExecutionDecide.OnChainID == 0 {
		return executionDecide{}, false
	}
	ticker := payload.ResolvedTicker
	if ticker == "" {
		ticker = payload.StockTicker
	}
	label := ""
	if task.Summary != nil {
		label = *task.Summary
	}
	return executionDecide{
		SubTaskID:     payload.ExecutionDecide.SubTaskID,
		OnChainID:     payload.ExecutionDecide.OnChainID,
		ReasoningHash: payload.ExecutionDecide.ReasoningHash,
		Side:          payload.Side,
		Ticker:        ticker,
		StockID:       payload.ExecutionDecide.StockID,
		Label:         label,
	}, true
}

func (s *TaskService) armAndExecute(ctx context.Context, task model.AgentTask, decide executionDecide, input ArmTaskInput) (ArmTaskResult, error) {
	onChainTaskID := uint64(*task.OnChainTaskID)
	amount, ok := new(big.Int).SetString(input.TotalBudget, 10)
	if !ok || amount.Sign() <= 0 {
		_ = s.Tasks.SetStatus(ctx, task.ID, "pending")
		return ArmTaskResult{}, fmt.Errorf("agent: invalid total_budget %q", input.TotalBudget)
	}
	side := contracts.TradeSide(decide.Side)
	if side != contracts.TradeSideBuy && side != contracts.TradeSideSell {
		_ = s.Tasks.SetStatus(ctx, task.ID, "pending")
		return ArmTaskResult{}, fmt.Errorf("agent: invalid side %q", decide.Side)
	}
	stockID := decide.StockID
	var stockContract string
	if stockID == 0 || (side == contracts.TradeSideSell && input.TokenAddress == "") {
		stock, found, err := s.Stocks.FindByTicker(ctx, decide.Ticker)
		if err != nil || !found {
			_ = s.Tasks.SetStatus(ctx, task.ID, "pending")
			return ArmTaskResult{}, fmt.Errorf("agent: ticker %q not found", decide.Ticker)
		}
		if stockID == 0 {
			stockID = stock.ID
		}
		if stock.ContractAddress != nil {
			stockContract = *stock.ContractAddress
		}
	}
	if stockID == 0 {
		_ = s.Tasks.SetStatus(ctx, task.ID, "pending")
		return ArmTaskResult{}, fmt.Errorf("agent: stock id is missing for %q", decide.Ticker)
	}
	token := common.HexToAddress(input.TokenAddress)
	if token == (common.Address{}) && stockContract != "" {
		token = common.HexToAddress(stockContract)
	}
	if token == (common.Address{}) {
		_ = s.Tasks.SetStatus(ctx, task.ID, "pending")
		return ArmTaskResult{}, fmt.Errorf("agent: token address is empty")
	}
	minOutStarted := time.Now()
	minOut, err := s.minOutFor(ctx, side, decide.Ticker, amount)
	if err != nil {
		_ = s.Tasks.SetStatus(ctx, task.ID, "pending")
		return ArmTaskResult{}, err
	}
	slog.InfoContext(ctx, "agent: instant minOut", "task_id", task.ID, "duration_ms", time.Since(minOutStarted).Milliseconds())

	submitStarted := time.Now()
	_, executeHash, _, err := s.Chain.SubmitTradePair(ctx,
		onChainTaskID, decide.OnChainID, token, decide.Ticker, side,
		amount, minOut, decide.Label, common.HexToHash(decide.ReasoningHash),
		input.TotalBudget, input.DurationSec, input.MaxAmountPerTrade, input.CooldownInterval,
		s.ExecutionGasLimit,
	)
	if err != nil {
		_ = s.Tasks.SetStatus(ctx, task.ID, "pending")
		return ArmTaskResult{}, fmt.Errorf("agent: submit trade pair: %w", err)
	}
	slog.InfoContext(ctx, "agent: instant submitPair", "task_id", task.ID, "duration_ms", time.Since(submitStarted).Milliseconds())

	waitStarted := time.Now()
	trade, err := s.Chain.WaitTradeByHash(ctx, executeHash)
	if err != nil {
		_ = s.Tasks.SetStatus(ctx, task.ID, "pending")
		return ArmTaskResult{}, fmt.Errorf("agent: wait trade receipt: %w", err)
	}
	slog.InfoContext(ctx, "agent: instant waitTrade", "task_id", task.ID, "duration_ms", time.Since(waitStarted).Milliseconds())

	dbStarted := time.Now()

	amountRaw := amount.String()
	if trade.Amount != nil {
		amountRaw = trade.Amount.String()
	}
	receivedRaw := ""
	if trade.ReceivedAmount != nil {
		receivedRaw = trade.ReceivedAmount.String()
	}
	idrxAmount := amountRaw
	stockAmount := receivedRaw
	if side == contracts.TradeSideSell {
		idrxAmount = receivedRaw
		stockAmount = amountRaw
	}
	fee := "0"
	if trade.ProtocolFee != nil {
		fee = trade.ProtocolFee.String()
	}
	agentTrade, err := s.Trades.RecordInstantExecution(ctx, repository.InstantExecutionRecord{
		TaskID:          task.ID,
		SubTaskID:       decide.SubTaskID,
		OnChainTradeID:  int64(trade.TradeID),
		TxHash:          executeHash,
		Ticker:          decide.Ticker,
		Side:            string(side),
		Amount:          amountRaw,
		Summary:         decide.Label,
		StockID:         stockID,
		WalletAddress:   strings.ToLower(strings.TrimSpace(task.WalletAddress)),
		IdrxAmount:      idrxAmount,
		StockAmount:     stockAmount,
		ProtocolFeeIdrx: fee,
		BlockNumber:     trade.BlockNumber,
		LogIndex:        trade.LogIndex,
	})
	if err != nil {
		return ArmTaskResult{}, fmt.Errorf("agent: record instant execution: %w", err)
	}
	slog.InfoContext(ctx, "agent: instant dbWrites", "task_id", task.ID, "duration_ms", time.Since(dbStarted).Milliseconds())

	go func() {
		bg, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Minute)
		defer cancel()
		// The audit trail is not part of the trade. Delay it so its nonce
		// never sits in front of the next scalp trade on the same wallet.
		select {
		case <-time.After(30 * time.Second):
		case <-bg.Done():
			return
		}
		if err := s.recordArmed(bg, task, task.ID, onChainTaskID, input.TotalBudget); err != nil {
			slog.WarnContext(bg, "agent: record armed after execution failed", "task_id", task.ID, "error", err)
		}
		recorder, err := subtask.NewSubTaskRecorder(bg, s.SubTasks, s.Chain, onChainTaskID, task.ID, summaryOf(task), task.WalletAddress)
		if err != nil {
			slog.WarnContext(bg, "agent: audit recorder after execution failed", "task_id", task.ID, "error", err)
		} else {
			_, _ = recorder.Record(bg, "executor", "execute", "done", decide.Label, decide.Label, map[string]any{
				"tx_hash":  executeHash,
				"trade_id": trade.TradeID,
			})
			_, _ = recorder.Record(bg, "executor", "sync_database", "done", decide.Label, decide.Label, nil)
		}
		s.persistExecutionMessage(bg, &task, cardExecutedText(task))
	}()

	return ArmTaskResult{
		OnChainTaskID: int64(onChainTaskID),
		TokenAddress:  input.TokenAddress,
		TotalBudget:   input.TotalBudget,
		Status:        "executed",
		TxHash:        executeHash,
		Trade:         &agentTrade,
	}, nil
}

func (s *TaskService) minOutFor(ctx context.Context, side contracts.TradeSide, ticker string, amount *big.Int) (*big.Int, error) {
	if s.Prices == nil {
		return nil, fmt.Errorf("agent: price reader is not configured")
	}
	price, err := s.Prices.OnchainSpotPrice(ctx, ticker)
	if err != nil {
		return nil, fmt.Errorf("agent: spot price %s: %w", ticker, err)
	}
	if price <= 0 {
		return nil, fmt.Errorf("agent: non-positive spot price for %s", ticker)
	}

	oneE18 := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)
	var minOut *big.Int
	if side == contracts.TradeSideSell {
		wholeStock := new(big.Float).Quo(new(big.Float).SetInt(amount), new(big.Float).SetInt(oneE18))
		idrxRaw := new(big.Float).Mul(wholeStock, big.NewFloat(price*100))
		minOut, _ = new(big.Float).Mul(idrxRaw, big.NewFloat(0.99)).Int(nil)
	} else {
		idrxDisplay := new(big.Float).Quo(new(big.Float).SetInt(amount), big.NewFloat(100))
		wholeStock := new(big.Float).Quo(idrxDisplay, big.NewFloat(price))
		stockRaw := new(big.Float).Mul(wholeStock, new(big.Float).SetInt(oneE18))
		minOut, _ = new(big.Float).Mul(stockRaw, big.NewFloat(0.99)).Int(nil)
	}
	if minOut == nil || minOut.Sign() <= 0 {
		return nil, fmt.Errorf("agent: computed zero minimum output for %s", ticker)
	}
	return minOut, nil
}

func (s *TaskService) existingTradeOrConflict(ctx context.Context, taskID int64, wallet string) (ArmTaskResult, error) {
	task, found, err := s.Tasks.FindByID(ctx, taskID)
	if err != nil {
		return ArmTaskResult{}, err
	}
	if !found {
		return ArmTaskResult{}, agentcore.ErrTaskNotFound
	}
	if !strings.EqualFold(task.WalletAddress, wallet) {
		return ArmTaskResult{}, agentcore.ErrWalletMismatch
	}
	trades, err := s.Trades.FindByTaskID(ctx, taskID)
	if err == nil && len(trades) > 0 {
		last := trades[len(trades)-1]
		result := ArmTaskResult{Status: "executed", Trade: &last}
		if task.OnChainTaskID != nil {
			result.OnChainTaskID = *task.OnChainTaskID
		}
		if last.TxHash != nil {
			result.TxHash = *last.TxHash
		}
		return result, nil
	}
	return ArmTaskResult{}, agentcore.ErrExecutionInFlight
}

func summaryOf(task model.AgentTask) string {
	if task.Summary != nil {
		return *task.Summary
	}
	return ""
}

func cardExecutedText(task model.AgentTask) string {
	if task.TriggerDescription == nil {
		return ""
	}
	var payload struct {
		Card *contracts.CardContract `json:"card"`
	}
	if err := json.Unmarshal([]byte(*task.TriggerDescription), &payload); err != nil || payload.Card == nil {
		return ""
	}
	if payload.Card.Ledger.ExecutedDesc != "" {
		return payload.Card.Ledger.ExecutedDesc
	}
	return payload.Card.Footnotes.ExecutedSuccess
}

func (s *TaskService) grantAndPersistArm(ctx context.Context, taskID int64, onChainTaskID uint64, input ArmTaskInput) error {
	duration := time.Duration(input.DurationSec) * time.Second
	if err := s.Chain.GrantTradePermission(ctx, onChainTaskID, input.TotalBudget, duration, input.MaxAmountPerTrade, input.CooldownInterval); err != nil {
		return fmt.Errorf("agent: on-chain grantTradePermission: %w", err)
	}

	// Only here, only after the chain call actually succeeded — "armed"
	// means exactly "the on-chain permission exists".
	now := time.Now()
	var nextRunAt *time.Time
	if input.IsRecurring && input.CooldownInterval > 0 {
		t := now.Add(time.Duration(input.CooldownInterval) * time.Second)
		nextRunAt = &t
	}
	var horizonExpiresAt *time.Time
	if input.DurationSec > 0 {
		t := now.Add(duration)
		horizonExpiresAt = &t
	}
	var maxPerTrade int64
	if input.MaxAmountPerTrade != "" {
		if m, ok := new(big.Int).SetString(input.MaxAmountPerTrade, 10); ok {
			maxPerTrade = m.Int64()
		}
	}

	if err := s.Tasks.SetArmedWithGuardrails(ctx, taskID, now, input.IsRecurring, int32(input.CooldownInterval), maxPerTrade, nextRunAt, horizonExpiresAt); err != nil {
		return fmt.Errorf("agent: permission granted on-chain for task %d but persisting armed guardrails failed: %w", taskID, err)
	}
	return nil
}

// recordArmed writes the "armed" Sub Task — the last link in the
// understand -> route -> analyze -> await_confirmation -> armed -> execute
// chain — once GrantTradePermission has actually succeeded on-chain.
func (s *TaskService) recordArmed(ctx context.Context, task model.AgentTask, taskID int64, onChainTaskID uint64, totalBudget string) error {
	summary := ""
	if task.Summary != nil {
		summary = *task.Summary
	}
	recorder, err := subtask.NewSubTaskRecorder(ctx, s.SubTasks, s.Chain, onChainTaskID, taskID, summary, task.WalletAddress)
	if err != nil {
		return fmt.Errorf("agent: build subtask recorder for armed step: %w", err)
	}
	_, err = recorder.Record(ctx, "supervisor", "armed", "done", summary, summary, nil)
	return err
}

func (s *TaskService) recordPendingSubTasks(ctx context.Context, taskID int64, onChainTaskID uint64) error {
	allRows, err := s.SubTasks.FindByTaskID(ctx, taskID)
	if err != nil {
		return err
	}
	pending := make([]model.AgentSubTask, 0, len(allRows))
	for _, row := range allRows {
		if !row.RecordedOnChain {
			pending = append(pending, row)
		}
	}
	return s.recordSubTasksBatch(ctx, int64(onChainTaskID), pending)
}

func (s *TaskService) DisarmTask(ctx context.Context, taskID int64, wallet string) error {
	task, found, err := s.Tasks.FindByID(ctx, taskID)
	if err != nil {
		return err
	}
	if !found {
		return agentcore.ErrTaskNotFound
	}
	if !strings.EqualFold(task.WalletAddress, wallet) {
		return agentcore.ErrWalletMismatch
	}
	if task.OnChainTaskID == nil {
		return agentcore.ErrTaskNotArmed
	}
	if err := s.Chain.CancelTask(ctx, uint64(*task.OnChainTaskID)); err != nil {
		return fmt.Errorf("agent: on-chain cancelTask: %w", err)
	}
	return s.Tasks.SetCancelled(ctx, taskID)
}

func (s *TaskService) PauseTask(ctx context.Context, taskID int64, wallet string) error {
	task, found, err := s.Tasks.FindByID(ctx, taskID)
	if err != nil {
		return err
	}
	if !found {
		return agentcore.ErrTaskNotFound
	}
	if !strings.EqualFold(task.WalletAddress, wallet) {
		return agentcore.ErrWalletMismatch
	}
	return s.Tasks.SetPaused(ctx, taskID, true)
}

func (s *TaskService) ResumeTask(ctx context.Context, taskID int64, wallet string) error {
	task, found, err := s.Tasks.FindByID(ctx, taskID)
	if err != nil {
		return err
	}
	if !found {
		return agentcore.ErrTaskNotFound
	}
	if !strings.EqualFold(task.WalletAddress, wallet) {
		return agentcore.ErrWalletMismatch
	}
	return s.Tasks.SetPaused(ctx, taskID, false)
}

// ExecuteTask authorizes and triggers execution — nothing else. All LLM
// orchestration (reading Nova's verdict, deciding whether Comet fires,
// sizing, submit_trade) happens inside the graph via
// Orchestrator.ResumeExecute; this function never builds a prompt or calls
// an agent itself (docs/plans/quasar-clean-routing-scalp-refactor.md §2).
func (s *TaskService) ExecuteTask(ctx context.Context, taskID int64, wallet string) (agentcore.ExecuteTaskResult, error) {
	task, found, err := s.Tasks.FindByID(ctx, taskID)
	if err != nil {
		return agentcore.ExecuteTaskResult{}, err
	}
	if !found {
		return agentcore.ExecuteTaskResult{}, agentcore.ErrTaskNotFound
	}
	if !strings.EqualFold(task.WalletAddress, wallet) {
		return agentcore.ExecuteTaskResult{}, agentcore.ErrWalletMismatch
	}
	if !task.IsActionable {
		return agentcore.ExecuteTaskResult{}, agentcore.ErrTaskNotActionable
	}
	if task.ArmedAt == nil || task.OnChainTaskID == nil {
		return agentcore.ExecuteTaskResult{}, agentcore.ErrTaskNotArmed
	}

	if task.Status == "executed" {
		return s.alreadyExecutedResult(ctx, task)
	}
	if s.Orchestrator == nil {
		return agentcore.ExecuteTaskResult{}, errors.New("agent: orchestrator not initialized")
	}

	tradesBefore, _ := s.Trades.FindByTaskID(ctx, taskID)
	countBefore := len(tradesBefore)

	slog.InfoContext(ctx, "agent: resuming task execution via orchestrator graph", "task_id", taskID, "on_chain_id", *task.OnChainTaskID)

	res, err := s.Orchestrator.ResumeExecute(ctx, taskID, wallet)
	if err != nil {
		s.persistExecutionMessage(ctx, &task, fmt.Sprintf("On-chain execution failed: %s", err.Error()))
		return agentcore.ExecuteTaskResult{}, fmt.Errorf("agent: orchestrator resume execute: %w", err)
	}

	status, executedTrades := s.reconcileTradeStatus(ctx, taskID, task.Status, countBefore)

	cleanReply := cleanExecutionReply(res.Reply)
	if strings.TrimSpace(cleanReply) != "" {
		s.persistExecutionMessage(ctx, &task, cleanReply)
	}

	return agentcore.ExecuteTaskResult{TaskID: taskID, Status: status, Reply: cleanReply, Trades: executedTrades}, nil
}

func (s *TaskService) alreadyExecutedResult(ctx context.Context, task model.AgentTask) (agentcore.ExecuteTaskResult, error) {
	var card *contracts.CardContract
	if task.TriggerDescription != nil && *task.TriggerDescription != "" {
		var triggerData struct {
			Card *contracts.CardContract `json:"card"`
		}
		if err := json.Unmarshal([]byte(*task.TriggerDescription), &triggerData); err == nil {
			card = triggerData.Card
		}
	}

	trades, _ := s.Trades.FindByTaskID(ctx, task.ID)
	reply := "This task was already executed."
	if card != nil && card.Ledger.ExecutedDesc != "" {
		reply = card.Ledger.ExecutedDesc
	}
	return agentcore.ExecuteTaskResult{TaskID: task.ID, Status: task.Status, Reply: reply, Trades: trades}, nil
}

func (s *TaskService) reconcileTradeStatus(ctx context.Context, taskID int64, currentStatus string, countBefore int) (string, []model.AgentTrade) {
	tradesAfter, err := s.Trades.FindByTaskID(ctx, taskID)
	if err != nil {
		return currentStatus, nil
	}
	if len(tradesAfter) > countBefore {
		_ = s.Tasks.SetStatus(ctx, taskID, "executed")
		_ = s.Tasks.SetExecutedAt(ctx, taskID, time.Now())
		return "executed", tradesAfter[countBefore:]
	}
	if len(tradesAfter) > 0 {
		return "executed", tradesAfter
	}
	return currentStatus, nil
}

func (s *TaskService) persistExecutionMessage(ctx context.Context, task *model.AgentTask, content string) {
	if s.ChatMessages == nil || strings.TrimSpace(content) == "" {
		return
	}
	chatID := s.resolveExecutionChatID(ctx, task)
	if chatID == nil {
		return
	}
	if _, err := s.ChatMessages.Create(ctx, repository.AgentChatMessageCreateInput{
		ChatID:      *chatID,
		Sender:      "supervisor",
		ContentType: "text",
		Content:     content,
		UIRefTaskID: nil, // Do not duplicate PlanCard in chat thread
	}); err != nil {
		slog.ErrorContext(ctx, "agent: persist execution reply to chat", "error", err)
	}
}

func (s *TaskService) resolveExecutionChatID(ctx context.Context, task *model.AgentTask) *uuid.UUID {
	if task.SourceMessageID != nil {
		if sourceMsg, found, err := s.ChatMessages.FindByID(ctx, *task.SourceMessageID); err == nil && found {
			cid := sourceMsg.ChatID
			return &cid
		}
	}
	if s.Chats != nil {
		if chats, err := s.Chats.FindByOwnerWallet(ctx, task.WalletAddress); err == nil && len(chats) > 0 {
			cid := chats[0].ID
			return &cid
		}
	}
	return nil
}

// Evaluate (the scheduler-driven re-check tick for an armed, actionable
// Task) is intentionally not implemented — standing-instruction monitoring
// is still undesigned work, tracked separately, not part of this refactor's
// scalp-only scope.

func cleanExecutionReply(raw string) string {
	trimmed := strings.TrimSpace(raw)
	trimmed = stripCodeFence(trimmed)

	if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
		if reply, ok := extractReplyField(trimmed); ok {
			return reply
		}
	}
	return trimmed
}

func stripCodeFence(trimmed string) string {
	if !strings.HasPrefix(trimmed, "```") {
		return trimmed
	}
	lines := strings.Split(trimmed, "\n")
	if len(lines) < 2 {
		return trimmed
	}
	if strings.HasPrefix(strings.TrimSpace(lines[len(lines)-1]), "```") {
		lines = lines[1 : len(lines)-1]
	} else {
		lines = lines[1:]
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func extractReplyField(trimmed string) (string, bool) {
	var parsed struct {
		Reasoning   string `json:"reasoning"`
		Reply       string `json:"reply"`
		Message     string `json:"message"`
		Explanation string `json:"explanation"`
		Status      string `json:"status"`
	}
	if err := json.Unmarshal([]byte(trimmed), &parsed); err != nil {
		return "", false
	}
	switch {
	case parsed.Reasoning != "":
		return parsed.Reasoning, true
	case parsed.Reply != "":
		return parsed.Reply, true
	case parsed.Message != "":
		return parsed.Message, true
	case parsed.Explanation != "":
		return parsed.Explanation, true
	case parsed.Status != "":
		return fmt.Sprintf("Status eksekusi: %s", parsed.Status), true
	default:
		return "", false
	}
}

const (
	recordSubTasksGracePeriod = 2 * time.Minute
	subTaskRetryPollInterval  = 1 * time.Minute
)

func (s *TaskService) RunSubTaskRetry(ctx context.Context) {
	s.runSubTaskRetryOnce(ctx)

	ticker := time.NewTicker(subTaskRetryPollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.runSubTaskRetryOnce(ctx)
		}
	}
}

func (s *TaskService) runSubTaskRetryOnce(ctx context.Context) {
	cutoff := time.Now().Add(-recordSubTasksGracePeriod)
	unconfirmed, err := s.SubTasks.FindUnconfirmed(ctx, cutoff)
	if err != nil {
		slog.ErrorContext(ctx, "subtask_retry: failed to load unconfirmed rows", "error", err)
		return
	}
	if len(unconfirmed) == 0 {
		return
	}

	rowsByTask := map[int64][]model.AgentSubTask{}
	for _, row := range unconfirmed {
		rowsByTask[row.TaskID] = append(rowsByTask[row.TaskID], row)
	}

	for taskID, rows := range rowsByTask {
		task, found, err := s.Tasks.FindByID(ctx, taskID)
		if err != nil || !found || task.OnChainTaskID == nil {
			slog.ErrorContext(ctx, "subtask_retry: task missing or not armed, skipping", "task_id", taskID, "error", err)
			continue
		}

		_, txHash, err := s.Chain.RecordSubTasks(ctx, uint64(*task.OnChainTaskID), rows)
		if err != nil {
			slog.ErrorContext(ctx, "subtask_retry: resubmit failed, will retry next tick", "task_id", taskID, "error", err)
			continue
		}

		ids := make([]int64, len(rows))
		for i, row := range rows {
			ids[i] = row.ID
		}
		if err := s.SubTasks.MarkRecordedOnChain(ctx, ids, txHash); err != nil {
			slog.ErrorContext(ctx, "subtask_retry: confirmed on-chain but failed to mark locally", "task_id", taskID, "tx_hash", txHash, "error", err)
		}
	}
}

func (s *TaskService) SettleHorizonTask(ctx context.Context, taskID int64, wallet string, policy string) error {
	task, found, err := s.Tasks.FindByID(ctx, taskID)
	if err != nil {
		return err
	}
	if !found {
		return agentcore.ErrTaskNotFound
	}
	if !strings.EqualFold(task.WalletAddress, wallet) {
		return agentcore.ErrWalletMismatch
	}
	status := "completed"
	if policy == "leave_open" {
		status = "settled_held"
	}
	return s.Tasks.SetExitPolicy(ctx, taskID, policy, status)
}
