package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ethereum/go-ethereum/ethclient"
)

type EthClient struct {
	client *ethclient.Client
}

func NewEthClient(ctx context.Context, rpcURL string) (*EthClient, error) {
	client, err := ethclient.DialContext(ctx, rpcURL)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", rpcURL, err)
	}
	return &EthClient{client: client}, nil
}

func (e *EthClient) Close() {
	e.client.Close()
}

func (e *EthClient) Client() *ethclient.Client {
	return e.client
}

type MintRequest struct {
	Ticker string `json:"ticker"`
	Amount string `json:"amount"`
	To     string `json:"to"`
}

type BurnRequest struct {
	Ticker string `json:"ticker"`
	Amount string `json:"amount"`
	From   string `json:"from"`
}

func HandleMint(eth *EthClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req MintRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		// TODO: call pStock ERC-20 mint() via go-ethereum
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":  "queued",
			"ticker":  "p" + req.Ticker,
			"amount":  req.Amount,
			"to":      req.To,
			"message": "attestation signed (3/5 multisig) — awaiting on-chain confirmation",
		})
	}
}

func HandleBurn(eth *EthClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req BurnRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		// TODO: call pStock ERC-20 burn() via go-ethereum
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{
			"status":  "queued",
			"ticker":  "p" + req.Ticker,
			"amount":  req.Amount,
			"from":    req.From,
			"message": "IDR settlement initiated — T+2 schedule",
		})
	}
}

func HandleReserves(eth *EthClient) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// TODO: read on-chain supply from ERC-20 totalSupply()
		reserves := []map[string]any{
			{"ticker": "pBUMI", "custody": 18_240_000, "onchain": 18_240_000, "ratio": "1.0000"},
			{"ticker": "pENRG", "custody": 12_400_000, "onchain": 12_400_000, "ratio": "1.0000"},
			{"ticker": "pKIJA", "custody": 9_120_000,  "onchain": 9_120_000,  "ratio": "1.0000"},
			{"ticker": "pTLKM", "custody": 6_800_000,  "onchain": 6_800_000,  "ratio": "1.0000"},
			{"ticker": "pBBRI", "custody": 5_240_000,  "onchain": 5_240_000,  "ratio": "1.0000"},
			{"ticker": "pGOTO", "custody": 28_900_001, "onchain": 28_900_000, "ratio": "1.0000"},
			{"ticker": "pASII", "custody": 4_120_000,  "onchain": 4_120_000,  "ratio": "1.0000"},
			{"ticker": "pUNVR", "custody": 3_840_000,  "onchain": 3_840_000,  "ratio": "1.0000"},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"reserves": reserves, "attestedAt": "2026-05-23T14:02:12Z"})
	}
}
