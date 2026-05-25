package custodian

import (
	"context"
	"errors"
	"fmt"
	"math/big"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
)

var ErrCustodianNotFound = errors.New("custodian not found")

type CustodianService struct {
	Repos  *repository.Registry
	Stream *external.StreamService
	Price  *external.PriceService
}

type StatsResponse struct {
	AssetsUnderCustodyIDR string `json:"assets_under_custody_idr"`
	MintVolume24hIDRX     string `json:"mint_volume_24h_idrx"`
	MintCount24h          int64  `json:"mint_count_24h"`
	BurnCount24h          int64  `json:"burn_count_24h"`
	PendingRequests       struct {
		Total   int `json:"total"`
		Mints   int `json:"mints"`
		Redeems int `json:"redeems"`
	} `json:"pending_requests"`
}

type AttestorInfo struct {
	Name          string `json:"name"`
	WalletAddress string `json:"wallet_address"`
	Type          string `json:"type"`
	TxHash        string `json:"tx_hash,omitempty"`
	AttestedAt    any    `json:"attested_at"`
}

type PendingRequestResponse struct {
	ID               int64          `json:"id"`
	OnChainID        int64          `json:"on_chain_id"`
	Kind             string         `json:"kind"`
	Ticker           string         `json:"ticker"`
	StockName        string         `json:"stock_name"`
	IdxTicker        string         `json:"idx_ticker"`
	TokenAmount      string         `json:"token_amount"`
	IdrxAmount       *string        `json:"idrx_amount,omitempty"`
	FeeIdrx          *string        `json:"fee_idrx,omitempty"`
	UserAddress      *string        `json:"user_address,omitempty"`
	RequesterAddress *string        `json:"requester_address,omitempty"`
	Source           string         `json:"source"`
	Destination      *string        `json:"destination,omitempty"`
	ApprovalCount    int64          `json:"approval_count"`
	RejectCount      int64          `json:"reject_count"`
	Attestors        []AttestorInfo `json:"attestors"`
	Status           string         `json:"status"`
	RequestTxHash    *string        `json:"request_tx_hash,omitempty"`
	CreatedAt        any            `json:"created_at"`
}

type PendingRequestsResponse struct {
	Mints   []model.MintProposal     `json:"mints"`
	Redeems []model.RedeemProposal   `json:"redeems"`
	Items   []PendingRequestResponse `json:"items"`
}

type SubmitAttestationRequest struct {
	StockID           int64   `json:"stock_id" binding:"required"`
	CustodianHoldings string  `json:"custodian_holdings" binding:"required"`
	OnChainSupply     string  `json:"on_chain_supply" binding:"required"`
	AttestationHash   *string `json:"attestation_hash"`
}

func (s *CustodianService) GetStats(ctx context.Context) (StatsResponse, error) {
	var res StatsResponse

	assetsUnderCustodyIDR, err := s.assetsUnderCustodyIDR(ctx)
	if err != nil {
		return res, err
	}
	mintCount24h, err := s.Repos.MintProposal.CountExecutedLast24h(ctx)
	if err != nil {
		return res, err
	}
	mintVolume24hIDRX, err := s.Repos.MintProposal.SumIdrxLast24h(ctx)
	if err != nil {
		return res, err
	}
	burnCount24h, err := s.Repos.RedeemProposal.CountExecutedLast24h(ctx)
	if err != nil {
		return res, err
	}
	pending, err := s.ListPendingRequests(ctx)
	if err != nil {
		return res, err
	}

	res.AssetsUnderCustodyIDR = assetsUnderCustodyIDR
	res.MintVolume24hIDRX = mintVolume24hIDRX
	res.MintCount24h = mintCount24h
	res.BurnCount24h = burnCount24h
	res.PendingRequests.Total = len(pending.Items)
	res.PendingRequests.Mints = len(pending.Mints)
	res.PendingRequests.Redeems = len(pending.Redeems)
	return res, nil
}

func (s *CustodianService) assetsUnderCustodyIDR(ctx context.Context) (string, error) {
	attestations, err := s.Repos.StockAttestation.FindLatestPerStock(ctx)
	if err != nil {
		return "0", err
	}

	total := big.NewFloat(0)
	tokenDecimals := new(big.Float).SetFloat64(1e18)
	for _, attestation := range attestations {
		rawHoldings, ok := new(big.Int).SetString(attestation.CustodianHoldings, 10)
		if !ok {
			return "0", fmt.Errorf("invalid custodian holdings: %s", attestation.CustodianHoldings)
		}

		price, err := s.Price.GetYahooIDX(attestation.Stock.IdxTicker)
		if err != nil {
			continue
		}

		holdings := new(big.Float).Quo(new(big.Float).SetInt(rawHoldings), tokenDecimals)
		notional := new(big.Float).Mul(holdings, big.NewFloat(price.Price))
		total.Add(total, notional)
	}

	rounded, _ := new(big.Float).Add(total, big.NewFloat(0.5)).Int(nil)
	return rounded.String(), nil
}

