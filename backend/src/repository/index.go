package repository

import "gorm.io/gorm"

type Registry struct {
	Stock               *StockRepository
	Custodian           *CustodianRepository
	MintProposal        *MintProposalRepository
	MintApproval        *MintApprovalRepository
	RedeemProposal      *RedeemProposalRepository
	RedeemApproval      *RedeemApprovalRepository
	WalletVerification  *WalletVerificationRepository
	StockTransaction    *StockTransactionRepository
	StockAttestation    *StockAttestationRepository
}

func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		Stock:              &StockRepository{DB: db},
		Custodian:          &CustodianRepository{DB: db},
		MintProposal:       &MintProposalRepository{DB: db},
		MintApproval:       &MintApprovalRepository{DB: db},
		RedeemProposal:     &RedeemProposalRepository{DB: db},
		RedeemApproval:     &RedeemApprovalRepository{DB: db},
		WalletVerification: &WalletVerificationRepository{DB: db},
		StockTransaction:   &StockTransactionRepository{DB: db},
		StockAttestation:   &StockAttestationRepository{DB: db},
	}
}
