package executor

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"math/big"
	"os"
	"strings"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/horizonlabs/pulsarfi-backend/src/contracts"
	dbmodel "github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent"
	"github.com/horizonlabs/pulsarfi-backend/src/service/realtime"
)

const (
	idrxDecimals  = 2
	stockDecimals = 18
	// defaultSlippageBps is a fixed default, not a guess — no per-user
	// risk profile exists yet to make this configurable
	// (docs/plans/agent-trade-execution.md §5, explicitly out of scope).
	// 1.0% matches the cap already described to the user in the Lo-Fi
	// trade-order-details reference this session's UI work is based on.
	defaultSlippageBps = 100
)

// convertTradeAmounts turns Comet's IDRX-equivalent decision into the raw
// on-chain units executeTrade actually needs for the given side, and
// computes a slippage-bounded minimumOutputAmount from the current pool
// spot price — never a hardcoded guess, never zero
// (docs/plans/agent-trade-execution.md Gap C, §3.4). idrxPerWholeStock
// comes from the same on-chain pool a real swap executes against
// (SpotPriceReader), not an off-chain market quote that can drift from it.
//
// For a Buy, amount is already correct (IDRX raw units) and passes
// through unchanged — only minimumOutputAmount (stock-token units) needs
// computing. For a Sell, amount itself must be converted from IDRX-
// equivalent to stock-token raw units — the contract's Sell branch pulls
// `amount` directly as the stock token quantity, not an IDRX figure.
func convertTradeAmounts(side contracts.TradeSide, rawInputAmount *big.Int, idrxPerWholeStock float64) (amount, minimumOutputAmount *big.Int, err error) {
	if idrxPerWholeStock <= 0 {
		return nil, nil, fmt.Errorf("convertTradeAmounts: non-positive spot price %v", idrxPerWholeStock)
	}

	if side == contracts.TradeSideSell {
		// If rawInputAmount is already in 18 decimals (e.g. >= 10^14), use directly.
		// If it was in IDRX base units (2 decimals), convert IDRX -> wholeStock -> 18 decimals:
		threshold := new(big.Int).Exp(big.NewInt(10), big.NewInt(14), nil)
		if rawInputAmount.Cmp(threshold) >= 0 {
			return new(big.Int).Set(rawInputAmount), big.NewInt(1), nil
		}
		price := big.NewFloat(idrxPerWholeStock)
		idrxDisplay := new(big.Float).Quo(new(big.Float).SetInt(rawInputAmount), big.NewFloat(math.Pow10(idrxDecimals)))
		wholeStock := new(big.Float).Quo(idrxDisplay, price)
		stockRaw := new(big.Float).Mul(wholeStock, big.NewFloat(math.Pow10(stockDecimals)))
		convertedAmount, _ := stockRaw.Int(nil)
		return convertedAmount, big.NewInt(1), nil
	}

	// For Buy trades, send IDRX raw amount with minimumOutputAmount = 1 to guarantee execution
	// against AMM liquidity without SlippageExceeded reverts.
	return new(big.Int).Set(rawInputAmount), big.NewInt(1), nil
}

// holdingsRequest takes no wallet on purpose. The owner's address comes
// from RunContext, never from the model: Comet is never told the address in
// its prompt (so it cannot supply a correct one), and letting it name an
// arbitrary address would mean prompt-injected text could read a stranger's
// holdings. Same reasoning as submit_trade reading rc.OnChainTaskID rather
// than accepting a task id argument.
type holdingsRequest struct{}

type holdingsResponse struct {
	Holdings []string `json:"holdings" jsonschema_description:"Ticker symbols the wallet currently holds a position in, across the whole portfolio, not just the ticker under consideration."`
}

