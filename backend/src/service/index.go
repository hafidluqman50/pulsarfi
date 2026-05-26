package service

import (
	"github.com/horizonlabs/pulsarfi-backend/src/auth"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	authsvc "github.com/horizonlabs/pulsarfi-backend/src/service/auth"
	custodiansvc "github.com/horizonlabs/pulsarfi-backend/src/service/custodian"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
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
	Email                  *external.EmailService
	Storage                *external.StorageService
	Stream                 *external.StreamService
	Price                  *external.PriceService
}

type Config struct {
	Repos          *repository.Registry
	JwtConfig      auth.Config
	NonceStore     *auth.NonceStore
	EmailService   *external.EmailService
	StorageService *external.StorageService
}

func NewRegistry(cfg Config) *Registry {
	stream := external.NewStreamService()
	price := external.NewPriceService()
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
		},
		Email:   cfg.EmailService,
		Storage: cfg.StorageService,
		Stream:  stream,
		Price:   price,
	}
}
