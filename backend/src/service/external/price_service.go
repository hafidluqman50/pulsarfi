package external

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"
)

type PriceEntry struct {
	Price       float64   `json:"price"`
	Change24h   float64   `json:"change_24h"`
	Currency    string    `json:"currency"`
	Source      string    `json:"source"`
	FetchedAt   time.Time `json:"fetched_at"`
	Sparkline1d []float64 `json:"sparkline_1d,omitempty"`
}

type yahooChartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				RegularMarketPrice         float64 `json:"regularMarketPrice"`
				RegularMarketChangePercent float64 `json:"regularMarketChangePercent"`
				Currency                   string  `json:"currency"`
			} `json:"meta"`
			Indicators struct {
				Quote []struct {
					Close []*float64 `json:"close"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error *struct {
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

type rpcRequest struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  []any  `json:"params"`
	ID      int    `json:"id"`
}

type rpcResponse struct {
	Result string `json:"result"`
	Error  *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type PriceService struct {
	cacheMu  sync.RWMutex
	cache    map[string]PriceEntry
	cacheTTL time.Duration
	client   *http.Client
}

func NewPriceService() *PriceService {
	return &PriceService{
		cache:    map[string]PriceEntry{},
		cacheTTL: 30 * time.Second,
		client:   &http.Client{Timeout: 8 * time.Second},
	}
}

// GetIHSG fetches IDX Composite (^JKSE) from Yahoo Finance.
func (s *PriceService) GetIHSG() (PriceEntry, error) {
	return s.fetchFromYahoo("^JKSE", "IDR")
}

// GetYahooIDX fetches an IDX stock price using its idx_ticker (e.g. "BUMI" → "BUMI.JK").
func (s *PriceService) GetYahooIDX(idxTicker string) (PriceEntry, error) {
	return s.fetchFromYahoo(idxTicker+".JK", "IDRX")
}

func (s *PriceService) GetYahooIDXMarket(idxTicker string) (PriceEntry, []float64, error) {
	return s.fetchYahooMarket(idxTicker+".JK", "IDRX", "1d", "1m")
}

// GetOnchainPrice reads the AMM spot price from the Uniswap V2 pair for stockAddr/IDRX.
func (s *PriceService) GetOnchainPrice(stockAddr, idrxAddr, factoryAddr, rpcURL string) (PriceEntry, error) {
	cacheKey := "onchain:" + strings.ToLower(stockAddr)
	s.cacheMu.RLock()
	cached, hit := s.cache[cacheKey]
	s.cacheMu.RUnlock()
	if hit && time.Since(cached.FetchedAt) < s.cacheTTL {
		return cached, nil
	}

	pairAddr, err := s.getPairAddress(rpcURL, factoryAddr, idrxAddr, stockAddr)
	if err != nil {
		return PriceEntry{}, fmt.Errorf("getPair: %w", err)
	}
	zero := "0x0000000000000000000000000000000000000000"
	if strings.EqualFold(pairAddr, zero) {
		return PriceEntry{}, fmt.Errorf("no liquidity pair for %s", stockAddr)
	}

	reserve0, reserve1, err := s.getReserves(rpcURL, pairAddr)
	if err != nil {
		return PriceEntry{}, fmt.Errorf("getReserves: %w", err)
	}

	var idrxReserve, stockReserve *big.Int
	if strings.ToLower(idrxAddr) < strings.ToLower(stockAddr) {
		idrxReserve, stockReserve = reserve0, reserve1
	} else {
		stockReserve, idrxReserve = reserve0, reserve1
	}

	if stockReserve.Sign() == 0 {
		return PriceEntry{}, fmt.Errorf("zero stock reserve in pair")
	}

	// price (IDRX per stock) = (idrxReserve / 1e2) / (stockReserve / 1e18)
	//                        = idrxReserve * 1e16 / stockReserve
	multiplier := new(big.Int).Exp(big.NewInt(10), big.NewInt(16), nil)
	num := new(big.Int).Mul(idrxReserve, multiplier)
	priceInt := new(big.Int).Div(num, stockReserve)
	price, _ := new(big.Float).SetInt(priceInt).Float64()

	entry := PriceEntry{
		Price:     price,
		Change24h: 0,
		Currency:  "IDRX",
		Source:    "onchain",
		FetchedAt: time.Now(),
	}
	s.cacheMu.Lock()
	s.cache[cacheKey] = entry
	s.cacheMu.Unlock()
	return entry, nil
}

func (s *PriceService) fetchFromYahoo(symbol, currency string) (PriceEntry, error) {
	cacheKey := "yahoo:" + symbol
	s.cacheMu.RLock()
	cached, hit := s.cache[cacheKey]
	s.cacheMu.RUnlock()
	if hit && time.Since(cached.FetchedAt) < s.cacheTTL {
		return cached, nil
	}

	entry, _, err := s.fetchYahooMarket(symbol, currency, "2d", "1d")
	if err != nil {
		return PriceEntry{}, err
	}

	s.cacheMu.Lock()
	s.cache[cacheKey] = entry
	s.cacheMu.Unlock()
	return entry, nil
}

func (s *PriceService) fetchYahooMarket(symbol, currency, rangeParam, interval string) (PriceEntry, []float64, error) {
	url := fmt.Sprintf(
		"https://query1.finance.yahoo.com/v8/finance/chart/%s?interval=%s&range=%s",
		symbol,
		interval,
		rangeParam,
	)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return PriceEntry{}, nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")

	resp, err := s.client.Do(req)
	if err != nil {
		return PriceEntry{}, nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= http.StatusBadRequest {
		return PriceEntry{}, nil, fmt.Errorf("yahoo status: %d", resp.StatusCode)
	}

	var parsed yahooChartResponse
	if err := json.NewDecoder(resp.Body).Decode(&parsed); err != nil {
		return PriceEntry{}, nil, fmt.Errorf("decode: %w", err)
	}
	if parsed.Chart.Error != nil {
		return PriceEntry{}, nil, fmt.Errorf("yahoo error: %s", parsed.Chart.Error.Description)
	}
	if len(parsed.Chart.Result) == 0 {
		return PriceEntry{}, nil, fmt.Errorf("no data for %s", symbol)
	}

	result := parsed.Chart.Result[0]
	meta := result.Meta
	cur := currency
	if cur == "" {
		cur = meta.Currency
	}

	closes := yahooCloses(result.Indicators.Quote)
	change24h := meta.RegularMarketChangePercent
	if change24h == 0 {
		change24h = derivedChangePercent(meta.RegularMarketPrice, closes)
	}

	entry := PriceEntry{
		Price:     meta.RegularMarketPrice,
		Change24h: change24h,
		Currency:  cur,
		Source:    "yahoo",
		FetchedAt: time.Now(),
	}
	return entry, closes, nil
}

func yahooCloses(quotes []struct {
	Close []*float64 `json:"close"`
}) []float64 {
	if len(quotes) == 0 {
		return nil
	}

	closes := make([]float64, 0, len(quotes[0].Close))
	for _, closeValue := range quotes[0].Close {
		if closeValue == nil {
			continue
		}
		closes = append(closes, *closeValue)
	}
	return closes
}

func derivedChangePercent(currentPrice float64, closes []float64) float64 {
	if len(closes) < 2 {
		return 0
	}

	basePrice := closes[0]
	if currentPrice <= 0 {
		currentPrice = closes[len(closes)-1]
	}
	if basePrice <= 0 {
		return 0
	}

	return ((currentPrice - basePrice) / basePrice) * 100
}

func (s *PriceService) getPairAddress(rpcURL, factoryAddr, tokenA, tokenB string) (string, error) {
	data := "0xe6a43905" + padAddr(tokenA) + padAddr(tokenB)
	result, err := s.ethCall(rpcURL, factoryAddr, data)
	if err != nil {
		return "", err
	}
	result = strings.TrimPrefix(result, "0x")
	if len(result) < 40 {
		return "", fmt.Errorf("short getPair result")
	}
	return "0x" + result[len(result)-40:], nil
}

func (s *PriceService) getReserves(rpcURL, pairAddr string) (*big.Int, *big.Int, error) {
	result, err := s.ethCall(rpcURL, pairAddr, "0x0902f1ac")
	if err != nil {
		return nil, nil, err
	}
	h := strings.TrimPrefix(result, "0x")
	if len(h) < 128 {
		return nil, nil, fmt.Errorf("short getReserves result")
	}
	b0, err := hex.DecodeString(h[0:64])
	if err != nil {
		return nil, nil, err
	}
	b1, err := hex.DecodeString(h[64:128])
	if err != nil {
		return nil, nil, err
	}
	return new(big.Int).SetBytes(b0), new(big.Int).SetBytes(b1), nil
}

func (s *PriceService) ethCall(rpcURL, to, data string) (string, error) {
	payload := rpcRequest{
		JSONRPC: "2.0",
		Method:  "eth_call",
		Params:  []any{map[string]string{"to": to, "data": data}, "latest"},
		ID:      1,
	}
	body, _ := json.Marshal(payload)
	resp, err := s.client.Post(rpcURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var rpcResp rpcResponse
	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return "", err
	}
	if rpcResp.Error != nil {
		return "", fmt.Errorf("rpc: %s", rpcResp.Error.Message)
	}
	return rpcResp.Result, nil
}

func padAddr(addr string) string {
	addr = strings.TrimPrefix(strings.ToLower(addr), "0x")
	return fmt.Sprintf("%064s", addr)
}
