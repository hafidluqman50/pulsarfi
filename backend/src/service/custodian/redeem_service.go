package custodian

import (
	"context"
	"errors"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
)

var ErrRedeemProposalNotFound = errors.New("redeem proposal not found")
var ErrRedeemAlreadyVoted = errors.New("custodian already voted on this proposal")

type RedeemService struct {
	RedeemProposals    *repository.RedeemProposalRepository
	RedeemAttestations *repository.RedeemApprovalRepository
	Custodians         *repository.CustodianRepository
}

type RecordRedeemVoteRequest struct {
	OnChainID     int64
	CustodianAddr string
	VoteType      string // "approve" | "reject"
	TxHash        string
}

func (s *RedeemService) ListPending(ctx context.Context) ([]model.RedeemProposal, error) {
	return s.RedeemProposals.FindPending(ctx)
}

func (s *RedeemService) RecordVote(ctx context.Context, req RecordRedeemVoteRequest) error {
	proposal, found, err := s.RedeemProposals.FindByOnChainID(ctx, req.OnChainID)
	if err != nil {
		return err
	}
	if !found {
		return ErrRedeemProposalNotFound
	}

	custodian, found, err := s.Custodians.FindByWalletAddress(ctx, req.CustodianAddr)
	if err != nil {
		return err
	}
	if !found {
		return errors.New("custodian not found")
	}

	if err := s.RedeemAttestations.Create(ctx, proposal.ID, custodian.ID, req.VoteType, &req.TxHash); err != nil {
		return ErrRedeemAlreadyVoted
	}

	return nil
}
