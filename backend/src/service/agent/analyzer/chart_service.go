package analyzer

import (
	"context"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent"
	publicsvc "github.com/horizonlabs/pulsarfi-backend/src/service/public"
)

// Portfolio/chart questions are Tasks too (isActionable = false), created
// via create_task like any other informational request — Task = any
// request that needs data fetched or searched on the user's behalf; NOT a
// Task = pure conversation with nothing to fetch. This file is the tool
// Analyzer calls once Supervisor has routed such a Task to it — Executor
// stays strictly about executing trades, Analyzer stays about gathering
// and presenting evidence, which now includes rendering it as a chart.

type ChartLens string

const (
	LensNetWorthVsIndex  ChartLens = "net_worth_vs_index"
	LensAllocation       ChartLens = "allocation"
	LensPriceLine        ChartLens = "price_line"
	LensComparisonBar    ChartLens = "comparison_bar"
	LensCumulativeReturn ChartLens = "cumulative_return"
	// drift_from_target deliberately not included yet — blocked on the
	// deferred Risk Profile feature (agent-task-manager-rebuild.md §5),
	// not a supported lens until that ships.
)

var allowedLenses = map[ChartLens]bool{
	LensNetWorthVsIndex:  true,
	LensAllocation:       true,
	LensPriceLine:        true,
	LensComparisonBar:    true,
	LensCumulativeReturn: true,
}

// ChartPayload becomes agent_chat_messages.ui_props verbatim once this
// Task's final reply is persisted. Data's shape depends on Lens — the
// frontend's lensToOption.ts switches on Lens the same way this file does.
type ChartPayload struct {
	ChartQ   string    `json:"chartQ"`
	Lens     ChartLens `json:"lens"`
	LensNote string    `json:"lensNote"`
	Data     any       `json:"data"`
}

type NetWorthVsIndexPoint struct {
	Date        string  `json:"date"`
	NetWorthIDR string  `json:"netWorthIdr"`
	IndexValue  float64 `json:"indexValue"`
}

type AllocationSlice struct {
	Ticker     string  `json:"ticker"`
	ValueIDR   string  `json:"valueIdr"`
	Percentage float64 `json:"percentage"`
}

// PriceLinePoint is a plain price-over-time series — renamed from the
// originally-proposed "price_candles"/candlestick lens: the existing
// PriceService.GetStockHistory only returns one value per point (a
// closing price), not full OHLC, so a true candlestick chart isn't
// supportable without a new historical-OHLC data source. This is a
// discovered constraint, not silently faked as a candlestick.
type PriceLinePoint struct {
	Date  string  `json:"date"`
	Price float64 `json:"price"`
}

type ComparisonBarEntry struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

type CumulativeReturnPoint struct {
	Date          string  `json:"date"`
	ReturnPercent float64 `json:"returnPercent"`
}

// ChartDataReader wraps the existing public.StockTransactionService/
// public.PriceService equivalents — the real security boundary of this
// whole feature. The LLM never supplies Data itself; it only ever picks
// Lens (validated against allowedLenses below) plus, for lenses that need
// one, a ticker/range — both of which this reader must independently
// validate against real data, never trust as given.
type ChartDataReader interface {
	Fetch(ctx context.Context, lens ChartLens, wallet, ticker, rangeName string) (data any, err error)
}

// PortfolioChartReader is the concrete ChartDataReader — it replays real
// transaction history (net IDRX capital deployed per ticker) as an
// approximation of position value, the same approach used for the
// frontend's own portfolio-value-over-time chart
// (frontend/lib/portfolio.ts buildPortfolioSeries) — not a full historical
// mark-to-market, which would need per-ticker historical prices merged
// across time.
type PortfolioChartReader struct {
	Transactions *repository.StockTransactionRepository
	Price        *publicsvc.PriceService
}

func (r *PortfolioChartReader) Fetch(ctx context.Context, lens ChartLens, wallet, ticker, rangeName string) (any, error) {
	switch lens {
	case LensPriceLine:
		return r.priceLine(ctx, ticker, rangeName)
	case LensNetWorthVsIndex:
		return r.netWorthVsIndex(ctx, wallet, rangeName)
	case LensAllocation:
		return r.allocation(ctx, wallet)
	case LensComparisonBar:
		return r.comparisonBar(ctx, wallet)
	case LensCumulativeReturn:
		return r.cumulativeReturn(ctx, wallet)
	default:
		return nil, fmt.Errorf("chart data reader: unhandled lens %q", lens)
	}
}

