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
		Name:        "analyzer_agent",
		Description: "Gathers news and/or technical evidence for a Task's trigger condition and concludes whether it is satisfied, citing the specific evidence that drove the conclusion. Also answers portfolio/chart questions.",
		Instruction: instructions,
		Model:       chatModel,
		ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools}},
		// Raised from 10, found live: a genuinely legitimate two-ticker
		// request ("berita & sentimen BUMI dan ENRG sekaligus") needs
		// web_search + read_article for each ticker separately, plus the
		// final synthesis — comfortably more than 10 tool-call iterations
		// for a real, non-looping request, and it failed with exactly
		// ErrExceedMaxIterations (caught non-fatally by WrapToolGraceful,
		// but the whole point of gathering evidence never completed, so no
		// NewsBrief card had anything to render). 10 was itself already a
		// deliberate reduction from eino's own default of 20, chosen only
		// to fail a genuine infinite tool-call loop faster — this restores
		// that original default rather than picking a new number blind.
		MaxIterations: 20,
	})
	if err != nil {
		return nil, fmt.Errorf("analyzer: build agent: %w", err)
	}
	return analyzerAgent, nil
}
