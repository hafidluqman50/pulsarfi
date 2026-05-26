package app

import (
	"fmt"
	"log"
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
	cleanup := func() {
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
		storageSvc = external.NewStorageService(
			endpoint,
			config.GetEnv("SUPABASE_S3_ACCESS_KEY"),
			config.GetEnv("SUPABASE_S3_SECRET_KEY"),
			config.GetEnv("SUPABASE_S3_REGION"),
			config.GetEnv("SUPABASE_STORAGE_BUCKET"),
			config.GetEnv("SUPABASE_URL"),
		)
	}

	svcs := service.NewRegistry(service.Config{
		Repos:          repos,
		JwtConfig:      jwtConfig,
		NonceStore:     nonceStore,
		EmailService:   emailSvc,
		StorageService: storageSvc,
	})

	authhandler.Configure(svcs.Auth)
	custodianHandler.ConfigureServices(custodianHandler.Services{
		Repos:     repos,
		Custodian: svcs.Custodian,
		Email:     svcs.Email,
		Storage:   svcs.Storage,
		Stream:    svcs.Stream,
	})
	publicHandler.ConfigureRepos(repos)
	publicHandler.ConfigureServices(svcs)

	return routes.SetupRouter(db, jwtConfig), cleanup, nil
}
