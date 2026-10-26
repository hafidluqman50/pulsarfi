package agent_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/crypto"
	agentcore "github.com/horizonlabs/pulsarfi-backend/src/app/core/agent"
	"github.com/horizonlabs/pulsarfi-backend/src/config"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/subtask"
	agenttask "github.com/horizonlabs/pulsarfi-backend/src/service/agent/task"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
	"github.com/joho/godotenv"
)

type liveSpotPrices struct {
	price    *external.PriceService
	protocol string
	rpcURL   string
}

func (a liveSpotPrices) OnchainSpotPrice(ctx context.Context, ticker string) (float64, error) {
	entry, err := a.price.GetOnchainPriceV4(a.protocol, ticker, a.rpcURL)
	if err != nil {
		return 0, err
	}
	return entry.Price, nil
}

func (a liveSpotPrices) QuoteStockToIdrx(ctx context.Context, ticker string, stockAmountRaw *big.Int) (*big.Int, error) {
	return a.price.QuoteStockToIdrxRaw(a.protocol, ticker, stockAmountRaw, a.rpcURL)
}

func TestInstantExecution_LiveArm(t *testing.T) {
	if os.Getenv("INSTANT_EXECUTION_LIVE") != "1" {
		t.Skip("set INSTANT_EXECUTION_LIVE=1 to run the real chain/DB test")
	}
	_ = godotenv.Load("../../../.env")
	signerEnv, err := godotenv.Read("../../../../smart-contract/.env")
	if err != nil {
		t.Fatalf("read smart-contract/.env: %v", err)
	}
	wallet := signerEnv["DEMO_INVESTOR_RETAIL_WALLET"]
	if wallet == "" {
		t.Fatal("DEMO_INVESTOR_RETAIL_WALLET is missing")
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Fatal("DATABASE_URL is required")
	}
	db, err := config.NewDatabase(dsn)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	repos := repository.NewRegistry(db)
	cfg, err := config.LoadOnchain()
	if err != nil {
		t.Fatalf("load onchain config: %v", err)
	}
	ctx := context.Background()
	chain, err := external.NewOnchainService(ctx, cfg)
	if err != nil {
		t.Fatalf("build onchain service: %v", err)
	}
	prices := liveSpotPrices{price: external.NewPriceService(), protocol: cfg.PulsarProtocol, rpcURL: cfg.RPCURL}
	taskSvc := &agenttask.TaskService{
		Tasks:             repos.AgentTask,
		SubTasks:          repos.AgentSubTask,
		Trades:            repos.AgentTrade,
		Chain:             chain,
		Stocks:            repos.Stock,
		Prices:            prices,
		StockTransactions: repos.StockTransaction,
		ExecutionGasLimit: cfg.ExecutionGasLimit,
		InstantExecution:  true,
	}

	const runs = 3
	durations := make([]time.Duration, 0, runs)
	for run := 0; run < runs; run++ {
		started := time.Now()
		summary := fmt.Sprintf("instant execution live test %d", run)
		dbTask, err := repos.AgentTask.Create(ctx, repository.AgentTaskCreateInput{
			WalletAddress:      wallet,
			IsActionable:       true,
			Summary:            summary,
			TriggerDescription: stringPtr(fmt.Sprintf(`{"side":"buy","resolved_ticker":"BRPTP","confirmed_budget_idrx":"1000","card":{"ledger":{"executed_desc":"Live test execution"}},"run":%d}`, run)),
		})
		if err != nil {
			t.Fatalf("run %d: create db task: %v", run, err)
		}
		onChainID, err := chain.CreateTask(ctx, wallet, true, summary, crypto.Keccak256Hash([]byte(fmt.Sprintf("instant-%d-%d", run, started.UnixNano()))))
		if err != nil {
			t.Fatalf("run %d: create on-chain task: %v", run, err)
		}
		if err := repos.AgentTask.SetOnChainTaskID(ctx, dbTask.ID, int64(onChainID)); err != nil {
			t.Fatalf("run %d: set on-chain id: %v", run, err)
		}
		recorder, err := subtask.NewSubTaskRecorder(ctx, repos.AgentSubTask, chain, onChainID, dbTask.ID, summary, wallet)
		if err != nil {
			t.Fatalf("run %d: recorder: %v", run, err)
		}
		anchor, err := recorder.Record(ctx, "supervisor", "await_confirmation", "done", "Live test anchor", "Live test anchor", nil)
		if err != nil {
			t.Fatalf("run %d: record anchor: %v", run, err)
		}
		if anchor.OnChainSubTaskID == nil {
			t.Fatalf("run %d: anchor has no on-chain id", run)
		}
		stock, found, err := repos.Stock.FindByTicker(ctx, "BRPTP")
		if err != nil || !found {
			t.Fatalf("run %d: BRPTP stock not found: found=%v err=%v", run, found, err)
		}
		if err := repos.AgentTask.SetExecutionDecide(ctx, dbTask.ID, anchor.ID, uint64(*anchor.OnChainSubTaskID), anchor.DecisionHash, stock.ID); err != nil {
			t.Fatalf("run %d: set execution decide: %v", run, err)
		}

		armStarted := time.Now()
		result, err := taskSvc.ArmTask(ctx, dbTask.ID, wallet, agenttask.ArmTaskInput{
			TotalBudget:  "100000",
			DurationSec:  86400,
			TokenAddress: cfg.IDRXAddress,
		})
		elapsed := time.Since(armStarted)
		durations = append(durations, elapsed)
		if err != nil {
			t.Fatalf("run %d: ArmTask: %v", run, err)
		}
		if result.Status != "executed" || result.Trade == nil || result.TxHash == "" {
			t.Fatalf("run %d: unexpected result: %+v", run, result)
		}
		stored, found, err := repos.AgentTask.FindByID(ctx, dbTask.ID)
		if err != nil || !found {
			t.Fatalf("run %d: reload task: found=%v err=%v", run, found, err)
		}
		if stored.Status != "executed" || stored.ArmedAt == nil || stored.ExecutedAt == nil {
			t.Fatalf("run %d: task state wrong: status=%s armed=%v executed=%v", run, stored.Status, stored.ArmedAt, stored.ExecutedAt)
		}
		trades, err := repos.AgentTrade.FindByTaskID(ctx, dbTask.ID)
		if err != nil || len(trades) != 1 {
			t.Fatalf("run %d: agent_trades = %d, err=%v", run, len(trades), err)
		}
		stockTx, found, err := repos.StockTransaction.FindByTxHash(ctx, result.TxHash)
		if err != nil || !found {
			t.Fatalf("run %d: stock_transactions row missing: found=%v err=%v", run, found, err)
		}
		var payload map[string]any
		if stored.TriggerDescription != nil {
			_ = json.Unmarshal([]byte(*stored.TriggerDescription), &payload)
		}
		if payload["execution_decide"] == nil {
			t.Fatalf("run %d: execution_decide missing after arm", run)
		}
		t.Logf("run %d: server duration=%s tx=%s trade_id=%d stock_tx=%d status=%s", run, elapsed.Round(time.Millisecond), result.TxHash, result.Trade.ID, stockTx.ID, stored.Status)
	}

	t.Logf("durations: %v", durations)
}

