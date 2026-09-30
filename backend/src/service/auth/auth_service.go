package authsvc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/horizonlabs/pulsarfi-backend/src/auth"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
)

const nonceTTL = 5 * time.Minute

type VerifyError struct {
	Message string
}

func (e *VerifyError) Error() string { return e.Message }

type AuthService struct {
	Custodians *repository.CustodianRepository
	SiweNonces *repository.SiweNonceRepository
	JwtConfig  auth.Config
}

func (s *AuthService) Nonce(ctx context.Context, address string) (string, error) {
	raw := make([]byte, 16)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate nonce: %w", err)
	}
	nonce := hex.EncodeToString(raw)

	now := time.Now()
	if err := s.SiweNonces.DeleteExpired(ctx, now); err != nil {
		return "", fmt.Errorf("delete expired nonces: %w", err)
	}
	if err := s.SiweNonces.Upsert(ctx, strings.ToLower(address), nonce, now.Add(nonceTTL)); err != nil {
		return "", fmt.Errorf("store nonce: %w", err)
	}
	return nonce, nil
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
		return "", &VerifyError{Message: fmt.Sprintf("invalid signature: %v", err)}
	}

	if !strings.EqualFold(recovered.Hex(), input.Address) {
		return "", &VerifyError{Message: "signature address mismatch"}
	}

	address := strings.ToLower(input.Address)
	consumed, err := s.SiweNonces.Consume(ctx, address, input.Nonce, time.Now())
	if err != nil {
		return "", fmt.Errorf("consume nonce: %w", err)
	}
	if !consumed {
		return "", &VerifyError{Message: "invalid or expired nonce"}
	}

	role := "user"
	_, isCustodian, err := s.Custodians.FindByWalletAddress(ctx, address)
	if err != nil {
		return "", err
	}
	if isCustodian {
		role = "custodian"
	}

	return auth.NewAccessToken(s.JwtConfig, address, role)
}
