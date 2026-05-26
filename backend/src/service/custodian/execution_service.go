package custodian

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"

	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
)

type RecordMintExecutionRequest struct {
	OnChainID       int64
	TxHash          string
	ContractAddress string
}

type RecordRedeemExecutionRequest struct {
	OnChainID int64
	TxHash    string
}

func (s *CustodianService) RecordMintExecution(ctx context.Context, req RecordMintExecutionRequest) error {
	proposal, found, err := s.Repos.MintProposal.FindByOnChainID(ctx, req.OnChainID)
	if err != nil {
		return err
	}
	if !found {
		return ErrProposalNotFound
	}

	attestationHash := reserveAttestationHash("mint", proposal.StockID, req.TxHash)
	exists, err := s.Repos.StockAttestation.ExistsByAttestationHash(ctx, attestationHash)
	if err != nil {
		return err
	}
	if proposal.Status == "executed" && exists {
		return nil
	}

	nextHoldings, nextSupply, err := s.nextReserveSnapshot(ctx, proposal.StockID, proposal.TokenAmount, reserveDeltaAdd)
	if err != nil {
		return err
	}

	tx := s.Repos.StockAttestation.DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	txRepos := repository.NewRegistry(tx)

	if err := txRepos.MintProposal.MarkExecuted(ctx, req.OnChainID, req.TxHash); err != nil {
		tx.Rollback()
		return err
	}
	if err := txRepos.Stock.UpdateContractAddress(ctx, proposal.StockID, repository.StockUpdateContractInput{
		ContractAddress: req.ContractAddress,
	}); err != nil {
		tx.Rollback()
		return err
	}
	if !exists {
		if _, err := txRepos.StockAttestation.Create(ctx, repository.StockAttestationCreateInput{
			StockID:           proposal.StockID,
			CustodianHoldings: nextHoldings,
			OnChainSupply:     nextSupply,
			AttestationHash:   &attestationHash,
		}); err != nil {
			tx.Rollback()
			return err
		}
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}

	if s.Stream != nil {
		s.Stream.Emit(external.LevelOK, "[por]",
			fmt.Sprintf("reserve snapshot updated · mint proposal=%d · holdings=%s · supply=%s",
				req.OnChainID, nextHoldings, nextSupply))
	}
	return nil
}

func (s *CustodianService) RecordRedeemExecution(ctx context.Context, req RecordRedeemExecutionRequest) error {
	proposal, found, err := s.Repos.RedeemProposal.FindByOnChainID(ctx, req.OnChainID)
	if err != nil {
		return err
	}
	if !found {
		return ErrProposalNotFound
	}

	attestationHash := reserveAttestationHash("redeem", proposal.StockID, req.TxHash)
	exists, err := s.Repos.StockAttestation.ExistsByAttestationHash(ctx, attestationHash)
	if err != nil {
		return err
	}
	if proposal.Status == "executed" && exists {
		return nil
	}

	nextHoldings, nextSupply, err := s.nextReserveSnapshot(ctx, proposal.StockID, proposal.TokenAmount, reserveDeltaSubtract)
	if err != nil {
		return err
	}

	tx := s.Repos.StockAttestation.DB.WithContext(ctx).Begin()
	if tx.Error != nil {
		return tx.Error
	}
	txRepos := repository.NewRegistry(tx)

	if err := txRepos.RedeemProposal.MarkExecuted(ctx, req.OnChainID, req.TxHash); err != nil {
		tx.Rollback()
		return err
	}
	if !exists {
		if _, err := txRepos.StockAttestation.Create(ctx, repository.StockAttestationCreateInput{
			StockID:           proposal.StockID,
			CustodianHoldings: nextHoldings,
			OnChainSupply:     nextSupply,
			AttestationHash:   &attestationHash,
		}); err != nil {
			tx.Rollback()
			return err
		}
	}
	if err := tx.Commit().Error; err != nil {
		return err
	}

	if s.Stream != nil {
		s.Stream.Emit(external.LevelOK, "[por]",
			fmt.Sprintf("reserve snapshot updated · redeem proposal=%d · holdings=%s · supply=%s",
				req.OnChainID, nextHoldings, nextSupply))
	}
	return nil
}

type reserveDeltaMode int

const (
	reserveDeltaAdd reserveDeltaMode = iota
	reserveDeltaSubtract
)

func (s *CustodianService) nextReserveSnapshot(ctx context.Context, stockID int64, tokenAmount string, mode reserveDeltaMode) (string, string, error) {
	latest, found, err := s.Repos.StockAttestation.FindLatestByStockID(ctx, stockID)
	if err != nil {
		return "", "", err
	}

	holdings := big.NewInt(0)
	supply := big.NewInt(0)
	if found {
		holdings, err = parseRawAmount(latest.CustodianHoldings)
		if err != nil {
			return "", "", err
		}
		supply, err = parseRawAmount(latest.OnChainSupply)
		if err != nil {
			return "", "", err
		}
	}

	delta, err := parseRawAmount(tokenAmount)
	if err != nil {
		return "", "", err
	}

	switch mode {
	case reserveDeltaAdd:
		holdings.Add(holdings, delta)
		supply.Add(supply, delta)
	case reserveDeltaSubtract:
		holdings.Sub(holdings, delta)
		supply.Sub(supply, delta)
		if holdings.Sign() < 0 {
			holdings.SetInt64(0)
		}
		if supply.Sign() < 0 {
			supply.SetInt64(0)
		}
	}

	return holdings.String(), supply.String(), nil
}

func parseRawAmount(value string) (*big.Int, error) {
	n, ok := new(big.Int).SetString(value, 10)
	if !ok {
		return nil, fmt.Errorf("invalid raw amount: %s", value)
	}
	return n, nil
}

func reserveAttestationHash(kind string, stockID int64, txHash string) string {
	payload := fmt.Sprintf("por:v1:%s:%d:%s", kind, stockID, txHash)
	sum := sha256.Sum256([]byte(payload))
	return "0x" + hex.EncodeToString(sum[:])
}

var ErrProposalNotFound = errors.New("proposal not found")
