package app

import (
	"context"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/horizonlabs/pulsarfi-backend/src/auth"
	"github.com/horizonlabs/pulsarfi-backend/src/config"
	authhandler "github.com/horizonlabs/pulsarfi-backend/src/http/handlers/auth"
	custodianHandler "github.com/horizonlabs/pulsarfi-backend/src/http/handlers/custodian"
	publicHandler "github.com/horizonlabs/pulsarfi-backend/src/http/handlers/public"
	"github.com/horizonlabs/pulsarfi-backend/src/http/routes"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
	indexersvc "github.com/horizonlabs/pulsarfi-backend/src/service/indexer"
	"github.com/joho/godotenv"
)

func buildHandler() (*gin.Engine, func(), error) {
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, using environment variables")
	}

	databaseURL, err := config.RequireEnv("DATABASE_URL")
	if err != nil {
		return nil, nil, err
	}
	jwtSecret, err := config.RequireEnv("JWT_SECRET")
	if err != nil {
		return nil, nil, err
	}

	db, err := config.NewDatabase(databaseURL)
	if err != nil {
		return nil, nil, fmt.Errorf("database: %w", err)
	}
	indexerCtx, stopIndexer := context.WithCancel(context.Background())
	cleanup := func() {
		stopIndexer()
		if sqlDB, err := db.DB(); err == nil {
			sqlDB.Close()
		}
	}

	jwtConfig := auth.Config{
		Issuer:       "pulsarfi",
		AccessSecret: []byte(jwtSecret),
		AccessTTL:    24 * time.Hour,
	}

	repos := repository.NewRegistry(db)
	nonceStore := auth.NewNonceStore()

	var emailSvc *external.EmailService
	if apiKey := config.GetEnv("RESEND_API_KEY"); apiKey != "" {
		emailSvc = external.NewEmailService(
			apiKey,
			config.GetEnv("RESEND_FROM_EMAIL"),
			config.GetEnv("RESEND_FROM_NAME"),
		)
	}

	var storageSvc *external.StorageService
	if endpoint := config.GetEnv("SUPABASE_S3_ENDPOINT"); endpoint != "" {
		storageBucket := config.GetEnv("SUPABASE_KYC_BUCKET")
		storageSvc = external.NewStorageService(
			endpoint,
			config.GetEnv("SUPABASE_S3_ACCESS_KEY"),
			config.GetEnv("SUPABASE_S3_SECRET_KEY"),
			config.GetEnv("SUPABASE_S3_REGION"),
			storageBucket,
			config.GetEnv("SUPABASE_URL"),
		)
	}

	svcs := service.NewRegistry(service.Config{
		Repos:                 repos,
		JwtConfig:             jwtConfig,
		NonceStore:            nonceStore,
		EmailService:          emailSvc,
		StorageService:        storageSvc,
		TransferIndexerConfig: transferIndexerConfig(),
	})

	authhandler.Configure(svcs.Auth)
	custodianHandler.ConfigureServices(custodianHandler.Services{
		Repos:           repos,
		Custodian:       svcs.Custodian,
		CustodianRedeem: svcs.CustodianRedeem,
		CustodianKYC:    svcs.CustodianKYC,
		Email:           svcs.Email,
		Storage:         svcs.Storage,
		Stream:          svcs.Stream,
	})
	publicHandler.ConfigureRepos(repos)
	publicHandler.ConfigureServices(svcs)

	if transferIndexerEnabled() {
		go svcs.TransferIndexer.Run(indexerCtx)
	}

	return routes.SetupRouter(db, jwtConfig), cleanup, nil
}

func transferIndexerEnabled() bool {
	value := strings.ToLower(config.GetEnv("TRANSFER_INDEXER_ENABLED"))
	return value == "1" || value == "true" || value == "yes"
}

func transferIndexerConfig() indexersvc.TransferIndexerConfig {
	rpcURL := config.GetEnv("ALCHEMY_RPC_URL")
	if rpcURL == "" {
		rpcURL = config.GetEnv("RPC_URL")
	}

	return indexersvc.TransferIndexerConfig{
		RPCURL:        rpcURL,
		PollInterval:  envSeconds("TRANSFER_INDEXER_INTERVAL_SECONDS"),
		BatchSize:     envInt64("TRANSFER_INDEXER_BATCH_SIZE"),
		Confirmations: envInt64("TRANSFER_INDEXER_CONFIRMATIONS"),
		StartLookback: envInt64("TRANSFER_INDEXER_START_LOOKBACK_BLOCKS"),
	}
}

func envInt64(key string) int64 {
	value := config.GetEnv(key)
	if value == "" {
		return 0
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		log.Printf("invalid %s=%q, using default", key, value)
		return 0
	}
	return parsed
}

func envSeconds(key string) time.Duration {
	seconds := envInt64(key)
	if seconds <= 0 {
		return 0
	}
	return time.Duration(seconds) * time.Second
}
