package public

import (
	"context"
	"fmt"
	"math/big"
	"sort"
	"strings"
	"time"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
)

type ChartLens string

const (
	LensNetWorthVsIndex  ChartLens = "net_worth_vs_index"
	LensAllocation       ChartLens = "allocation"
	LensPriceLine        ChartLens = "price_line"
	LensComparisonBar    ChartLens = "comparison_bar"
	LensCumulativeReturn ChartLens = "cumulative_return"
)

var AllowedLenses = map[ChartLens]bool{
	LensNetWorthVsIndex:  true,
	LensAllocation:       true,
	LensPriceLine:        true,
	LensComparisonBar:    true,
	LensCumulativeReturn: true,
}

var PortfolioLenses = map[ChartLens]bool{
	LensNetWorthVsIndex:  true,
	LensAllocation:       true,
	LensComparisonBar:    true,
	LensCumulativeReturn: true,
}

type ChartPayload struct {
	ChartQ   string    `json:"chartQ"`
	Lens     ChartLens `json:"lens"`
	Ticker   string    `json:"ticker,omitempty"`
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

type ComparisonBarEntry struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

type CumulativeReturnPoint struct {
	Date          string  `json:"date"`
	ReturnPercent float64 `json:"returnPercent"`
}

type ChartDataReader interface {
	Fetch(ctx context.Context, lens ChartLens, wallet, rangeName string) (data any, err error)
}

type PortfolioChartReader struct {
	Transactions *repository.StockTransactionRepository
	Price        *PriceService
}

func (r *PortfolioChartReader) Fetch(ctx context.Context, lens ChartLens, wallet, rangeName string) (any, error) {
	switch lens {
	case LensNetWorthVsIndex:
		return r.netWorthVsIndex(ctx, wallet, rangeName)
	case LensAllocation:
		return r.allocation(ctx, wallet)
	case LensComparisonBar:
		return r.comparisonBar(ctx, wallet)
	case LensCumulativeReturn:
		return r.cumulativeReturn(ctx, wallet)
	default:
		return nil, fmt.Errorf("portfolio chart reader: %q is not a portfolio lens", lens)
	}
}

func (r *PortfolioChartReader) netCapitalByTicker(ctx context.Context, wallet string) (map[string]*big.Int, []model.StockTransaction, error) {
	txs, err := r.Transactions.FindByWallet(ctx, wallet)
	if err != nil {
		return nil, nil, err
	}
	net := map[string]*big.Int{}
	for i := len(txs) - 1; i >= 0; i-- {
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
	indexHistory, err := r.Price.GetStockHistory(ctx, "IHSG", rangeName)
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
		points = append(points, NetWorthVsIndexPoint{
			Date:        tx.CreatedAt.Format("2006-01-02"),
			NetWorthIDR: running.String(),
			IndexValue:  indexValueAt(indexHistory, tx.CreatedAt),
		})
	}
	return points, nil
}

func indexValueAt(indexHistory []external.PriceHistoryPoint, at time.Time) float64 {
	if len(indexHistory) == 0 {
		return 0
	}
	sorted := make([]external.PriceHistoryPoint, len(indexHistory))
	copy(sorted, indexHistory)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Timestamp < sorted[j].Timestamp })

	target := at.UnixMilli()
	best := sorted[0]
	for _, point := range sorted {
		if point.Timestamp > target {
			break
		}
		best = point
	}
	return best.Value
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
