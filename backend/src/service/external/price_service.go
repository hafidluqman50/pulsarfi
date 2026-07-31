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

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/crypto"
)

type PriceEntry struct {
	Price       float64   `json:"price"`
	Change24h   float64   `json:"change_24h"`
	Currency    string    `json:"currency"`
	Source      string    `json:"source"`
	FetchedAt   time.Time `json:"fetched_at"`
	Sparkline1d []float64 `json:"sparkline_1d,omitempty"`
}

type PriceHistoryPoint struct {
	Timestamp int64   `json:"timestamp"`
	Value     float64 `json:"value"`
}

type yahooChartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				RegularMarketPrice         float64 `json:"regularMarketPrice"`
				RegularMarketChangePercent float64 `json:"regularMarketChangePercent"`
				Currency                   string  `json:"currency"`
			} `json:"meta"`
			Timestamp  []int64 `json:"timestamp"`
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

// GetUSDIDRRate fetches the USD/IDR exchange rate from Yahoo Finance (IDR=X).
// Returns IDR per 1 USD (e.g. 16142).
func (s *PriceService) GetUSDIDRRate() (float64, error) {
	entry, err := s.GetUSDIDR()
	if err != nil {
		return 0, err
	}
	return entry.Price, nil
}

// GetUSDIDR fetches the USD/IDR exchange rate from Yahoo Finance (IDR=X).
func (s *PriceService) GetUSDIDR() (PriceEntry, error) {
	return s.fetchFromYahoo("IDR=X", "IDR")
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

func (s *PriceService) GetIHSGHistory(rangeName string) ([]PriceHistoryPoint, error) {
	rangeParam, interval := yahooRangeParams(rangeName)
	return s.fetchYahooHistory("^JKSE", "IDR", rangeParam, interval)
}

func (s *PriceService) GetYahooIDXHistory(idxTicker string, rangeName string) ([]PriceHistoryPoint, error) {
	rangeParam, interval := yahooRangeParams(rangeName)
	return s.fetchYahooHistory(idxTicker+".JK", "IDRX", rangeParam, interval)
}

// oneWholeStock is 1e18 raw units (PulsarStock has 18 decimals).
var oneWholeStock = new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil)

// quoteStockToIdrxSelector is the 4-byte selector of
// PulsarProtocol.quoteStockToIdrx(string,uint256).
var quoteStockToIdrxSelector = crypto.Keccak256([]byte("quoteStockToIdrx(string,uint256)"))[:4]

// GetOnchainPriceV4 reads the Uniswap V4 pool spot price (IDRX per whole stock) by
// calling PulsarProtocol.quoteStockToIdrx(ticker, 1 whole stock). The contract
// returns raw IDRX (2 decimals) using the same math as the on-chain redeem-fee
// quote, so on-chain and off-chain prices never drift. price = raw / 100.
func (s *PriceService) GetOnchainPriceV4(protocolAddr, ticker, rpcURL string) (PriceEntry, error) {
	cacheKey := "onchainv4:" + strings.ToUpper(ticker)
	s.cacheMu.RLock()
	cached, hit := s.cache[cacheKey]
	s.cacheMu.RUnlock()
	if hit && time.Since(cached.FetchedAt) < s.cacheTTL {
		return cached, nil
	}

	data, err := encodeQuoteStockToIdrx(ticker, oneWholeStock)
	if err != nil {
		return PriceEntry{}, fmt.Errorf("encode quote: %w", err)
	}

	result, err := s.ethCall(rpcURL, protocolAddr, data)
	if err != nil {
		return PriceEntry{}, fmt.Errorf("quoteStockToIdrx: %w", err)
	}

	raw, err := decodeUint256(result)
	if err != nil {
		return PriceEntry{}, fmt.Errorf("decode quote: %w", err)
	}
	if raw.Sign() == 0 {
		return PriceEntry{}, fmt.Errorf("zero pool price for %s", ticker)
	}

	// IDRX has 2 decimals, so raw IDRX per whole stock / 100 is the display price.
	price, _ := new(big.Float).Quo(new(big.Float).SetInt(raw), big.NewFloat(100)).Float64()

	entry := PriceEntry{
		Price:     price,
		Change24h: 0,
		Currency:  "IDRX",
		Source:    "onchain-v4",
		FetchedAt: time.Now(),
	}
	s.cacheMu.Lock()
	s.cache[cacheKey] = entry
	s.cacheMu.Unlock()
	return entry, nil
}

// encodeQuoteStockToIdrx ABI-encodes the calldata for quoteStockToIdrx(string,uint256).
func encodeQuoteStockToIdrx(ticker string, amount *big.Int) (string, error) {
	stringType, err := abi.NewType("string", "", nil)
	if err != nil {
		return "", err
	}
	uint256Type, err := abi.NewType("uint256", "", nil)
	if err != nil {
		return "", err
	}
	arguments := abi.Arguments{{Type: stringType}, {Type: uint256Type}}
	packed, err := arguments.Pack(ticker, amount)
	if err != nil {
		return "", err
	}
	return "0x" + hex.EncodeToString(append(append([]byte{}, quoteStockToIdrxSelector...), packed...)), nil
}

func decodeUint256(result string) (*big.Int, error) {
	trimmed := strings.TrimPrefix(result, "0x")
	if len(trimmed) == 0 {
		return nil, fmt.Errorf("empty result")
	}
	decoded, err := hex.DecodeString(trimmed)
	if err != nil {
		return nil, err
	}
	return new(big.Int).SetBytes(decoded), nil
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

func (s *PriceService) fetchYahooHistory(symbol, currency, rangeParam, interval string) ([]PriceHistoryPoint, error) {
	entry, points, err := s.fetchYahooMarketPoints(symbol, currency, rangeParam, interval)
	if err != nil {
		return nil, err
	}
	if entry.Price > 0 {
		lastIndex := len(points) - 1
		if lastIndex < 0 || points[lastIndex].Value != entry.Price {
			points = append(points, PriceHistoryPoint{
				Timestamp: entry.FetchedAt.UnixMilli(),
				Value:     entry.Price,
			})
		}
	}
	return points, nil
}

func (s *PriceService) fetchYahooMarketPoints(symbol, currency, rangeParam, interval string) (PriceEntry, []PriceHistoryPoint, error) {
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
	return entry, yahooHistoryPoints(result.Timestamp, result.Indicators.Quote), nil
}

func yahooRangeParams(rangeName string) (string, string) {
	switch strings.ToUpper(rangeName) {
	case "1D":
		return "1d", "1m"
	case "1W":
		return "5d", "15m"
	case "3M":
		return "3mo", "1d"
	case "1Y":
		return "1y", "1d"
	default:
		return "1mo", "1d"
	}
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

func yahooHistoryPoints(timestamps []int64, quotes []struct {
	Close []*float64 `json:"close"`
}) []PriceHistoryPoint {
	if len(quotes) == 0 || len(timestamps) == 0 {
		return nil
	}

	closes := quotes[0].Close
	limit := len(timestamps)
	if len(closes) < limit {
		limit = len(closes)
	}

	points := make([]PriceHistoryPoint, 0, limit)
	for i := 0; i < limit; i++ {
		if closes[i] == nil {
			continue
		}
		points = append(points, PriceHistoryPoint{
			Timestamp: timestamps[i] * 1000,
			Value:     *closes[i],
		})
	}
	return points
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
