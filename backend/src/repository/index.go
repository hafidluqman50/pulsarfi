package repository

import "gorm.io/gorm"

type Registry struct {
	Stock              *StockRepository
	Custodian          *CustodianRepository
	MintProposal       *MintProposalRepository
	MintApproval       *MintApprovalRepository
	RedeemProposal     *RedeemProposalRepository
	RedeemApproval     *RedeemApprovalRepository
	WalletVerification *WalletVerificationRepository
	StockTransaction   *StockTransactionRepository
	StockAttestation   *StockAttestationRepository
	TransferCheckpoint *TransferIndexerCheckpointRepository
	AgentTask          *AgentTaskRepository
	AgentChat          *AgentChatRepository
	AgentChatMessage   *AgentChatMessageRepository
	AgentSubTask       *AgentSubTaskRepository
	AgentTrade         *AgentTradeRepository
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
		TransferCheckpoint: &TransferIndexerCheckpointRepository{DB: db},
		AgentTask:          &AgentTaskRepository{DB: db},
		AgentChat:          &AgentChatRepository{DB: db},
		AgentChatMessage:   &AgentChatMessageRepository{DB: db},
		AgentSubTask:       &AgentSubTaskRepository{DB: db},
		AgentTrade:         &AgentTradeRepository{DB: db},
	}
}
