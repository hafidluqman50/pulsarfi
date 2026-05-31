package public

import (
	"context"
	"strings"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
)

type PublicRedeemService struct {
	Stocks          *repository.StockRepository
	RedeemProposals *repository.RedeemProposalRepository
}

type RecordRedeemRequest struct {
	OnChainID   int64
	Ticker      string
	TokenAmount string
	FeeIdrx     string
	UserAddress string
	TxHash      string
}

type RedeemProposalResponse struct {
	ID          int64  `json:"id"`
	OnChainID   int64  `json:"on_chain_id"`
	Ticker      string `json:"ticker"`
	TokenAmount string `json:"token_amount"`
	FeeIdrx     string `json:"fee_idrx"`
	UserAddress string `json:"user_address"`
	Status      string `json:"status"`
	TxHash      *string `json:"tx_hash"`
}

func (s *PublicRedeemService) Record(ctx context.Context, req RecordRedeemRequest) (model.RedeemProposal, bool, error) {
	existing, found, err := s.RedeemProposals.FindByOnChainID(ctx, req.OnChainID)
	if err != nil {
		return model.RedeemProposal{}, false, err
	}
	if found {
		return existing, false, nil
	}

	stock, found, err := s.Stocks.FindByTicker(ctx, req.Ticker)
	if err != nil {
		return model.RedeemProposal{}, false, err
	}
	if !found {
		stock, found, err = s.Stocks.FindByTickerOrIdxTicker(ctx, req.Ticker)
		if err != nil || !found {
			return model.RedeemProposal{}, false, ErrStockNotFound
		}
	}

	feeIdrx := strings.TrimSpace(req.FeeIdrx)
	if feeIdrx == "" {
		feeIdrx = "0"
	}

	txHash := req.TxHash
	proposal, err := s.RedeemProposals.Create(ctx, repository.RedeemProposalCreateInput{
		OnChainID:     req.OnChainID,
		StockID:       stock.ID,
		TokenAmount:   req.TokenAmount,
		FeeIdrx:       feeIdrx,
		UserAddress:   strings.ToLower(req.UserAddress),
		RequestTxHash: &txHash,
	})
	if err != nil {
		return model.RedeemProposal{}, false, err
	}

	return proposal, true, nil
}

func (s *PublicRedeemService) ListByUser(ctx context.Context, userAddress string) ([]model.RedeemProposal, error) {
	all, err := s.RedeemProposals.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	lower := strings.ToLower(userAddress)
	filtered := make([]model.RedeemProposal, 0)
	for _, p := range all {
		if strings.ToLower(p.UserAddress) == lower {
			filtered = append(filtered, p)
		}
	}
	return filtered, nil
}
