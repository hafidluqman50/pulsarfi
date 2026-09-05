package service

import (
	"context"
	"log"

	"github.com/horizonlabs/pulsarfi-backend/src/auth"
	"github.com/horizonlabs/pulsarfi-backend/src/onchain/agenttaskmanager"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	agentsvc "github.com/horizonlabs/pulsarfi-backend/src/service/agent"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/analyzer"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/executor"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/supervisor"
	authsvc "github.com/horizonlabs/pulsarfi-backend/src/service/auth"
	custodiansvc "github.com/horizonlabs/pulsarfi-backend/src/service/custodian"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
	indexersvc "github.com/horizonlabs/pulsarfi-backend/src/service/indexer"
	publicsvc "github.com/horizonlabs/pulsarfi-backend/src/service/public"
)

type Registry struct {
	Repos                  *repository.Registry
	Auth                   *authsvc.AuthService
	Custodian              *custodiansvc.CustodianService
	PublicStock            *publicsvc.StockService
	PublicPrice            *publicsvc.PriceService
	PublicReserve          *publicsvc.ReserveService
	PublicStockTransaction *publicsvc.StockTransactionService
	PublicStats            *publicsvc.StatsService
	PublicRedeem           *publicsvc.PublicRedeemService
	CustodianRedeem        *custodiansvc.RedeemService
	CustodianKYC           *custodiansvc.KYCService
	Email                  *external.EmailService
	Storage                *external.StorageService
	Stream                 *external.StreamService
	Price                  *external.PriceService
	TransferIndexer        *indexersvc.TransferIndexerService
	AgentTask              *agentsvc.TaskService
	AgentSubTaskRetry      *agentsvc.SubTaskRetryService
}

type Config struct {
	Repos                 *repository.Registry
	JwtConfig             auth.Config
	NonceStore            *auth.NonceStore
	EmailService          *external.EmailService
	StorageService        *external.StorageService
	TransferIndexerConfig indexersvc.TransferIndexerConfig
}

