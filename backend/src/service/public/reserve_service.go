package public

import (
	"context"
	"fmt"
	"strconv"

	"github.com/horizonlabs/pulsarfi-backend/src/repository"
)

type ReserveService struct {
	Attestations *repository.StockAttestationRepository
}

type ReserveStockResponse struct {
	ID              int64   `json:"id"`
	Ticker          string  `json:"ticker"`
	StockName       string  `json:"stock_name"`
	IdxTicker       string  `json:"idx_ticker"`
	Sector          *string `json:"sector"`
	ContractAddress *string `json:"contract_address"`
}

type ReserveEntryResponse struct {
	Stock             ReserveStockResponse `json:"stock"`
	CustodianHoldings string               `json:"custodian_holdings"`
	OnChainSupply     string               `json:"on_chain_supply"`
	PegRatio          string               `json:"peg_ratio"`
	PegStatus         string               `json:"peg_status"`
	LastAttestedAt    any                  `json:"last_attested_at"`
	AttestationHash   *string              `json:"attestation_hash"`
}

func (s *ReserveService) GetReserves(ctx context.Context) ([]ReserveEntryResponse, error) {
	attestations, err := s.Attestations.FindLatestPerStock(ctx)
	if err != nil {
		return nil, err
	}

	entries := make([]ReserveEntryResponse, 0, len(attestations))
	for _, a := range attestations {
		pegRatio, pegStatus := reservePeg(a.CustodianHoldings, a.OnChainSupply)
		entries = append(entries, ReserveEntryResponse{
			Stock: ReserveStockResponse{
				ID:              a.Stock.ID,
				Ticker:          a.Stock.Ticker,
				StockName:       a.Stock.StockName,
				IdxTicker:       a.Stock.IdxTicker,
				Sector:          a.Stock.Sector,
				ContractAddress: a.Stock.ContractAddress,
			},
			CustodianHoldings: a.CustodianHoldings,
			OnChainSupply:     a.OnChainSupply,
			PegRatio:          pegRatio,
			PegStatus:         pegStatus,
			LastAttestedAt:    a.AttestedAt,
			AttestationHash:   a.AttestationHash,
		})
	}

	return entries, nil
}

func reservePeg(custodianHoldings string, onChainSupply string) (string, string) {
	holdings, errH := strconv.ParseFloat(custodianHoldings, 64)
	supply, errS := strconv.ParseFloat(onChainSupply, 64)

	if errH != nil || errS != nil || supply <= 0 {
		return "N/A", "unknown"
	}

	ratio := holdings / supply
	if ratio >= 1.0 {
		return fmt.Sprintf("%.4f", ratio), "pegged"
	}
	return fmt.Sprintf("%.4f", ratio), "depegged"
}
