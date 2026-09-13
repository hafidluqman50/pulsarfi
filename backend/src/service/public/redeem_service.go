package public

import (
	"context"
	"errors"
	"math/big"
	"strings"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
)

var ErrWalletMismatch = errors.New("authenticated wallet does not match the submitted user_address")
var ErrVerifierUnavailable = errors.New("on-chain verifier is not configured")

// RedeemVerifier re-derives redeem facts from the real RedeemRequested event
// via a single tx_hash lookup — no polling, no indexing — so a valid-but-
// dishonest caller can't submit fabricated amounts under their own wallet.
type RedeemVerifier interface {
	VerifyRedeemRequested(ctx context.Context, txHash string) (*external.RedeemRequestedEvent, error)
}

type PublicRedeemService struct {
	Stocks            *repository.StockRepository
	RedeemProposals   *repository.RedeemProposalRepository
	StockTransactions *repository.StockTransactionRepository
	Verifier          RedeemVerifier
}

type RecordRedeemRequest struct {
	OnChainID           int64
	Ticker              string
	TokenAmount         string
	FeeIdrx             string
	UserAddress         string
	AttestationHash     string
	TxHash              string
	BlockNumber         int64
	AuthenticatedWallet string
}

type RedeemProposalResponse struct {
	ID          int64   `json:"id"`
	OnChainID   int64   `json:"on_chain_id"`
	Ticker      string  `json:"ticker"`
	TokenAmount string  `json:"token_amount"`
	FeeIdrx     string  `json:"fee_idrx"`
	UserAddress string  `json:"user_address"`
	Status      string  `json:"status"`
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

	// 1. The token proves who is calling — blocks submitting a redeem
	// record under someone else's wallet.
	if !strings.EqualFold(req.AuthenticatedWallet, req.UserAddress) {
		return model.RedeemProposal{}, false, ErrWalletMismatch
	}

	// 2. tx_hash proves what actually happened — blocks a valid caller from
	// fabricating amounts under their own wallet. One RPC call, no indexing.
	if s.Verifier == nil {
		return model.RedeemProposal{}, false, ErrVerifierUnavailable
	}
	event, err := s.Verifier.VerifyRedeemRequested(ctx, req.TxHash)
	if err != nil {
		return model.RedeemProposal{}, false, ErrOnChainMismatch
	}
	if event.RequestID.Cmp(big.NewInt(req.OnChainID)) != 0 {
		return model.RedeemProposal{}, false, ErrOnChainMismatch
	}
	if !strings.EqualFold(event.User.Hex(), req.UserAddress) {
		return model.RedeemProposal{}, false, ErrOnChainMismatch
	}
	if !strings.EqualFold(event.Ticker, stock.Ticker) {
		return model.RedeemProposal{}, false, ErrOnChainMismatch
	}

	userAddress := strings.ToLower(req.UserAddress)
	tokenAmount := event.TokenAmount.String()
	feeIdrx := event.FeeIdrx.String()

	txHash := req.TxHash
	proposal, err := s.RedeemProposals.Create(ctx, repository.RedeemProposalCreateInput{
		OnChainID:       req.OnChainID,
		StockID:         stock.ID,
		TokenAmount:     tokenAmount,
		FeeIdrx:         feeIdrx,
		UserAddress:     userAddress,
		AttestationHash: req.AttestationHash,
		RequestTxHash:   &txHash,
	})
	if err != nil {
		return model.RedeemProposal{}, false, err
	}

	// Record in stock_transactions for portfolio activity feed
	s.StockTransactions.Create(ctx, repository.StockTransactionCreateInput{
		StockID:       stock.ID,
		WalletAddress: userAddress,
		Side:          "request-redeem",
		IdrxAmount:    feeIdrx,
		StockAmount:   tokenAmount,
		TxHash:        req.TxHash,
		BlockNumber:   req.BlockNumber,
	})

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