func stringPtr(value string) *string { return &value }

func TestInstantExecution_LiveDoubleClick(t *testing.T) {
	if os.Getenv("INSTANT_EXECUTION_LIVE") != "1" {
		t.Skip("set INSTANT_EXECUTION_LIVE=1 to run the real chain/DB test")
	}
	_ = godotenv.Load("../../../.env")
	signerEnv, err := godotenv.Read("../../../../smart-contract/.env")
	if err != nil {
		t.Fatalf("read smart-contract/.env: %v", err)
	}
	wallet := signerEnv["DEMO_INVESTOR_RETAIL_WALLET"]
	if wallet == "" {
		t.Fatal("DEMO_INVESTOR_RETAIL_WALLET is missing")
	}
	db, err := config.NewDatabase(os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	repos := repository.NewRegistry(db)
	cfg, err := config.LoadOnchain()
	if err != nil {
		t.Fatalf("load onchain config: %v", err)
	}
	ctx := context.Background()
	chain, err := external.NewOnchainService(ctx, cfg)
	if err != nil {
		t.Fatalf("build onchain service: %v", err)
	}
	prices := liveSpotPrices{price: external.NewPriceService(), protocol: cfg.PulsarProtocol, rpcURL: cfg.RPCURL}
	taskSvc := &agenttask.TaskService{
		Tasks:             repos.AgentTask,
		SubTasks:          repos.AgentSubTask,
		Trades:            repos.AgentTrade,
		Chain:             chain,
		Stocks:            repos.Stock,
		Prices:            prices,
		StockTransactions: repos.StockTransaction,
		ExecutionGasLimit: cfg.ExecutionGasLimit,
		InstantExecution:  true,
	}

	summary := "instant execution double click"
	dbTask, err := repos.AgentTask.Create(ctx, repository.AgentTaskCreateInput{
		WalletAddress:      wallet,
		IsActionable:       true,
		Summary:            summary,
		TriggerDescription: stringPtr(`{"side":"buy","resolved_ticker":"BRPTP","confirmed_budget_idrx":"1000","card":{"ledger":{"executed_desc":"Double click test"}}}`),
	})
	if err != nil {
		t.Fatalf("create db task: %v", err)
	}
	onChainID, err := chain.CreateTask(ctx, wallet, true, summary, crypto.Keccak256Hash([]byte(fmt.Sprintf("double-%d", time.Now().UnixNano()))))
	if err != nil {
		t.Fatalf("create on-chain task: %v", err)
	}
	if err := repos.AgentTask.SetOnChainTaskID(ctx, dbTask.ID, int64(onChainID)); err != nil {
		t.Fatalf("set on-chain id: %v", err)
	}
	recorder, err := subtask.NewSubTaskRecorder(ctx, repos.AgentSubTask, chain, onChainID, dbTask.ID, summary, wallet)
	if err != nil {
		t.Fatalf("recorder: %v", err)
	}
	anchor, err := recorder.Record(ctx, "supervisor", "await_confirmation", "done", "Double click anchor", "Double click anchor", nil)
	if err != nil {
		t.Fatalf("record anchor: %v", err)
	}
	if anchor.OnChainSubTaskID == nil {
		t.Fatal("anchor has no on-chain id")
	}
	stock, found, err := repos.Stock.FindByTicker(ctx, "BRPTP")
	if err != nil || !found {
		t.Fatalf("BRPTP not found: found=%v err=%v", found, err)
	}
	if err := repos.AgentTask.SetExecutionDecide(ctx, dbTask.ID, anchor.ID, uint64(*anchor.OnChainSubTaskID), anchor.DecisionHash, stock.ID); err != nil {
		t.Fatalf("set execution decide: %v", err)
	}

	input := agenttask.ArmTaskInput{TotalBudget: "100000", DurationSec: 86400, TokenAddress: cfg.IDRXAddress}
	type outcome struct {
		result agenttask.ArmTaskResult
		err    error
	}
	results := make(chan outcome, 2)
	for i := 0; i < 2; i++ {
		go func() {
			result, err := taskSvc.ArmTask(ctx, dbTask.ID, wallet, input)
			results <- outcome{result: result, err: err}
		}()
	}
	okCount, conflictCount := 0, 0
	for i := 0; i < 2; i++ {
		got := <-results
		switch {
		case got.err == nil:
			okCount++
			if got.result.Status != "executed" || got.result.Trade == nil {
				t.Fatalf("successful arm returned %+v", got.result)
			}
		case errors.Is(got.err, agentcore.ErrExecutionInFlight):
			conflictCount++
		default:
			t.Fatalf("unexpected second arm error: %v", got.err)
		}
	}
	t.Logf("double click: ok=%d conflict=%d", okCount, conflictCount)
	if okCount < 1 {
		t.Fatal("no arm succeeded")
	}

	trades, err := repos.AgentTrade.FindByTaskID(ctx, dbTask.ID)
	if err != nil {
		t.Fatalf("load trades: %v", err)
	}
	if len(trades) != 1 {
		t.Fatalf("double click produced %d trades, want exactly 1", len(trades))
	}
	stored, found, err := repos.AgentTask.FindByID(ctx, dbTask.ID)
	if err != nil || !found {
		t.Fatalf("reload task: found=%v err=%v", found, err)
	}
	if stored.Status != "executed" || stored.ExecutedAt == nil {
		t.Fatalf("task state wrong: status=%s executed=%v", stored.Status, stored.ExecutedAt)
	}
	t.Logf("double click trade id=%d tx=%s status=%s", trades[0].ID, deref(trades[0].TxHash), stored.Status)
}

func deref(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}
