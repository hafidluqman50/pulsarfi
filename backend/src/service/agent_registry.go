package service

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"os"

	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	agentsvc "github.com/horizonlabs/pulsarfi-backend/src/service/agent"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/analyzer"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/executor"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
	publicsvc "github.com/horizonlabs/pulsarfi-backend/src/service/public"
)

// spotPriceAdapter closes over the env vars PriceService.GetOnchainPriceV4
// itself needs (PULSAR_PROTOCOL/ALCHEMY_RPC_URL, same pattern already used
// by public/stock_service.go's own market-listing call site) — keeps
// executor.SpotPriceReader's own interface free of env-var/wiring
// knowledge (ISP), matching every other adapter in this file.
type spotPriceAdapter struct {
	price        *publicsvc.PriceService
	protocolAddr string
	rpcURL       string
}

func (a spotPriceAdapter) OnchainSpotPrice(ctx context.Context, ticker string) (float64, error) {
	entry, err := a.price.Price.GetOnchainPriceV4(a.protocolAddr, ticker, a.rpcURL)
	if err != nil {
		return 0, err
	}
	return entry.Price, nil
}

func (a spotPriceAdapter) QuoteStockToIdrx(ctx context.Context, ticker string, stockAmountRaw *big.Int) (*big.Int, error) {
	return a.price.Price.QuoteStockToIdrxRaw(a.protocolAddr, ticker, stockAmountRaw, a.rpcURL)
}

// balanceAdapter reads the owner's IDRX balance on-chain, closing over the
// same env wiring as spotPriceAdapter (IDRX_ADDRESS/ALCHEMY_RPC_URL) so
// executor.BalanceReader stays free of it.
type balanceAdapter struct {
	price    *publicsvc.PriceService
	idrxAddr string
	rpcURL   string
}

func (a balanceAdapter) IDRXBalance(ctx context.Context, wallet string) (*big.Int, error) {
	if a.idrxAddr == "" {
		return nil, fmt.Errorf("agent: IDRX_ADDRESS not configured, cannot read balance")
	}
	return a.price.Price.ERC20BalanceOf(a.idrxAddr, wallet, a.rpcURL)
}

