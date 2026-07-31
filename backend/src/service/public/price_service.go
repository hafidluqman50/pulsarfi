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

func (s *PriceService) GetStockPrice(ctx context.Context, ticker string, source string) (external.PriceEntry, error) {
	if strings.EqualFold(source, "idx") {
		return s.GetIDXStockPrice(ctx, ticker)
	}

	ticker = strings.ToUpper(ticker)
	if ticker == "USDIDR" || ticker == "IDRUSD" || ticker == "IDR=X" {
		return s.Price.GetUSDIDR()
	}
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

	entry, err := s.Price.GetOnchainPriceV4(
		os.Getenv("PULSAR_PROTOCOL"),
		stock.Ticker,
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

func (s *PriceService) GetStockHistory(ctx context.Context, ticker string, source string, rangeName string) ([]external.PriceHistoryPoint, error) {
	ticker = strings.ToUpper(ticker)
	if ticker == "IHSG" {
		return s.Price.GetIHSGHistory(rangeName)
	}

	stock, found, err := s.Stocks.FindByTickerOrIdxTicker(ctx, ticker)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrStockNotFound
	}

	if strings.EqualFold(source, "idx") || stock.ContractAddress == nil || *stock.ContractAddress == "" {
		return s.Price.GetYahooIDXHistory(stock.IdxTicker, rangeName)
	}

	return s.Price.GetYahooIDXHistory(stock.IdxTicker, rangeName)
}

func (s *PriceService) GetIDXStockPrice(ctx context.Context, ticker string) (external.PriceEntry, error) {
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

	entry, sparkline, err := s.Price.GetYahooIDXMarket(stock.IdxTicker)
	if err != nil {
		return external.PriceEntry{}, err
	}
	entry.Sparkline1d = sparkline
	return entry, nil
}
