package service

import (
	"github.com/horizonlabs/pulsarfi-backend/src/auth"
	authsvc "github.com/horizonlabs/pulsarfi-backend/src/service/auth"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
)

type Registry struct {
	Auth    *authsvc.AuthService
	Email   *external.EmailService
	Storage *external.StorageService
	Stream  *external.StreamService
}

type Config struct {
	Repos          *repository.Registry
	JwtConfig      auth.Config
	NonceStore     *auth.NonceStore
	EmailService   *external.EmailService
	StorageService *external.StorageService
}

func NewRegistry(cfg Config) *Registry {
	return &Registry{
		Auth: &authsvc.AuthService{
			Custodians: cfg.Repos.Custodian,
			Nonces:     cfg.NonceStore,
			JwtConfig:  cfg.JwtConfig,
		},
		Email:   cfg.EmailService,
		Storage: cfg.StorageService,
		Stream:  external.NewStreamService(),
	}
}