func (s *CustodianService) ListPendingRequests(ctx context.Context) (PendingRequestsResponse, error) {
	mints, err := s.Repos.MintProposal.FindPending(ctx)
	if err != nil {
		return PendingRequestsResponse{}, err
	}
	redeems, err := s.Repos.RedeemProposal.FindPending(ctx)
	if err != nil {
		return PendingRequestsResponse{}, err
	}

	items := make([]PendingRequestResponse, 0, len(mints)+len(redeems))
	for _, mint := range mints {
		item, err := s.mintPendingResponse(ctx, mint)
		if err != nil {
			return PendingRequestsResponse{}, err
		}
		items = append(items, item)
	}
	for _, redeem := range redeems {
		item, err := s.redeemPendingResponse(ctx, redeem)
		if err != nil {
			return PendingRequestsResponse{}, err
		}
		items = append(items, item)
	}

	return PendingRequestsResponse{Mints: mints, Redeems: redeems, Items: items}, nil
}

func (s *CustodianService) SubmitAttestation(
	ctx context.Context,
	walletAddress string,
	req SubmitAttestationRequest,
) (model.StockAttestation, error) {
	custodian, found, err := s.Repos.Custodian.FindByWalletAddress(ctx, walletAddress)
	if err != nil {
		return model.StockAttestation{}, err
	}
	if !found {
		return model.StockAttestation{}, ErrCustodianNotFound
	}

	record, err := s.Repos.StockAttestation.Create(ctx, repository.StockAttestationCreateInput{
		StockID:           req.StockID,
		CustodianHoldings: req.CustodianHoldings,
		OnChainSupply:     req.OnChainSupply,
		AttestationHash:   req.AttestationHash,
	})
	if err != nil {
		return model.StockAttestation{}, err
	}

	if s.Stream != nil {
		s.Stream.Emit(external.LevelOK, "[attest]",
			fmt.Sprintf("proof-of-reserves submitted · stock=%d · holdings=%s · supply=%s · attester=%s",
				req.StockID, req.CustodianHoldings, req.OnChainSupply, custodian.WalletAddress[:10]+"..."))
	}

	return record, nil
}

func (s *CustodianService) ListAttestations(ctx context.Context, stockID *int64) ([]model.StockAttestation, error) {
	if stockID != nil {
		return s.Repos.StockAttestation.FindByStockID(ctx, *stockID)
	}
	return s.Repos.StockAttestation.FindLatestPerStock(ctx)
}

func (s *CustodianService) mintPendingResponse(ctx context.Context, mint model.MintProposal) (PendingRequestResponse, error) {
	votes, err := s.Repos.MintApproval.FindByProposalID(ctx, mint.ID)
	if err != nil {
		return PendingRequestResponse{}, err
	}

	var approvals, rejections int64
	attestors := make([]AttestorInfo, 0, len(votes))
	for _, v := range votes {
		if v.Type == "approve" {
			approvals++
		} else {
			rejections++
		}
		txHash := ""
		if v.TxHash != nil {
			txHash = *v.TxHash
		}
		attestors = append(attestors, AttestorInfo{
			Name:          v.Custodian.Name,
			WalletAddress: v.Custodian.WalletAddress,
			Type:          v.Type,
			TxHash:        txHash,
			AttestedAt:    v.AttestedAt,
		})
	}

	source := "institutional"
	if mint.Destination == "liquidity_pool" {
		source = "retail"
	}

	var requesterAddress *string
	if mint.Requester != nil {
		requesterAddress = &mint.Requester.WalletAddress
	}

	return PendingRequestResponse{
		ID:               mint.ID,
		OnChainID:        mint.OnChainID,
		Kind:             "mint",
		Ticker:           mint.Stock.Ticker,
		StockName:        mint.Stock.StockName,
		IdxTicker:        mint.Stock.IdxTicker,
		TokenAmount:      mint.TokenAmount,
		IdrxAmount:       mint.IdrxAmount,
		RequesterAddress: requesterAddress,
		Source:           source,
		Destination:      &mint.Destination,
		ApprovalCount:    approvals,
		RejectCount:      rejections,
		Attestors:        attestors,
		Status:           mint.Status,
		RequestTxHash:    mint.RequestTxHash,
		CreatedAt:        mint.CreatedAt,
	}, nil
}

func (s *CustodianService) redeemPendingResponse(ctx context.Context, redeem model.RedeemProposal) (PendingRequestResponse, error) {
	approvals, err := s.Repos.RedeemApproval.CountByProposalID(ctx, redeem.ID, "approve")
	if err != nil {
		return PendingRequestResponse{}, err
	}
	rejections, err := s.Repos.RedeemApproval.CountByProposalID(ctx, redeem.ID, "reject")
	if err != nil {
		return PendingRequestResponse{}, err
	}

	return PendingRequestResponse{
		ID:            redeem.ID,
		OnChainID:     redeem.OnChainID,
		Kind:          "redeem",
		Ticker:        redeem.Stock.Ticker,
		StockName:     redeem.Stock.StockName,
		IdxTicker:     redeem.Stock.IdxTicker,
		TokenAmount:   redeem.TokenAmount,
		FeeIdrx:       &redeem.FeeIdrx,
		UserAddress:   &redeem.UserAddress,
		Source:        "retail",
		ApprovalCount: approvals,
		RejectCount:   rejections,
		Status:        redeem.Status,
		RequestTxHash: redeem.RequestTxHash,
		CreatedAt:     redeem.CreatedAt,
	}, nil
}
