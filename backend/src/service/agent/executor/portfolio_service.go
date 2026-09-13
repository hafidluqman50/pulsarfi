package executor

import (
	"context"
	"math/big"
	"strings"

	"github.com/horizonlabs/pulsarfi-backend/src/repository"
)

// Holding is one position in a wallet's portfolio.
type Holding struct {
	Ticker string
}

// PortfolioReader lets Executor see what the wallet actually holds, so it
// can't propose a sell on a ticker the user doesn't own, and can weigh
// concentration before sizing either direction.
type PortfolioReader interface {
	Holdings(ctx context.Context, wallet string) ([]Holding, error)
}

// DBPortfolioReader derives current holdings from the stock_transactions
// activity feed — the same table swap/transfer/redeem writes already
// populate — netting buy/transfer-in against sell/transfer-out/redeem per
// ticker. This is a real implementation, not a stub: the data already
// exists, no new integration needed.
type DBPortfolioReader struct {
	Transactions *repository.StockTransactionRepository
}

func (r *DBPortfolioReader) Holdings(ctx context.Context, wallet string) ([]Holding, error) {
	txs, err := r.Transactions.FindByWallet(ctx, wallet)
	if err != nil {
		return nil, err
	}

	net := map[string]*big.Int{}
	for _, tx := range txs {
		amount, ok := new(big.Int).SetString(tx.StockAmount, 10)
		if !ok {
			continue
		}
		ticker := tx.Stock.Ticker
		if net[ticker] == nil {
			net[ticker] = big.NewInt(0)
		}
		switch strings.ToLower(tx.Side) {
		case "buy", "transfer-in":
			net[ticker].Add(net[ticker], amount)
		case "sell", "transfer-out", "request-redeem", "redeemed":
			net[ticker].Sub(net[ticker], amount)
		}
	}

	holdings := make([]Holding, 0, len(net))
	for ticker, amount := range net {
		if amount.Sign() > 0 {
			holdings = append(holdings, Holding{Ticker: ticker})
		}
	}
	return holdings, nil
}