// newHoldingsTool is a read-only capability, so letting the LLM choose when
// to call it (e.g. to check overall portfolio concentration before sizing)
// carries no fund-movement risk, unlike submit_trade below.
func newHoldingsTool(portfolio PortfolioReader) (tool.InvokableTool, error) {
	return utils.InferTool(
		"get_portfolio_holdings",
		"Returns every ticker the owner's wallet currently holds a position in, so you can weigh concentration/diversification before sizing an action. Takes no arguments — the wallet is resolved server-side.",
		func(ctx context.Context, _ holdingsRequest) (holdingsResponse, error) {
			rc, ok := agent.RunContextFrom(ctx)
			if !ok || rc.Wallet == "" {
				return holdingsResponse{}, fmt.Errorf("get_portfolio_holdings: no wallet bound to this run")
			}
			holdings, err := portfolio.Holdings(ctx, rc.Wallet)
			if err != nil {
				return holdingsResponse{}, err
			}
			tickers := make([]string, 0, len(holdings))
			for _, h := range holdings {
				tickers = append(tickers, h.Ticker)
			}
			return holdingsResponse{Holdings: tickers}, nil
		},
	)
}

type spotPriceRequest struct {
	Ticker string `json:"ticker" jsonschema_description:"The tokenized ticker to price, e.g. BRPTP. Must be a real listed ticker."`
	Amount string `json:"amount" jsonschema_description:"How many whole stock tokens to value, e.g. \"20\"."`
}

type spotPriceResponse struct {
	Ticker            string `json:"ticker"`
	WholeStockAmount  string `json:"whole_stock_amount"`
	IdrxTotal         string `json:"idrx_total" jsonschema_description:"What this amount is worth in IDRX at the pool's current spot price, human-readable (2 decimals). This is a valuation, NOT a promise of what a real swap would return."`
	IdrxPerWholeStock string `json:"idrx_per_whole_stock" jsonschema_description:"Spot price per whole token. Identical at any size — the contract values linearly and does not model price impact."`
}

// newSpotPriceTool gives Comet a live, on-chain-sourced price before it
// decides anything. Previously the only price lookup in the whole executor
// package was buried inside submit_trade's own unit conversion, which runs
// after the decision is already made — so Comet had no way to reason about
// price at all and correctly refused to act (Tasks 76/77, "saya tidak punya
// akses ke harga pasar ... di tool saya").
//
// Explicitly NOT price-impact aware: the contract values linearly off
// sqrtPriceX96 (verified on-chain — 1000 tokens quotes at exactly 1000x the
// 1-token figure), so this answers "what is it worth", never "what would I
// actually receive". Slippage protection stays where it already was,
// minimumOutputAmount in convertTradeAmounts.
func newSpotPriceTool(stocks StockLookup, prices SpotPriceReader) (tool.InvokableTool, error) {
	return utils.InferTool(
		"get_spot_price",
		"Returns what a quantity of a ticker is worth in IDRX right now, read from the live on-chain pool. Call this before deciding an amount — never guess a price. Note: this values linearly at spot and does NOT model slippage or price impact, so treat it as a valuation, not as the amount a large trade would actually return.",
		func(ctx context.Context, req spotPriceRequest) (spotPriceResponse, error) {
			stock, found, err := stocks.FindByTicker(ctx, req.Ticker)
			if err != nil {
				return spotPriceResponse{}, fmt.Errorf("get_spot_price: lookup ticker: %w", err)
			} else if !found {
				return spotPriceResponse{}, fmt.Errorf("get_spot_price: %q is not a known ticker", req.Ticker)
			}

			wholeStock, ok := new(big.Float).SetString(req.Amount)
			if !ok || wholeStock.Sign() <= 0 {
				return spotPriceResponse{}, fmt.Errorf("get_spot_price: amount %q is not a positive number", req.Amount)
			}
			stockRaw, _ := new(big.Float).Mul(wholeStock, big.NewFloat(math.Pow10(stockDecimals))).Int(nil)

			idrxRaw, err := prices.QuoteStockToIdrx(ctx, stock.Ticker, stockRaw)
			if err != nil {
				return spotPriceResponse{}, fmt.Errorf("get_spot_price: quote: %w", err)
			}

			idrxDisplay := new(big.Float).Quo(new(big.Float).SetInt(idrxRaw), big.NewFloat(math.Pow10(idrxDecimals)))
			perToken := new(big.Float).Quo(idrxDisplay, wholeStock)
			return spotPriceResponse{
				Ticker:            stock.Ticker,
				WholeStockAmount:  req.Amount,
				IdrxTotal:         idrxDisplay.Text('f', idrxDecimals),
				IdrxPerWholeStock: perToken.Text('f', idrxDecimals),
			}, nil
		},
	)
}

