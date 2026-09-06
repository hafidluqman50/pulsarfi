package analyzer

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent"
	publicsvc "github.com/horizonlabs/pulsarfi-backend/src/service/public"
)

type portfolioSnapshotRequest struct {
	Lens      string `json:"lens" jsonschema_description:"Exactly one of: net_worth_vs_index, allocation, comparison_bar, cumulative_return. Any other value is rejected — never invent a lens outside this list, and never pass price_line here, that is get_stock_chart's job."`
	RangeName string `json:"range,omitempty" jsonschema_description:"Time range, e.g. 1M/3M/1Y/ALL — required for any lens with a time axis."`
	ChartQ    string `json:"chart_q" jsonschema_description:"The user's question verbatim."`
	LensNote  string `json:"lens_note" jsonschema_description:"One short sentence explaining why this lens answers the question."`
}

func newPortfolioSnapshotTool(portfolioReader publicsvc.ChartDataReader) (tool.InvokableTool, error) {
	return utils.InferTool(
		"get_portfolio_snapshot",
		"Fetches chart-ready data about the current wallet's own portfolio: net worth over time, allocation, a comparison across holdings, or cumulative return. Never use this for a question about a stock in general, use get_stock_chart for that instead.",
		func(ctx context.Context, req portfolioSnapshotRequest) (publicsvc.ChartPayload, error) {
			rc, ok := agent.RunContextFrom(ctx)
			if !ok {
				return publicsvc.ChartPayload{}, fmt.Errorf("get_portfolio_snapshot: no run context bound to this call")
			}

			lens := publicsvc.ChartLens(req.Lens)
			if !publicsvc.PortfolioLenses[lens] {
				return publicsvc.ChartPayload{}, fmt.Errorf("get_portfolio_snapshot: %q is not a portfolio lens, refusing", req.Lens)
			}

			data, err := portfolioReader.Fetch(ctx, lens, rc.Wallet, req.RangeName)
			if err != nil {
				return publicsvc.ChartPayload{}, fmt.Errorf("get_portfolio_snapshot: fetch data: %w", err)
			}

			return publicsvc.ChartPayload{
				ChartQ:   truncate(req.ChartQ, 200),
				Lens:     lens,
				LensNote: truncate(req.LensNote, 300),
				Data:     data,
			}, nil
		},
	)
}

type stockChartRequest struct {
	Ticker    string `json:"ticker" jsonschema_description:"The exact ticker to chart. Must be a real, existing ticker — never invent or guess one."`
	RangeName string `json:"range,omitempty" jsonschema_description:"Time range, e.g. 1M/3M/1Y/ALL."`
	ChartQ    string `json:"chart_q" jsonschema_description:"The user's question verbatim."`
	LensNote  string `json:"lens_note" jsonschema_description:"One short sentence explaining what this chart shows."`
}

func newStockChartTool(priceSvc *publicsvc.PriceService) (tool.InvokableTool, error) {
	return utils.InferTool(
		"get_stock_chart",
		"Fetches a stock's own price history, sourced fresh from real market data (Yahoo/IDX), never from the user's own portfolio. Use this for any question about a stock in general, never get_portfolio_snapshot.",
		func(ctx context.Context, req stockChartRequest) (publicsvc.ChartPayload, error) {
			data, err := priceSvc.PriceLineHistory(ctx, req.Ticker, req.RangeName)
			if errors.Is(err, publicsvc.ErrStockNotFound) {
				return publicsvc.ChartPayload{
					ChartQ:   truncate(req.ChartQ, 200),
					Lens:     publicsvc.LensPriceLine,
					Ticker:   strings.ToUpper(req.Ticker),
					LensNote: fmt.Sprintf("No chart data is available for %q, it may not be a real IDX ticker or the market data provider has no history for it.", req.Ticker),
					Data:     []any{},
				}, nil
			}
			if err != nil {
				return publicsvc.ChartPayload{}, fmt.Errorf("get_stock_chart: fetch data: %w", err)
			}

			return publicsvc.ChartPayload{
				ChartQ:   truncate(req.ChartQ, 200),
				Lens:     publicsvc.LensPriceLine,
				Ticker:   strings.ToUpper(req.Ticker),
				LensNote: truncate(req.LensNote, 300),
				Data:     data,
			}, nil
		},
	)
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
