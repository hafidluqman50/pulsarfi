package authsvc

import (
	"context"
	"fmt"
	"strings"

	"github.com/horizonlabs/pulsarfi-backend/src/auth"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
)

type AuthService struct {
	Custodians *repository.CustodianRepository
	Nonces     *auth.NonceStore
	JwtConfig  auth.Config
}

func (s *AuthService) Nonce(address string) string {
	return s.Nonces.Issue(address)
}

type VerifyInput struct {
	Address   string
	Message   string
	Signature string
	Nonce     string
}

func (s *AuthService) Verify(ctx context.Context, input VerifyInput) (string, error) {
	recovered, err := auth.RecoverAddress(input.Message, input.Signature)
	if err != nil {
		return "", fmt.Errorf("invalid signature: %w", err)
	}

	if !strings.EqualFold(recovered.Hex(), input.Address) {
		return "", fmt.Errorf("signature address mismatch")
	}

	if !s.Nonces.Consume(input.Address, input.Nonce) {
		return "", fmt.Errorf("invalid or expired nonce")
	}

	role := "user"
	_, isCustodian, err := s.Custodians.FindByWalletAddress(ctx, strings.ToLower(input.Address))
	if err != nil {
		return "", err
	}
	if isCustodian {
		role = "custodian"
	}

	return auth.NewAccessToken(s.JwtConfig, strings.ToLower(input.Address), role)
}
