package analyzer

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent"
	publicsvc "github.com/horizonlabs/pulsarfi-backend/src/service/public"
)

func New(ctx context.Context, chatModel model.ToolCallingChatModel, chartReader publicsvc.ChartDataReader, priceSvc *publicsvc.PriceService, extraTools ...tool.BaseTool) (adk.Agent, error) {
	portfolioSnapshotTool, err := newPortfolioSnapshotTool(chartReader)
	if err != nil {
		return nil, fmt.Errorf("analyzer: build get_portfolio_snapshot tool: %w", err)
	}
	stockChartTool, err := newStockChartTool(priceSvc)
	if err != nil {
		return nil, fmt.Errorf("analyzer: build get_stock_chart tool: %w", err)
	}
	verifyTickerTool, err := newVerifyTickerTool(priceSvc)
	if err != nil {
		return nil, fmt.Errorf("analyzer: build verify_ticker tool: %w", err)
	}
	readArticleTool, webSearchTool, err := NewNewsTools()
	if err != nil {
		return nil, fmt.Errorf("analyzer: build news tools: %w", err)
	}
	baseTools := []tool.BaseTool{portfolioSnapshotTool, stockChartTool, verifyTickerTool, readArticleTool, webSearchTool}
	tools := append(baseTools, extraTools...)

	analyzerAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "analyzer_agent",
		Description: "Gathers news and/or technical evidence for a Task's trigger condition and concludes whether it is satisfied, citing the specific evidence that drove the conclusion. Also answers portfolio/chart questions.",
		Instruction: instructions,
		Model:       chatModel,
		ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: agent.WrapToolsGraceful(tools)}},
		// Bounded at 6 iterations: strictly well below 20 to conserve on-chain
		// ETH gas, while giving enough headroom for a realistic news pipeline
		// (1 web_search + up to 2 read_article calls + final synthesis turn)
		// without hitting premature ErrExceedMaxIterations / HTTP 500 errors.
		MaxIterations: 6,
	})
	if err != nil {
		return nil, fmt.Errorf("analyzer: build agent: %w", err)
	}
	return analyzerAgent, nil
}