// newAgentTaskServices builds Quasar/Nova/Comet and wires them into the new
// orchestrator graph (agent/orchestrator_service.go), then into
// TaskService/SubTaskRetryService. Lives here, in package service, rather
// than package agent (task_service.go) — analyzer/executor already import
// package agent (agent.WrapToolGraceful, agent.RunContextFrom), so package
// agent importing them back would be an import cycle. Returns (nil, nil) and
// logs instead of failing NewRegistry outright — same "disabled, not fatal"
// pattern as the rest of NewRegistry's optional integrations.
//
// The old supervisor package (supervisor.New, an adk.ChatModelAgent with
// analyzer_agent/executor_agent/create_task as LLM-callable tools) is no
// longer built here — Quasar is now two plain LLM calls owned directly by
// the Orchestrator (see docs/plans/agent-orchestration-graph-rebuild.md),
// specifically because adk.ChatModelAgent can never stream. Quick/deep
// Analyzer model tiering is also not carried over yet (single Analyzer
// instance) — tracked as an open item in that same plan, not lost.
func newAgentTaskServices(repos *repository.Registry, chartReader *publicsvc.PortfolioChartReader) (*agentsvc.ChatService, *agentsvc.TaskService) {
	ctx := context.Background()

	// One model for every role now — deepseek-v4-pro is discontinued
	// 2026-09-14 and the Flash tier that replaced it beats it outright on a
	// direct side-by-side test (docs/plans/dynamic-model-tier-routing.md
	// v2.3), so there is no second tier left to reserve for the
	// higher-stakes role.
	supervisorModel, supervisorErr := external.NewDeepSeekChatModel(ctx, external.ModelFlash)
	analyzerModel, analyzerErr := external.NewDeepSeekChatModel(ctx, external.ModelFlash)
	executorModel, executorErr := external.NewDeepSeekChatModel(ctx, external.ModelFlash)

	// Zero fallback (docs/plans/agent-orchestration-graph-rebuild.md v2.5):
	// on-chain is the source of truth, so a missing/misconfigured real chain
	// client must disable the whole agent service, exactly like a missing
	// DeepSeek key does below — never silently substitute a fake client in
	// its place. StubAgentContractClient (the old fake) has been deleted
	// entirely, not just unused (v2.7) — there is no fallback type left to
	// reach for by mistake.
	contractClient, chainErr := agentsvc.NewClient(ctx)
	if chainErr != nil {
		log.Printf("agent task service disabled: on-chain client: %v", chainErr)
		return nil, nil
	}

	switch {
	case supervisorErr != nil:
		log.Printf("agent task service disabled: %v", supervisorErr)
		return nil, nil
	case analyzerErr != nil:
		log.Printf("agent task service disabled: %v", analyzerErr)
		return nil, nil
	case executorErr != nil:
		log.Printf("agent task service disabled: %v", executorErr)
		return nil, nil
	}

	analyzerAgent, err := analyzer.New(ctx, analyzerModel, chartReader, chartReader.Price)
	if err != nil {
		log.Printf("agent task service disabled: build analyzer agent: %v", err)
		return nil, nil
	}
	// contractClient itself implements executor.TaskExecutor for real now
	// (docs/plans/agent-trade-execution.md) — UnimplementedTaskExecutor is
	// retired from this wiring; a failed ExecuteTrade call fails loudly on
	// its own merits (a real, in-flight chain error), never a stub standing
	// in for one.
	prices := spotPriceAdapter{price: chartReader.Price, protocolAddr: os.Getenv("PULSAR_PROTOCOL"), rpcURL: os.Getenv("ALCHEMY_RPC_URL")}
	balances := balanceAdapter{price: chartReader.Price, idrxAddr: os.Getenv("IDRX_ADDRESS"), rpcURL: os.Getenv("ALCHEMY_RPC_URL")}
	executorAgent, err := executor.New(ctx, executorModel, &executor.DBPortfolioReader{Transactions: repos.StockTransaction}, contractClient, repos.Stock, prices, balances, repos.AgentTrade, repos.StockTransaction)

	if err != nil {
		log.Printf("agent task service disabled: build executor agent: %v", err)
		return nil, nil
	}

	// supervisorModel is used for both Quasar calls (route, reply) — a plain
	// stateless API client wrapper, safe to reuse for two logically separate
	// calls, no need to construct it twice.
	checkpointStore := agentsvc.NewPostgresCheckPointStore(repos.AgentCheckpoint)

	orchestrator, err := agentsvc.NewOrchestrator(ctx, &agentsvc.Orchestrator{
		Tasks:             repos.AgentTask,
		SubTasks:          repos.AgentSubTask,
		Chain:             contractClient,
		CheckPointStore:   checkpointStore,
		RouteModel:        supervisorModel,
		ReplyModel:        supervisorModel,
		Analyzer:          analyzerAgent,
		Executor:          executorAgent,
		RouteModelName:    external.ModelFlash,
		ReplyModelName:    external.ModelFlash,
		AnalyzerModelName: external.ModelFlash,
		ExecutorModelName: external.ModelFlash,
		AnalyzerIntake:    analyzer.RequiredIntake,
		ExecutorIntake:    executor.RequiredIntake,
		Stocks:            repos.Stock,
	})
	if err != nil {
		log.Printf("agent task service disabled: build orchestrator: %v", err)
		return nil, nil
	}

	chatSvc := &agentsvc.ChatService{
		Chats:           repos.AgentChat,
		ChatMessages:    repos.AgentChatMessage,
		Orchestrator:    orchestrator,
		CheckPointStore: checkpointStore,
	}
	taskSvc := &agentsvc.TaskService{
		Tasks:        repos.AgentTask,
		SubTasks:     repos.AgentSubTask,
		Trades:       repos.AgentTrade,
		ChatMessages: repos.AgentChatMessage,
		Chats:        repos.AgentChat,
		Chain:        contractClient,
		Executor:     executorAgent,
	}
	return chatSvc, taskSvc
}
