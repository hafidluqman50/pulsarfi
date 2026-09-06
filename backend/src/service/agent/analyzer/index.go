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
	tools := []tool.BaseTool{agent.WrapToolGraceful(portfolioSnapshotTool), agent.WrapToolGraceful(stockChartTool)}
	for _, t := range extraTools {
		if it, ok := t.(tool.InvokableTool); ok {
			tools = append(tools, agent.WrapToolGraceful(it))
		} else {
			tools = append(tools, t)
		}
	}

	analyzerAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "analyzer_agent",
		Description:   "Gathers news and/or technical evidence for a Task's trigger condition and concludes whether it is satisfied, citing the specific evidence that drove the conclusion. Also answers portfolio/chart questions.",
		Instruction:   instructions,
		Model:         chatModel,
		ToolsConfig:   adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools}},
		MaxIterations: 10,
	})
	if err != nil {
		return nil, fmt.Errorf("analyzer: build agent: %w", err)
	}
	return analyzerAgent, nil
}
