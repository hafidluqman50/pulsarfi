package public

import (
	"context"
	"errors"
	"os"
	"strings"

	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
)

var ErrStockNotFound = errors.New("stock not found")

type PriceService struct {
	Stocks *repository.StockRepository
	Price  *external.PriceService
}

func (s *PriceService) GetStockPrice(ctx context.Context, ticker string) (external.PriceEntry, error) {
	ticker = strings.ToUpper(ticker)
	if ticker == "IHSG" {
		return s.Price.GetIHSG()
	}

	stock, found, err := s.Stocks.FindByTickerOrIdxTicker(ctx, ticker)
	if err != nil {
		return external.PriceEntry{}, err
	}
	if !found {
		return external.PriceEntry{}, ErrStockNotFound
	}

	if stock.ContractAddress == nil || *stock.ContractAddress == "" {
		return s.Price.GetYahooIDX(stock.IdxTicker)
	}

	entry, err := s.Price.GetOnchainPrice(
		*stock.ContractAddress,
		os.Getenv("IDRX_ADDRESS"),
		os.Getenv("UNISWAP_V2_FACTORY"),
		os.Getenv("ALCHEMY_RPC_URL"),
	)
	if err != nil {
		return external.PriceEntry{}, err
	}

	if yahoo, err := s.Price.GetYahooIDX(stock.IdxTicker); err == nil {
		entry.Change24h = yahoo.Change24h
	}

	return entry, nil
}