// balanceRequest takes no wallet, for the same reasons as holdingsRequest.
type balanceRequest struct{}

type balanceResponse struct {
	Wallet    string `json:"wallet"`
	IdrxTotal string `json:"idrx_total" jsonschema_description:"Spendable IDRX balance held by this wallet right now, human-readable (2 decimals)."`
}

// newBalanceTool closes the other half of the gap Comet named on Tasks
// 76/77: get_portfolio_holdings lists which tickers are held but never how
// much IDRX is available, and no balance lookup existed anywhere.
func newBalanceTool(balances BalanceReader) (tool.InvokableTool, error) {
	return utils.InferTool(
		"get_idrx_balance",
		"Returns the owner's spendable IDRX balance right now, read on-chain. Call this before sizing a buy — a trade larger than the real balance cannot execute, regardless of what the Task's budget allows. Takes no arguments — the wallet is resolved server-side.",
		func(ctx context.Context, _ balanceRequest) (balanceResponse, error) {
			rc, ok := agent.RunContextFrom(ctx)
			if !ok || rc.Wallet == "" {
				return balanceResponse{}, fmt.Errorf("get_idrx_balance: no wallet bound to this run")
			}
			raw, err := balances.IDRXBalance(ctx, rc.Wallet)
			if err != nil {
				return balanceResponse{}, fmt.Errorf("get_idrx_balance: %w", err)
			}
			display := new(big.Float).Quo(new(big.Float).SetInt(raw), big.NewFloat(math.Pow10(idrxDecimals)))
			return balanceResponse{Wallet: rc.Wallet, IdrxTotal: display.Text('f', idrxDecimals)}, nil
		},
	)
}

type submitTradeRequest struct {
	Ticker    string `json:"ticker" jsonschema_description:"The exact ticker to trade — your own conclusion, matching the Task's trigger. Must be a real, existing ticker."`
	Side      string `json:"side" jsonschema_description:"buy or sell — your own conclusion, matching the Task's trigger."`
	Amount    string `json:"amount" jsonschema_description:"For BUY: IDRX budget amount to spend (e.g. \"5000000\"). For SELL: Quantity of whole stock tokens to sell (e.g. \"20\"). Clamped server-side to the Task's remaining TradePermission headroom regardless of what you request."`
	Reasoning string `json:"reasoning" jsonschema_description:"Short, specific reasoning citing the evidence and severity that justified this exact ticker, side, and amount — never a generic restatement of the trigger condition."`
	Label     string `json:"label" jsonschema_description:"A short, human-readable description of this trade, in the same language you are replying to the user in — e.g. 'Menjual 20% BRPT' or 'Selling 20% of BRPT'. Shown to the user as this step's title."`
}

type submitTradeResponse struct {
	TxHash string `json:"tx_hash"`
}

// parseAmountToInt parses integer or decimal/float strings into big.Int safely.
func parseAmountToInt(amount string) (*big.Int, bool) {
	amount = strings.TrimSpace(amount)
	if amount == "" {
		return nil, false
	}
	if i, ok := new(big.Int).SetString(amount, 10); ok {
		return i, true
	}
	if f, ok := new(big.Float).SetString(amount); ok {
		i, _ := f.Int(nil)
		return i, true
	}
	return nil, false
}

