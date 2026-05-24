package repository

import "gorm.io/gorm"

type Registry struct {
	Stock               *StockRepository
	Custodian           *CustodianRepository
	MintProposal        *MintProposalRepository
	MintApproval        *MintApprovalRepository
	WalletVerification  *WalletVerificationRepository
	StockTransaction    *StockTransactionRepository
}

func NewRegistry(db *gorm.DB) *Registry {
	return &Registry{
		Stock:              &StockRepository{DB: db},
		Custodian:          &CustodianRepository{DB: db},
		MintProposal:       &MintProposalRepository{DB: db},
		MintApproval:       &MintApprovalRepository{DB: db},
		WalletVerification: &WalletVerificationRepository{DB: db},
		StockTransaction:   &StockTransactionRepository{DB: db},
	}
}
