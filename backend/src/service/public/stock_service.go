package public

import (
	"context"
	"os"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
)

type StockService struct {
	Stocks *repository.StockRepository
	Price  *external.PriceService
}

type MarketStockResponse struct {
	Ticker          string    `json:"ticker"`
	StockName       string    `json:"stock_name"`
	IdxTicker       string    `json:"idx_ticker"`
	Sector          *string   `json:"sector"`
	ContractAddress *string   `json:"contract_address"`
	Price           float64   `json:"price"`
	PoolPrice       float64   `json:"pool_price"`
	Change24h       float64   `json:"change_24h"`
	Sparkline7d     []float64 `json:"sparkline_7d"`
}

func (s *StockService) ListMarketStocks(ctx context.Context) ([]MarketStockResponse, error) {
	stocks, err := s.Stocks.FindMarketReady(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]MarketStockResponse, 0, len(stocks))
	for _, stock := range stocks {
		price, sparkline, err := s.Price.GetYahooIDXMarket(stock.IdxTicker)
		if err != nil {
			items = append(items, marketStockResponse(stock, external.PriceEntry{}, 0, nil))
			continue
		}
		poolPriceValue := 0.0
		if stock.ContractAddress != nil {
			poolPrice, err := s.Price.GetOnchainPrice(
				*stock.ContractAddress,
				os.Getenv("IDRX_ADDRESS"),
				os.Getenv("UNISWAP_V2_FACTORY"),
				os.Getenv("ALCHEMY_RPC_URL"),
			)
			if err == nil {
				poolPriceValue = poolPrice.Price
			}
		}
		items = append(items, marketStockResponse(stock, price, poolPriceValue, sparkline))
	}

	return items, nil
}

func marketStockResponse(stock model.Stock, price external.PriceEntry, poolPrice float64, sparkline []float64) MarketStockResponse {
	return MarketStockResponse{
		Ticker:          stock.Ticker,
		StockName:       stock.StockName,
		IdxTicker:       stock.IdxTicker,
		Sector:          stock.Sector,
		ContractAddress: stock.ContractAddress,
		Price:           price.Price,
		PoolPrice:       poolPrice,
		Change24h:       price.Change24h,
		Sparkline7d:     sparkline,
	}
}