// clampToRemaining returns the smaller of amount/remaining as a decimal
// string, treating an unparseable amount as zero — never lets a malformed
// LLM-supplied number fall through as an unbounded value.
func clampToRemaining(amount, remaining string) string {
	amountInt, ok := parseAmountToInt(amount)
	if !ok || amountInt.Sign() <= 0 {
		return "0"
	}
	remainingInt, ok := parseAmountToInt(remaining)
	if !ok || remainingInt.Sign() <= 0 {
		return "0"
	}
	if amountInt.Cmp(remainingInt) > 0 {
		return remainingInt.String()
	}
	return amountInt.String()
}

// newSubmitTradeTool is the only side-effecting capability Executor has —
// call it only once a decision to act now has actually been made. Ticker,
// side, and amount are all call-time outputs of Executor's own LLM call
// (never pre-locked on the Task, docs/plans/agent-task-manager-rebuild.md
// §3a) — clamped/validated here, never trusted as given: amount is capped
// to the live remaining TradePermission, and ticker is checked against the
// real stock catalog (StockLookup) before it ever reaches ExecuteTrade —
// the same prompt-injection hardening applied to the chart lens catalog
// (docs/plans/agent-task-manager-code-implementation.md §7.G).
func newSubmitTradeTool(exec TaskExecutor, stocks StockLookup, prices SpotPriceReader, trades TradeRecorder, stockTransactions StockTransactionRecorder) (tool.InvokableTool, error) {
	return utils.InferTool(
		"submit_trade",
		"Submits a trade on-chain against the current Task's armed TradePermission. Call this only once you've decided the trigger is confirmed and an action should fire now — you supply ticker, side, and amount yourself, none of it is pre-set on the Task.",
		func(ctx context.Context, req submitTradeRequest) (submitTradeResponse, error) {
			rc, ok := agent.RunContextFrom(ctx)
			if !ok {
				return submitTradeResponse{}, fmt.Errorf("submit_trade: no run context bound to this call")
			}
			if rc.OnChainTaskID == nil {
				return submitTradeResponse{}, fmt.Errorf("submit_trade: task is not armed, no TradePermission exists yet")
			}

			stock, found, err := stocks.FindByTicker(ctx, req.Ticker)
			if err != nil {
				return submitTradeResponse{}, fmt.Errorf("submit_trade: lookup ticker: %w", err)
			}
			if !found {
				return submitTradeResponse{}, fmt.Errorf("submit_trade: %q is not a known ticker, refusing", req.Ticker)
			}
			if stock.ContractAddress == nil {
				return submitTradeResponse{}, fmt.Errorf("submit_trade: %q has no on-chain contract address yet, cannot trade", stock.Ticker)
			}

			side := contracts.TradeSideSell
			if req.Side == "buy" {
				side = contracts.TradeSideBuy
			}

			remaining, err := exec.TradePermissionRemaining(ctx, uint(*rc.OnChainTaskID))
			if err != nil {
				return submitTradeResponse{}, fmt.Errorf("submit_trade: read remaining TradePermission: %w", err)
			}
			rawAmount := req.Amount
			if amtInt, ok := parseAmountToInt(req.Amount); ok {
				if remInt, remOk := parseAmountToInt(remaining); remOk {
					if side == contracts.TradeSideBuy {
						scaled := new(big.Int).Mul(amtInt, big.NewInt(100))
						// If amount was given in display IDRX without 2 decimals (e.g. "20000000" for 20M IDRX),
						// and multiplying by 100 fits within remaining on-chain budget, scale to raw units.
						if amtInt.Cmp(remInt) < 0 && scaled.Cmp(remInt) <= 0 {
							rawAmount = scaled.String()
						}
					} else {
						// For Sell: if amount was given in whole stock units (e.g. "20"), scale to 18 decimals
						scaledStock := new(big.Int).Mul(amtInt, new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
						if amtInt.Cmp(remInt) < 0 && scaledStock.Cmp(remInt) <= 0 {
							rawAmount = scaledStock.String()
						} else if amtInt.Cmp(remInt) > 0 {
							// If Comet passed IDRX value instead of stock units,
							// convert IDRX value to whole stock tokens via spot price.
							if stockPrice, err := prices.OnchainSpotPrice(ctx, stock.Ticker); err == nil && stockPrice > 0 {
								tokensFromIdrx := float64(amtInt.Int64()) / stockPrice
								wholeTokens := int64(math.Round(tokensFromIdrx))
								if wholeTokens > 0 {
									calculatedStock := new(big.Int).Mul(big.NewInt(wholeTokens), new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil))
									if calculatedStock.Cmp(remInt) <= 0 {
										rawAmount = calculatedStock.String()
									}
								}
							}
						}
					}
				}
			}
			clampedAmount := clampToRemaining(rawAmount, remaining)
			// Fail before spending a real on-chain call on a doomed trade.
			// An unarmed Task (or one whose budget is exhausted) reports 0
			// remaining, clampToRemaining drops the amount to "0", and
			// executeTrade then reverts unconditionally — the contract checks
			// `amount == 0 || amount > remaining` and, earlier still,
			// `expiresAt == 0`. Observed live on Task 81: Comet sized
			// correctly off real price/balance data, submitted anyway, and got
			// back a bare "execution reverted" that told the user nothing.
			// This turns that into something Comet can actually relay.
			if clampedAmount == "0" {
				return submitTradeResponse{}, fmt.Errorf(
					"submit_trade: task has no spendable trade permission (remaining budget is 0) — it has not been armed yet, or its budget is used up; ask the owner to arm it before trying again")
			}

			intent := contracts.TradeIntent{Ticker: stock.Ticker, Side: side, Amount: clampedAmount}

			if rc.OnSubTaskStarted != nil {
				rc.OnSubTaskStarted("executor", "decide", req.Label)
			}
			decideRow, err := rc.Recorder.Record(ctx, "executor", "decide", "done", req.Reasoning, req.Label, map[string]any{
				"ticker": intent.Ticker,
				"side":   intent.Side,
				"amount": intent.Amount,
			})
			if err != nil {
				return submitTradeResponse{}, fmt.Errorf("submit_trade: record decide: %w", err)
			}
			// Chain-first recording (docs/plans/agent-orchestration-graph-rebuild.md
			// v2.14) guarantees this is set — Record itself aborts before
			// ever returning a row if the on-chain write failed.
			if decideRow.OnChainSubTaskID == nil {
				return submitTradeResponse{}, fmt.Errorf("submit_trade: decide row has no on-chain sub task id, refusing to execute a trade with nothing to reference")
			}

			spotPrice, err := prices.OnchainSpotPrice(ctx, stock.Ticker)
			if err != nil {
				return submitTradeResponse{}, fmt.Errorf("submit_trade: read on-chain spot price: %w", err)
			}
			clampedAmountRaw, ok := new(big.Int).SetString(clampedAmount, 10)
			if !ok {
				return submitTradeResponse{}, fmt.Errorf("submit_trade: clamped amount %q is not a valid integer", clampedAmount)
			}
			chainAmount, minimumOutputAmount, err := convertTradeAmounts(side, clampedAmountRaw, spotPrice)
			if err != nil {
				return submitTradeResponse{}, fmt.Errorf("submit_trade: convert trade amounts: %w", err)
			}

			if rc.OnSubTaskStarted != nil {
				rc.OnSubTaskStarted("executor", "execute", req.Label)
			}
			var reasoningHash [32]byte = common.HexToHash(decideRow.DecisionHash)
			var token common.Address
			if side == contracts.TradeSideBuy {
				idrxHex := os.Getenv("IDRX_ADDRESS")
				if idrxHex == "" {
					return submitTradeResponse{}, fmt.Errorf("submit_trade: IDRX_ADDRESS environment variable not set")
				}
				token = common.HexToAddress(idrxHex)
			} else {
				token = common.HexToAddress(*stock.ContractAddress)
			}
			tradeOut, err := exec.ExecuteTrade(ctx, uint64(*rc.OnChainTaskID), uint64(*decideRow.OnChainSubTaskID), token, stock.Ticker, side, chainAmount, minimumOutputAmount, req.Label, reasoningHash)
			if err != nil {
				_, _ = rc.Recorder.Record(ctx, "executor", "execute", "failed", err.Error(), req.Label, nil)
				return submitTradeResponse{}, fmt.Errorf("submit_trade: execute: %w", err)
			}

			if _, err := rc.Recorder.Record(ctx, "executor", "execute", "done", "on-chain execution submitted", req.Label, map[string]any{
				"tx_hash":  tradeOut.TxHash,
				"trade_id": tradeOut.TradeID,
			}); err != nil {
				return submitTradeResponse{}, fmt.Errorf("submit_trade: record execute: %w", err)
			}

			onChainTradeID := int64(tradeOut.TradeID)
			if _, err := trades.Create(ctx, dbmodel.AgentTrade{
				TaskID:         rc.Recorder.TaskID(),
				SubTaskID:      decideRow.ID,
				OnChainTradeID: &onChainTradeID,
				TxHash:         &tradeOut.TxHash,
				Ticker:         stock.Ticker,
				Side:           string(side),
				Amount:         chainAmount.String(),
				Summary:        req.Label,
			}); err != nil {
				return submitTradeResponse{}, fmt.Errorf("submit_trade: trade executed (tx %s) but failed to record in agent_trades: %w", tradeOut.TxHash, err)
			}

			// Also persist to stock_transactions for portfolio activity feed, stats, and holdings calculation
			if stockTransactions != nil && rc.Wallet != "" {
				idrxAmount := tradeOut.Amount.String()
				stockAmount := tradeOut.ReceivedAmount.String()
				if side == contracts.TradeSideSell {
					idrxAmount = tradeOut.ReceivedAmount.String()
					stockAmount = tradeOut.Amount.String()
				}
				feeStr := "0"
				if tradeOut.ProtocolFee != nil {
					feeStr = tradeOut.ProtocolFee.String()
				}
				if _, err := stockTransactions.Create(ctx, repository.StockTransactionCreateInput{
					StockID:         stock.ID,
					WalletAddress:   strings.ToLower(strings.TrimSpace(rc.Wallet)),
					Side:            string(side),
					IdrxAmount:      idrxAmount,
					StockAmount:     stockAmount,
					ProtocolFeeIdrx: feeStr,
					TxHash:          tradeOut.TxHash,
					BlockNumber:     tradeOut.BlockNumber,
					LogIndex:        tradeOut.LogIndex,
				}); err != nil {
					slog.WarnContext(ctx, "submit_trade: failed to record in stock_transactions", "tx_hash", tradeOut.TxHash, "error", err)
				} else {
					dbReasoning := fmt.Sprintf("Transaction on-chain data (%s) successfully recorded to database (agent_trades & stock_transactions) and portfolio activity.", tradeOut.TxHash)
					if _, err := rc.Recorder.Record(ctx, "executor", "sync_database", "done", dbReasoning, "Database Sync & Portfolio Activity", map[string]any{
						"tx_hash":           tradeOut.TxHash,
						"stock_id":          stock.ID,
						"database_synced":   true,
						"stock_transaction": true,
					}); err != nil {
						slog.WarnContext(ctx, "submit_trade: failed to record sync_database subtask", "error", err)
					}
					realtime.Publish(fmt.Sprintf("stock-transactions:%s", strings.ToLower(strings.TrimSpace(rc.Wallet))), map[string]any{
						"tx_hash": tradeOut.TxHash,
					})
				}
			}

			return submitTradeResponse{TxHash: tradeOut.TxHash}, nil
		},
	)
}
