package custodian

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
)

var walletAddressPattern = regexp.MustCompile(`^0x[0-9a-f]{40}$`)

var (
	ErrKYCStorageNotConfigured = errors.New("kyc storage not configured")
	ErrKYCRecordNotFound       = errors.New("kyc record not found")
)

type KYCService struct {
	WalletVerifications *repository.WalletVerificationRepository
	Custodians          *repository.CustodianRepository
	Storage             *external.StorageService
}

type CreateVerifiedKYCRequest struct {
	WalletAddress  string
	Type           string
	FullName       string
	Email          string
	ApprovalTxHash string
	Document       multipart.File
	DocumentHeader *multipart.FileHeader
	CustodianAddr  string
}

type DocumentURLResult struct {
	URL       string `json:"url"`
	ExpiresIn int    `json:"expires_in"`
}

func (s *KYCService) List(ctx context.Context, status string) ([]model.WalletVerification, error) {
	return s.WalletVerifications.FindAll(ctx, status)
}

func (s *KYCService) CreateVerified(ctx context.Context, req CreateVerifiedKYCRequest) (model.WalletVerification, error) {
	custodian, found, err := s.Custodians.FindByWalletAddress(ctx, req.CustodianAddr)
	if err != nil {
		return model.WalletVerification{}, err
	}
	if !found {
		return model.WalletVerification{}, ErrCustodianNotFound
	}
	if s.Storage == nil {
		return model.WalletVerification{}, ErrKYCStorageNotConfigured
	}
	if req.Document == nil || req.DocumentHeader == nil {
		return model.WalletVerification{}, errors.New("signed statement document is required")
	}
	if req.Type == "" {
		req.Type = "retail"
	}
	if req.Type != "retail" && req.Type != "institution" {
		return model.WalletVerification{}, errors.New("type must be retail or institution")
	}
	if req.DocumentHeader.Size > 10*1024*1024 {
		return model.WalletVerification{}, errors.New("signed statement must be 10MB or smaller")
	}

	wallet := strings.ToLower(strings.TrimSpace(req.WalletAddress))
	if !walletAddressPattern.MatchString(wallet) {
		return model.WalletVerification{}, errors.New("invalid wallet address")
	}
	ext := strings.ToLower(filepath.Ext(req.DocumentHeader.Filename))
	if ext == "" {
		ext = ".pdf"
	}
	timestamp := time.Now().UTC().Format("2006-01-02T15-04-05Z")
	documentRef := fmt.Sprintf("wallet-verifications/%s/%s-signed-statement%s", wallet, timestamp, ext)

	ref, err := s.Storage.UploadPrivate(ctx, documentRef, req.Document, req.DocumentHeader)
	if err != nil {
		return model.WalletVerification{}, err
	}

	fullName := strings.TrimSpace(req.FullName)
	email := strings.TrimSpace(req.Email)
	txHash := strings.TrimSpace(req.ApprovalTxHash)

	return s.WalletVerifications.Create(ctx, repository.WalletVerificationCreateInput{
		WalletAddress:  wallet,
		Type:           req.Type,
		Status:         "approved",
		FullName:       stringPtrOrNil(fullName),
		Email:          stringPtrOrNil(email),
		DocumentRef:    &ref,
		ApprovalTxHash: stringPtrOrNil(txHash),
		VerifiedBy:     &custodian.ID,
	})
}

func (s *KYCService) UpdateStatus(ctx context.Context, id int64, status string, custodianAddr string) error {
	custodian, found, err := s.Custodians.FindByWalletAddress(ctx, custodianAddr)
	if err != nil {
		return err
	}
	if !found {
		return ErrCustodianNotFound
	}
	return s.WalletVerifications.UpdateStatus(ctx, id, status, &custodian.ID)
}

func (s *KYCService) DocumentURL(ctx context.Context, id int64) (DocumentURLResult, error) {
	if s.Storage == nil {
		return DocumentURLResult{}, ErrKYCStorageNotConfigured
	}

	record, found, err := s.WalletVerifications.FindByID(ctx, id)
	if err != nil {
		return DocumentURLResult{}, err
	}
	if !found || record.DocumentRef == nil || *record.DocumentRef == "" {
		return DocumentURLResult{}, ErrKYCRecordNotFound
	}
	ttl := 5 * time.Minute
	url, err := s.Storage.SignedURL(ctx, *record.DocumentRef, ttl)
	if err != nil {
		return DocumentURLResult{}, err
	}
	return DocumentURLResult{URL: url, ExpiresIn: int(ttl.Seconds())}, nil
}

func stringPtrOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}
