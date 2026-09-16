package analyzer

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	publicsvc "github.com/horizonlabs/pulsarfi-backend/src/service/public"
)

type verifyTickerRequest struct {
	Ticker string `json:"ticker" jsonschema_description:"The stock ticker to verify, e.g. BUMI, BUMIP, BBCA, BBCAP."`
}

type TickerVerificationResult struct {
	Ticker          string  `json:"ticker"`
	Found           bool    `json:"found"`
	Tokenized       bool    `json:"tokenized"`
	ContractAddress string  `json:"contract_address,omitempty"`
	Price           float64 `json:"price,omitempty"`
	Currency        string  `json:"currency,omitempty"`
	Change24h       float64 `json:"change_24h,omitempty"`
	Source          string  `json:"source,omitempty"`
	Note            string  `json:"note"`
}

func NewVerifyTickerTool(priceSvc *publicsvc.PriceService) (tool.InvokableTool, error) {
	return newVerifyTickerTool(priceSvc)
}

func newVerifyTickerTool(priceSvc *publicsvc.PriceService) (tool.InvokableTool, error) {
	return utils.InferTool(
		"verify_ticker",
		"Verifies whether a stock ticker is tokenized and tradeable on PulsarFi, checks its live on-chain pool price and 24h change, or checks if it exists only as an untokenized IDX stock on Yahoo. Use this for all stock ticker validation and price checks before trading.",
		func(ctx context.Context, req verifyTickerRequest) (TickerVerificationResult, error) {
			rawTicker := strings.ToUpper(strings.TrimSpace(req.Ticker))
			if rawTicker == "" {
				return TickerVerificationResult{
					Ticker: req.Ticker,
					Found:  false,
					Note:   "Ticker is empty.",
				}, nil
			}

			if priceSvc == nil {
				return TickerVerificationResult{
					Ticker: rawTicker,
					Found:  false,
					Note:   "Price service unavailable.",
				}, nil
			}

			entry, err := priceSvc.GetStockPrice(ctx, rawTicker, "")
			if err == nil {
				var contractAddr string
				tokenized := false
				if priceSvc.Stocks != nil {
					if stock, found, sErr := priceSvc.Stocks.FindByTickerOrIdxTicker(ctx, rawTicker); sErr == nil && found {
						if stock.ContractAddress != nil && *stock.ContractAddress != "" {
							contractAddr = *stock.ContractAddress
							tokenized = true
						}
						rawTicker = stock.Ticker
					}
				}
				if entry.Source == "onchain-v4" || tokenized {
					return TickerVerificationResult{
						Ticker:          rawTicker,
						Found:           true,
						Tokenized:       true,
						ContractAddress: contractAddr,
						Price:           entry.Price,
						Currency:        entry.Currency,
						Change24h:       entry.Change24h,
						Source:          entry.Source,
						Note:            fmt.Sprintf("%s is tokenized and tradeable on PulsarFi.", rawTicker),
					}, nil
				}
			}

			if errors.Is(err, publicsvc.ErrStockNotFound) || err != nil {
				// Check if it exists on Yahoo/IDX as an untokenized stock
				idxTicker := rawTicker
				if strings.HasSuffix(idxTicker, "P") && len(idxTicker) > 1 {
					idxTicker = strings.TrimSuffix(idxTicker, "P")
				}
				if priceSvc.Price != nil {
					yahoo, yErr := priceSvc.Price.GetYahooIDX(idxTicker)
					if yErr == nil && yahoo.Price > 0 {
						return TickerVerificationResult{
							Ticker:          idxTicker,
							Found:           true,
							Tokenized:       false,
							ContractAddress: "",
							Price:           yahoo.Price,
							Currency:        yahoo.Currency,
							Change24h:       yahoo.Change24h,
							Source:          "yahoo",
							Note:            fmt.Sprintf("%s exists on IDX (Yahoo Finance) but is NOT tokenized on PulsarFi. It cannot be traded on-chain.", idxTicker),
						}, nil
					}
				}

				return TickerVerificationResult{
					Ticker:    rawTicker,
					Found:     false,
					Tokenized: false,
					Note:      fmt.Sprintf("%s was not found on PulsarFi or IDX/Yahoo.", rawTicker),
				}, nil
			}

			return TickerVerificationResult{
				Ticker:    rawTicker,
				Found:     false,
				Tokenized: false,
				Note:      fmt.Sprintf("Unable to verify ticker %s.", rawTicker),
			}, nil
		},
	)
}
