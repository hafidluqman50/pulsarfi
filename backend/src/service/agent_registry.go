package service

import (
	"context"
	"log"

	"github.com/horizonlabs/pulsarfi-backend/src/onchain/agenttaskmanager"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	agentsvc "github.com/horizonlabs/pulsarfi-backend/src/service/agent"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/analyzer"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/executor"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
	publicsvc "github.com/horizonlabs/pulsarfi-backend/src/service/public"
)

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
func newAgentTaskServices(repos *repository.Registry, chartReader *publicsvc.PortfolioChartReader) (*agentsvc.TaskService, *agentsvc.SubTaskRetryService) {
	ctx := context.Background()

	// Quasar (route/reply) runs on every turn, at least twice — cheap,
	// high-volume, Flash tier. Executor is always Pro — any call means an
	// action might actually be taken, the highest-stakes case regardless of
	// what led there.
	supervisorModel, supervisorErr := external.NewDeepSeekChatModelFromEnv(ctx, external.ModelFlash)
	analyzerModel, analyzerErr := external.NewDeepSeekChatModelFromEnv(ctx, external.ModelFlash)
	executorModel, executorErr := external.NewDeepSeekChatModelFromEnv(ctx, external.ModelPro)

	// Real trade execution against AgentTaskManager.sol has never been built
	// (executor.TaskExecutor's interface is missing subTaskId/token/
	// minimumOutputAmount/summary, see onchain/agenttaskmanager/client_service.go's
	// doc comment) — UnimplementedTaskExecutor fails loudly on every call
	// (ErrTradeExecutionNotImplemented) rather than fabricating a fake tx
	// hash, per the zero-fallback rule
	// (docs/plans/agent-orchestration-graph-rebuild.md v2.5/v2.7). Tracked
	// in that plan's §11 as unfinished-feature work, not a fallback for one
	// that sometimes works.
	taskExecutor := executor.UnimplementedTaskExecutor{}

	// Zero fallback (docs/plans/agent-orchestration-graph-rebuild.md v2.5):
	// on-chain is the source of truth, so a missing/misconfigured real chain
	// client must disable the whole agent service, exactly like a missing
	// DeepSeek key does below — never silently substitute a fake client in
	// its place. StubAgentContractClient (the old fake) has been deleted
	// entirely, not just unused (v2.7) — there is no fallback type left to
	// reach for by mistake.
	contractClient, chainErr := agenttaskmanager.NewClientFromEnv(ctx)
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
	executorAgent, err := executor.New(ctx, executorModel, &executor.DBPortfolioReader{Transactions: repos.StockTransaction}, taskExecutor, repos.Stock)
	if err != nil {
		log.Printf("agent task service disabled: build executor agent: %v", err)
		return nil, nil
	}

	// supervisorModel is used for both Quasar calls (route, reply) — a plain
	// stateless API client wrapper, safe to reuse for two logically separate
	// calls, no need to construct it twice.
	orchestrator, err := agentsvc.NewOrchestrator(ctx, &agentsvc.Orchestrator{
		Tasks:      repos.AgentTask,
		SubTasks:   repos.AgentSubTask,
		Chain:      contractClient,
		RouteModel: supervisorModel,
		ReplyModel: supervisorModel,
		Analyzer:   analyzerAgent,
		Executor:   executorAgent,
	})
	if err != nil {
		log.Printf("agent task service disabled: build orchestrator: %v", err)
		return nil, nil
	}

	taskSvc := &agentsvc.TaskService{
		Tasks:        repos.AgentTask,
		SubTasks:     repos.AgentSubTask,
		Trades:       repos.AgentTrade,
		Chats:        repos.AgentChat,
		ChatMessages: repos.AgentChatMessage,
		Orchestrator: orchestrator,
		Chain:        contractClient,
	}
	retrySvc := &agentsvc.SubTaskRetryService{
		Tasks:    repos.AgentTask,
		SubTasks: repos.AgentSubTask,
		Chain:    contractClient,
	}
	return taskSvc, retrySvc
}
