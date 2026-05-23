package oracle

import (
	"encoding/json"
	"net/http"
	"time"
)

type PriceEntry struct {
	Ticker    string  `json:"ticker"`
	Price     float64 `json:"price"`
	Change24h float64 `json:"change24h"`
	UpdatedAt string  `json:"updatedAt"`
}

type Oracle struct {
	prices []PriceEntry
}

func New() *Oracle {
	now := time.Now().UTC().Format(time.RFC3339)
	return &Oracle{
		prices: []PriceEntry{
			{Ticker: "pBUMI", Price: 0.0152, Change24h: +4.21, UpdatedAt: now},
			{Ticker: "pENRG", Price: 0.0234, Change24h: +1.84, UpdatedAt: now},
			{Ticker: "pKIJA", Price: 0.0089, Change24h: -0.61, UpdatedAt: now},
			{Ticker: "pTLKM", Price: 0.1842, Change24h: +0.42, UpdatedAt: now},
			{Ticker: "pBBRI", Price: 0.2961, Change24h: -1.12, UpdatedAt: now},
			{Ticker: "pGOTO", Price: 0.0061, Change24h: +7.93, UpdatedAt: now},
			{Ticker: "pASII", Price: 0.3214, Change24h: +0.18, UpdatedAt: now},
			{Ticker: "pUNVR", Price: 0.1487, Change24h: -0.34, UpdatedAt: now},
		},
	}
}

func (o *Oracle) HandlePrices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"prices":    o.prices,
		"source":    "horizon-oracle-v1",
		"staleness": "2.1s",
	})
}