func (r *PortfolioChartReader) priceLine(ctx context.Context, ticker, rangeName string) ([]PriceLinePoint, error) {
	if ticker == "" {
		return nil, fmt.Errorf("price_line: ticker is required")
	}
	history, err := r.Price.GetStockHistory(ctx, ticker, "idx", rangeName)
	if err != nil {
		return nil, err
	}
	points := make([]PriceLinePoint, 0, len(history))
	for _, p := range history {
		points = append(points, PriceLinePoint{Date: formatTimestamp(p.Timestamp), Price: p.Value})
	}
	return points, nil
}

// netCapitalByTicker replays a wallet's transactions into running net IDRX
// capital deployed per ticker — buys/transfers-in add, sells/transfers-out/
// redeems subtract. The same approximation as buildCostBasis on the
// frontend (lib/portfolio.ts), reused here on the Go side for the chat
// chart tool.
func (r *PortfolioChartReader) netCapitalByTicker(ctx context.Context, wallet string) (map[string]*big.Int, []model.StockTransaction, error) {
	txs, err := r.Transactions.FindByWallet(ctx, wallet)
	if err != nil {
		return nil, nil, err
	}
	net := map[string]*big.Int{}
	for i := len(txs) - 1; i >= 0; i-- { // FindByWallet orders newest-first; replay oldest-first
		tx := txs[i]
		idrx, ok := new(big.Int).SetString(tx.IdrxAmount, 10)
		if !ok {
			continue
		}
		ticker := tx.Stock.Ticker
		if net[ticker] == nil {
			net[ticker] = big.NewInt(0)
		}
		switch strings.ToLower(tx.Side) {
		case "buy", "transfer-in":
			net[ticker].Add(net[ticker], idrx)
		case "sell", "transfer-out", "redeemed":
			net[ticker].Sub(net[ticker], idrx)
			if net[ticker].Sign() < 0 {
				net[ticker].SetInt64(0)
			}
		}
	}
	return net, txs, nil
}

func (r *PortfolioChartReader) allocation(ctx context.Context, wallet string) ([]AllocationSlice, error) {
	net, _, err := r.netCapitalByTicker(ctx, wallet)
	if err != nil {
		return nil, err
	}
	total := big.NewInt(0)
	for _, v := range net {
		total.Add(total, v)
	}
	slices := make([]AllocationSlice, 0, len(net))
	for ticker, value := range net {
		if value.Sign() <= 0 {
			continue
		}
		percentage := 0.0
		if total.Sign() > 0 {
			pct := new(big.Float).Quo(new(big.Float).SetInt(value), new(big.Float).SetInt(total))
			pct.Mul(pct, big.NewFloat(100))
			percentage, _ = pct.Float64()
		}
		slices = append(slices, AllocationSlice{Ticker: ticker, ValueIDR: value.String(), Percentage: percentage})
	}
	return slices, nil
}

func (r *PortfolioChartReader) comparisonBar(ctx context.Context, wallet string) ([]ComparisonBarEntry, error) {
	net, _, err := r.netCapitalByTicker(ctx, wallet)
	if err != nil {
		return nil, err
	}
	entries := make([]ComparisonBarEntry, 0, len(net))
	for ticker, value := range net {
		if value.Sign() <= 0 {
			continue
		}
		f, _ := new(big.Float).SetInt(value).Float64()
		entries = append(entries, ComparisonBarEntry{Label: ticker, Value: f})
	}
	return entries, nil
}

func (r *PortfolioChartReader) netWorthVsIndex(ctx context.Context, wallet, rangeName string) ([]NetWorthVsIndexPoint, error) {
	_, txs, err := r.netCapitalByTicker(ctx, wallet)
	if err != nil {
		return nil, err
	}
	indexHistory, err := r.Price.GetStockHistory(ctx, "IHSG", "", rangeName)
	if err != nil {
		return nil, err
	}

	running := big.NewInt(0)
	points := make([]NetWorthVsIndexPoint, 0, len(txs))
	for i := len(txs) - 1; i >= 0; i-- {
		tx := txs[i]
		idrx, ok := new(big.Int).SetString(tx.IdrxAmount, 10)
		if !ok {
			continue
		}
		switch strings.ToLower(tx.Side) {
		case "buy", "transfer-in":
			running.Add(running, idrx)
		case "sell", "transfer-out", "redeemed":
			running.Sub(running, idrx)
			if running.Sign() < 0 {
				running.SetInt64(0)
			}
		}
		var indexValue float64
		if len(indexHistory) > 0 {
			indexValue = indexHistory[len(indexHistory)-1].Value
		}
		points = append(points, NetWorthVsIndexPoint{
			Date:        tx.CreatedAt.Format("2006-01-02"),
			NetWorthIDR: running.String(),
			IndexValue:  indexValue,
		})
	}
	return points, nil
}