func NewRegistry(cfg Config) *Registry {
	stream := external.NewStreamService()
	price := external.NewPriceService()

	// Degrade to nil (not a crash) if DEEPSEEK_API_KEY isn't set — mirrors
	// the existing "disabled, not fatal" pattern for other optional external
	// integrations (email/storage) in this same function.
	var agentTaskSvc *agentsvc.TaskService
	var agentSubTaskRetrySvc *agentsvc.SubTaskRetryService
	// Supervisor runs on every prompt (cheap, high-volume routing — Flash
	// tier). Analyzer/Executor only run once Supervisor has already decided
	// they're needed — rarer, deeper reasoning, so they get the Pro tier.
	supervisorModel, supervisorErr := external.NewDeepSeekChatModelFromEnv(context.Background(), external.ModelFlash)
	analyzerModel, analyzerErr := external.NewDeepSeekChatModelFromEnv(context.Background(), external.ModelPro)
	executorModel, executorErr := external.NewDeepSeekChatModelFromEnv(context.Background(), external.ModelPro)
	// Analyzer's own news tools — moved into the analyzer package itself
	// (an agent's own tools live in that agent's own folder, not a shared
	// top-level file). DuckDuckGo needs no API key, unlike a
	// production-grade provider (e.g. Tavily) would — eino-ext flags it
	// "not recommended for production" (no stable API, scraping-based) —
	// fine for now, upgrade later. read_article gives Analyzer the
	// article's full text, not just the search snippet — gated to
	// analyzer.TrustedNewsDomains so the trust decision is enforced in
	// code, not left to the LLM.
	searchTool, searchToolErr := analyzer.NewSearchTool(context.Background(), 5)
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
	if realClient, err := agenttaskmanager.NewClientFromEnv(context.Background()); err != nil {
		log.Printf("agent on-chain client disabled, using stub: %v", err)
		contractClient = &agentsvc.StubAgentContractClient{}
	} else {
		contractClient = realClient
	}
	chartReader := &analyzer.PortfolioChartReader{
		Transactions: cfg.Repos.StockTransaction,
		Price: &publicsvc.PriceService{
			Stocks: cfg.Repos.Stock,
			Price:  price,
		},
	}
	if supervisorErr != nil {
		log.Printf("agent task service disabled: %v", supervisorErr)
	} else if analyzerErr != nil {
		log.Printf("agent task service disabled: %v", analyzerErr)
	} else if executorErr != nil {
		log.Printf("agent task service disabled: %v", executorErr)
	} else if searchToolErr != nil {
		log.Printf("agent task service disabled: %v", searchToolErr)
	} else if readArticleToolErr != nil {
		log.Printf("agent task service disabled: %v", readArticleToolErr)
	} else {
		ctx := context.Background()
		analyzerAgent, err := analyzer.New(ctx, analyzerModel, chartReader, searchTool, readArticleTool)
		if err != nil {
			log.Printf("agent task service disabled: build analyzer agent: %v", err)
		} else if executorAgent, err := executor.New(
			ctx,
			executorModel,
			&executor.DBPortfolioReader{Transactions: cfg.Repos.StockTransaction},
			taskExecutor,
			cfg.Repos.Stock,
		); err != nil {
			log.Printf("agent task service disabled: build executor agent: %v", err)
		} else if supervisorAgent, err := supervisor.New(ctx, supervisorModel, analyzerAgent, executorAgent, cfg.Repos.AgentTask, cfg.Repos.AgentSubTask); err != nil {
			log.Printf("agent task service disabled: build supervisor agent: %v", err)
		} else {
			agentTaskSvc = &agentsvc.TaskService{
				Tasks:        cfg.Repos.AgentTask,
				SubTasks:     cfg.Repos.AgentSubTask,
				Trades:       cfg.Repos.AgentTrade,
				Chats:        cfg.Repos.AgentChat,
				ChatMessages: cfg.Repos.AgentChatMessage,
				Supervisor:   supervisorAgent,
				Chain:        contractClient,
			}
			agentSubTaskRetrySvc = &agentsvc.SubTaskRetryService{
				Tasks:    cfg.Repos.AgentTask,
				SubTasks: cfg.Repos.AgentSubTask,
				Chain:    contractClient,
			}
		}
	}

	return &Registry{
		Repos: cfg.Repos,
		Auth: &authsvc.AuthService{
			Custodians: cfg.Repos.Custodian,
			Nonces:     cfg.NonceStore,
			JwtConfig:  cfg.JwtConfig,
		},
		Custodian: &custodiansvc.CustodianService{
			Repos:  cfg.Repos,
			Stream: stream,
			Price:  price,
		},
		PublicStock: &publicsvc.StockService{
			Stocks: cfg.Repos.Stock,
			Price:  price,
		},
		PublicPrice: &publicsvc.PriceService{
			Stocks: cfg.Repos.Stock,
			Price:  price,
		},
		PublicReserve: &publicsvc.ReserveService{
			Attestations: cfg.Repos.StockAttestation,
		},
		PublicStockTransaction: &publicsvc.StockTransactionService{
			Stocks:       cfg.Repos.Stock,
			Transactions: cfg.Repos.StockTransaction,
		},
		PublicStats: &publicsvc.StatsService{
			Transactions: cfg.Repos.StockTransaction,
			Stocks:       cfg.Repos.Stock,
			Price:        price,
		},
		PublicRedeem: &publicsvc.PublicRedeemService{
			Stocks:            cfg.Repos.Stock,
			RedeemProposals:   cfg.Repos.RedeemProposal,
			StockTransactions: cfg.Repos.StockTransaction,
		},
		CustodianRedeem: &custodiansvc.RedeemService{
			RedeemProposals:    cfg.Repos.RedeemProposal,
			RedeemAttestations: cfg.Repos.RedeemApproval,
			Custodians:         cfg.Repos.Custodian,
		},
		CustodianKYC: &custodiansvc.KYCService{
			WalletVerifications: cfg.Repos.WalletVerification,
			Custodians:          cfg.Repos.Custodian,
			Storage:             cfg.StorageService,
		},
		Email:   cfg.EmailService,
		Storage: cfg.StorageService,
		Stream:  stream,
		Price:   price,
		TransferIndexer: &indexersvc.TransferIndexerService{
			Stocks:      cfg.Repos.Stock,
			Checkpoints: cfg.Repos.TransferCheckpoint,
			Recorder: &publicsvc.StockTransactionService{
				Stocks:       cfg.Repos.Stock,
				Transactions: cfg.Repos.StockTransaction,
			},
			Price:  price,
			Config: cfg.TransferIndexerConfig,
		},
		AgentTask:         agentTaskSvc,
		AgentSubTaskRetry: agentSubTaskRetrySvc,
	}
}
