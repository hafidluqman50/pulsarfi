package service

import (
	"log"

	"github.com/horizonlabs/pulsarfi-backend/src/auth"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	authsvc "github.com/horizonlabs/pulsarfi-backend/src/service/auth"
	custodiansvc "github.com/horizonlabs/pulsarfi-backend/src/service/custodian"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
	indexersvc "github.com/horizonlabs/pulsarfi-backend/src/service/indexer"
	publicsvc "github.com/horizonlabs/pulsarfi-backend/src/service/public"
)

type Registry struct {
	Repos                  *repository.Registry
	Auth                   *authsvc.AuthService
	Custodian              *custodiansvc.CustodianService
	PublicStock            *publicsvc.StockService
	PublicPrice            *publicsvc.PriceService
	PublicReserve          *publicsvc.ReserveService
	PublicStockTransaction *publicsvc.StockTransactionService
	PublicStats            *publicsvc.StatsService
	PublicRedeem           *publicsvc.PublicRedeemService
	CustodianRedeem        *custodiansvc.RedeemService
	CustodianKYC           *custodiansvc.KYCService
	Email                  *external.EmailService
	Storage                *external.StorageService
	Stream                 *external.StreamService
	Price                  *external.PriceService
	TransferIndexer        *indexersvc.TransferIndexerService
}

type Config struct {
	Repos                 *repository.Registry
	JwtConfig             auth.Config
	NonceStore            *auth.NonceStore
	EmailService          *external.EmailService
	StorageService        *external.StorageService
	TransferIndexerConfig indexersvc.TransferIndexerConfig
}

func NewRegistry(cfg Config) *Registry {
	stream := external.NewStreamService()
	price := external.NewPriceService()

	// Assign only on success: a failed *external.ChainVerifyService stored in
	// a nil interface var would make publicsvc.RedeemVerifier(redeemVerifier)
	// != nil even though the underlying pointer is nil (Go typed-nil trap).
	var chainVerifier *external.ChainVerifyService
	if v, err := external.NewChainVerifyServiceFromEnv(); err != nil {
		log.Printf("chainverify disabled: %v (redeem/swap/transfer recording will reject writes)", err)
	} else {
		chainVerifier = v
	}

	var redeemVerifier publicsvc.RedeemVerifier
	var swapVerifier publicsvc.SwapVerifier
	if chainVerifier != nil {
		redeemVerifier = chainVerifier
		swapVerifier = chainVerifier
	}

	return &Registry{
		Repos: cfg.Repos,
		Auth: &authsvc.AuthService{
			Custodians: cfg.Repos.Custodian,
			Nonces:     cfg.NonceStore,
			JwtConfig:  cfg.JwtConfig,
		},
		Custodian: &custodiansvc.CustodianService{
			Repos:  cfg.Repos,
			Stream: stream,
			Price:  price,
		},
		PublicStock: &publicsvc.StockService{
			Stocks: cfg.Repos.Stock,
			Price:  price,
		},
		PublicPrice: &publicsvc.PriceService{
			Stocks: cfg.Repos.Stock,
			Price:  price,
		},
		PublicReserve: &publicsvc.ReserveService{
			Attestations: cfg.Repos.StockAttestation,
		},
		PublicStockTransaction: &publicsvc.StockTransactionService{
			Stocks:       cfg.Repos.Stock,
			Transactions: cfg.Repos.StockTransaction,
			Verifier:     swapVerifier,
		},
		PublicStats: &publicsvc.StatsService{
			Transactions: cfg.Repos.StockTransaction,
			Stocks:       cfg.Repos.Stock,
			Price:        price,
		},
		PublicRedeem: &publicsvc.PublicRedeemService{
			Stocks:            cfg.Repos.Stock,
			RedeemProposals:   cfg.Repos.RedeemProposal,
			StockTransactions: cfg.Repos.StockTransaction,
			Verifier:          redeemVerifier,
		},
		CustodianRedeem: &custodiansvc.RedeemService{
			RedeemProposals:    cfg.Repos.RedeemProposal,
			RedeemAttestations: cfg.Repos.RedeemApproval,
			Custodians:         cfg.Repos.Custodian,
		},
		CustodianKYC: &custodiansvc.KYCService{
			WalletVerifications: cfg.Repos.WalletVerification,
			Custodians:          cfg.Repos.Custodian,
			Storage:             cfg.StorageService,
		},
		Email:   cfg.EmailService,
		Storage: cfg.StorageService,
		Stream:  stream,
		Price:   price,
		TransferIndexer: &indexersvc.TransferIndexerService{
			Stocks:      cfg.Repos.Stock,
			Checkpoints: cfg.Repos.TransferCheckpoint,
			Recorder: &publicsvc.StockTransactionService{
				Stocks:       cfg.Repos.Stock,
				Transactions: cfg.Repos.StockTransaction,
				Verifier:     swapVerifier,
			},
			Price:  price,
			Config: cfg.TransferIndexerConfig,
		},
	}
}