func (r *PortfolioChartReader) cumulativeReturn(ctx context.Context, wallet string) ([]CumulativeReturnPoint, error) {
	series, err := r.netWorthVsIndex(ctx, wallet, "ALL")
	if err != nil {
		return nil, err
	}
	if len(series) == 0 {
		return nil, nil
	}
	base, ok := new(big.Float).SetString(series[0].NetWorthIDR)
	if !ok || base.Sign() == 0 {
		base = big.NewFloat(1)
	}
	points := make([]CumulativeReturnPoint, 0, len(series))
	for _, p := range series {
		value, _ := new(big.Float).SetString(p.NetWorthIDR)
		if value == nil {
			value = big.NewFloat(0)
		}
		diff := new(big.Float).Sub(value, base)
		pct := new(big.Float).Quo(diff, base)
		pct.Mul(pct, big.NewFloat(100))
		f, _ := pct.Float64()
		points = append(points, CumulativeReturnPoint{Date: p.Date, ReturnPercent: f})
	}
	return points, nil
}

func formatTimestamp(unixSeconds int64) string {
	return time.Unix(unixSeconds, 0).UTC().Format("2006-01-02")
}

type portfolioChartRequest struct {
	Lens      string `json:"lens" jsonschema_description:"Exactly one of: net_worth_vs_index, allocation, price_line, comparison_bar, cumulative_return. Any other value is rejected — never invent a lens outside this list."`
	Ticker    string `json:"ticker,omitempty" jsonschema_description:"Required only for price_line — the ticker to chart. Must be a real, existing ticker."`
	RangeName string `json:"range,omitempty" jsonschema_description:"Time range, e.g. 1M/3M/1Y/ALL — required for any lens with a time axis."`
	ChartQ    string `json:"chart_q" jsonschema_description:"The user's question verbatim."`
	LensNote  string `json:"lens_note" jsonschema_description:"One short sentence explaining why this lens answers the question."`
}

// newPortfolioChartTool is the prompt-injection boundary for this whole
// capability. Analyzer reaches this after reading attacker-influenceable
// external content (search results, articles) via its other tools — a
// poisoned source could try to make the model claim a fake lens or fake
// numbers. Two things make that claim inert: Lens is checked against a
// closed server-side enum (not the model's word for it), and Data always
// comes from reader.Fetch's own DB/service query, never from req itself.
func newPortfolioChartTool(reader ChartDataReader) (tool.InvokableTool, error) {
	return utils.InferTool(
		"get_portfolio_snapshot",
		"Fetches chart-ready portfolio/price data for the current wallet. lens must be exactly one of the five allowed values — never anything else.",
		func(ctx context.Context, req portfolioChartRequest) (ChartPayload, error) {
			rc, ok := agent.RunContextFrom(ctx)
			if !ok {
				return ChartPayload{}, fmt.Errorf("get_portfolio_snapshot: no run context bound to this call")
			}

			lens := ChartLens(req.Lens)
			if !allowedLenses[lens] {
				return ChartPayload{}, fmt.Errorf("get_portfolio_snapshot: %q is not an allowed lens, refusing", req.Lens)
			}

			data, err := reader.Fetch(ctx, lens, rc.Wallet, req.Ticker, req.RangeName)
			if err != nil {
				return ChartPayload{}, fmt.Errorf("get_portfolio_snapshot: fetch data: %w", err)
			}

			return ChartPayload{
				ChartQ:   truncate(req.ChartQ, 200),
				Lens:     lens,
				LensNote: truncate(req.LensNote, 300),
				Data:     data,
			}, nil
		},
	)
}

// truncate caps free-text fields Analyzer supplies (ChartQ, LensNote) —
// pure display captions, never used to drive logic, but still bounded so a
// crafted input can't stuff an oversized blob into storage.
func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max]
}
