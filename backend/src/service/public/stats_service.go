package public

import (
	"context"

	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
)

type StatsService struct {
	Transactions *repository.StockTransactionRepository
	Stocks       *repository.StockRepository
	Price        *external.PriceService
}

type ProtocolStatsResponse struct {
	Volume24h  float64 `json:"volume_24h"`
	TvlIdrx    float64 `json:"tvl_idrx"`
	PairCount  int     `json:"pair_count"`
	IdrUsdRate float64 `json:"idr_usd_rate"`
}

func (s *StatsService) GetStats(ctx context.Context) (ProtocolStatsResponse, error) {
	stats, err := s.Transactions.ComputeStats(ctx)
	if err != nil {
		return ProtocolStatsResponse{}, err
	}
	stocks, err := s.Stocks.FindMarketReady(ctx)
	if err != nil {
		return ProtocolStatsResponse{}, err
	}
	rate, err := s.Price.GetUSDIDRRate()
	if err != nil {
		rate = 16142 // fallback if Yahoo is unavailable
	}
	return ProtocolStatsResponse{
		Volume24h:  stats.Volume24h,
		TvlIdrx:   stats.TvlIdrx,
		PairCount:  len(stocks),
		IdrUsdRate: rate,
	}, nil
}
