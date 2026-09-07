package service

import (
	"context"
	"log"

	"github.com/cloudwego/eino/components/tool"
	"github.com/horizonlabs/pulsarfi-backend/src/config"
	"github.com/horizonlabs/pulsarfi-backend/src/onchain/agenttaskmanager"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	agentsvc "github.com/horizonlabs/pulsarfi-backend/src/service/agent"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/analyzer"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/executor"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/supervisor"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
	publicsvc "github.com/horizonlabs/pulsarfi-backend/src/service/public"
)

// newAgentTaskServices builds the three agent roles (Supervisor/Analyzer/
// Executor) and wires them into TaskService/SubTaskRetryService. Lives here,
// in package service, rather than package agent (task_service.go) — every
// one of analyzer/executor/supervisor already imports package agent
// (agent.WrapToolGraceful, agent.RunContextFrom), so package agent importing
// them back would be an import cycle. Returns (nil, nil) and logs instead of
// failing NewRegistry outright — same "disabled, not fatal" pattern as the
// rest of NewRegistry's optional integrations.
func newAgentTaskServices(repos *repository.Registry, chartReader *publicsvc.PortfolioChartReader) (*agentsvc.TaskService, *agentsvc.SubTaskRetryService) {
	ctx := context.Background()

	// Supervisor runs on every prompt (cheap, high-volume routing — Flash
	// tier). Analyzer/Executor only run once Supervisor has already decided
	// they're needed — rarer, deeper reasoning, so they get the Pro tier.
	supervisorModel, supervisorErr := external.NewDeepSeekChatModelFromEnv(ctx, external.ModelFlash)
	analyzerModel, analyzerErr := external.NewDeepSeekChatModelFromEnv(ctx, external.ModelPro)
	executorModel, executorErr := external.NewDeepSeekChatModelFromEnv(ctx, external.ModelPro)
	// Analyzer's own news tools — an agent's own tools live in that agent's
	// own folder, not a shared top-level file. Tavily replaced the previous
	// DuckDuckGo-backed search tool (unreliable, scraping-based, and a
	// confirmed real failure point live) — see analyzer/tools_service.go's
	// own doc comment. read_article gives Analyzer the article's full text,
	// not just the search snippet — gated to analyzer.TrustedNewsDomains so
	// the trust decision is enforced in code, not left to the LLM.
	tavilyAPIKey, tavilyAPIKeyErr := config.RequireEnv("TAVILY_API_KEY")
	var searchTool tool.InvokableTool
	var searchToolErr error
	if tavilyAPIKeyErr == nil {
		searchTool, searchToolErr = analyzer.NewSearchTool(tavilyAPIKey, 5, analyzer.TrustedNewsDomains)
	} else {
		searchToolErr = tavilyAPIKeyErr
	}
	readArticleTool, readArticleToolErr := analyzer.NewReadArticleTool(analyzer.TrustedNewsDomains)

	// StubTaskExecutor remains the ExecuteTrade path — onchain.Client's own
	// ExecuteTrade is deliberately not implemented yet (executor.TaskExecutor's
	// interface is missing subTaskId/token/minimumOutputAmount/summary, see
	// onchain/agenttaskmanager/client_service.go's doc comment). CreateTask/
	// GrantTradePermission/RecordSubTasks/CancelTask are real once
	// ALCHEMY_RPC_URL/AGENT_WALLET_PRIVATE_KEY/AGENT_TASK_MANAGER_ADDRESS are
	// all set — falls back to the stub otherwise, same "disabled, not fatal"
	// pattern as the rest of this function.
	taskExecutor := executor.StubTaskExecutor{}
	var contractClient agentsvc.AgentContractClient
	if realClient, err := agenttaskmanager.NewClientFromEnv(ctx); err != nil {
		log.Printf("agent on-chain client disabled, using stub: %v", err)
		contractClient = &agentsvc.StubAgentContractClient{}
	} else {
		contractClient = realClient
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
	case readArticleToolErr != nil:
		log.Printf("agent task service disabled: %v", readArticleToolErr)
		return nil, nil
	}

	// searchToolErr (missing TAVILY_API_KEY) degrades, it does not disable —
	// only news-based trigger evaluation loses its evidence source; chart,
	// portfolio, and trade requests never touch web_search at all, so they
	// have no reason to be down along with it. Every other missing piece
	// above genuinely leaves the whole pipeline unable to run at all, which
	// is why those stay fatal.
	analyzerExtraTools := []tool.BaseTool{readArticleTool}
	if searchToolErr != nil {
		log.Printf("web_search disabled (news evidence unavailable, chart/portfolio/trade unaffected): %v", searchToolErr)
	} else {
		analyzerExtraTools = append(analyzerExtraTools, searchTool)
	}

	analyzerAgent, err := analyzer.New(ctx, analyzerModel, chartReader, chartReader.Price, analyzerExtraTools...)
	if err != nil {
		log.Printf("agent task service disabled: build analyzer agent: %v", err)
		return nil, nil
	}
	executorAgent, err := executor.New(ctx, executorModel, &executor.DBPortfolioReader{Transactions: repos.StockTransaction}, taskExecutor, repos.Stock)
	if err != nil {
		log.Printf("agent task service disabled: build executor agent: %v", err)
		return nil, nil
	}
	supervisorAgent, err := supervisor.New(ctx, supervisorModel, analyzerAgent, executorAgent, repos.AgentTask, repos.AgentSubTask)
	if err != nil {
		log.Printf("agent task service disabled: build supervisor agent: %v", err)
		return nil, nil
	}

	taskSvc := &agentsvc.TaskService{
		Tasks:        repos.AgentTask,
		SubTasks:     repos.AgentSubTask,
		Trades:       repos.AgentTrade,
		Chats:        repos.AgentChat,
		ChatMessages: repos.AgentChatMessage,
		Supervisor:   supervisorAgent,
		Chain:        contractClient,
	}
	retrySvc := &agentsvc.SubTaskRetryService{
		Tasks:    repos.AgentTask,
		SubTasks: repos.AgentSubTask,
		Chain:    contractClient,
	}
	return taskSvc, retrySvc
}
