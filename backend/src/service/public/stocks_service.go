package public

import (
	"context"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
)

type StocksService struct {
	Stocks *repository.StockRepository
	Price  *external.PriceService
}

type MarketStockResponse struct {
	Ticker      string    `json:"ticker"`
	StockName   string    `json:"stock_name"`
	Sector      *string   `json:"sector"`
	Price       float64   `json:"price"`
	Change24h   float64   `json:"change_24h"`
	Sparkline7d []float64 `json:"sparkline_7d"`
}

func (s *StocksService) ListMarketStocks(ctx context.Context) ([]MarketStockResponse, error) {
	stocks, err := s.Stocks.FindMarketReady(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]MarketStockResponse, 0, len(stocks))
	for _, stock := range stocks {
		price, sparkline, err := s.Price.GetYahooIDXMarket(stock.IdxTicker)
		if err != nil {
			items = append(items, marketStockResponse(stock, external.PriceEntry{}, nil))
			continue
		}
		items = append(items, marketStockResponse(stock, price, sparkline))
	}

	return items, nil
}

func marketStockResponse(stock model.Stock, price external.PriceEntry, sparkline []float64) MarketStockResponse {
	return MarketStockResponse{
		Ticker:      stock.Ticker,
		StockName:   stock.StockName,
		Sector:      stock.Sector,
		Price:       price.Price,
		Change24h:   price.Change24h,
		Sparkline7d: sparkline,
	}
}
