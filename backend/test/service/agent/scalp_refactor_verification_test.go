package agent_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/cloudwego/eino/compose"
	"github.com/horizonlabs/pulsarfi-backend/src/config"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service"
	agentsvc "github.com/horizonlabs/pulsarfi-backend/src/service/agent"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/analyzer"
	"github.com/joho/godotenv"
)

func TestVerifyTickerTool_TokenizedAndUntokenized(t *testing.T) {
	if err := godotenv.Load("../../../.env"); err != nil {
		t.Logf("no .env loaded (%v), relying on already-exported environment", err)
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping test")
	}

	db, err := config.NewDatabase(databaseURL)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	repos := repository.NewRegistry(db)
	registry := service.NewRegistry(service.Config{Repos: repos})

	if registry.PublicPrice == nil {
		t.Fatal("expected registry.PublicPrice to be configured")
	}

	verifyTool, err := analyzer.NewVerifyTickerTool(registry.PublicPrice)
	if err != nil {
		t.Fatalf("failed to create verify_ticker tool: %v", err)
	}

	ctx := context.Background()

	// 1. Test tokenized ticker (e.g. BRPTP)
	rawRes, err := verifyTool.InvokableRun(ctx, `{"ticker": "BRPTP"}`)
	if err != nil {
		t.Fatalf("verify_ticker BRPTP error: %v", err)
	}

	var res analyzer.TickerVerificationResult
	if err := json.Unmarshal([]byte(rawRes), &res); err != nil {
		t.Fatalf("unmarshal verify_ticker result: %v", err)
	}

	if !res.Found {
		t.Errorf("expected BRPTP to be found, got found=false")
	}
	if !res.Tokenized {
		t.Errorf("expected BRPTP to be tokenized, got tokenized=false")
	}
	if res.ContractAddress == "" {
		t.Errorf("expected non-empty contract address for BRPTP")
	}
	if res.Price <= 0 {
		t.Errorf("expected positive price for BRPTP, got %f", res.Price)
	}

	// 2. Test unknown ticker
	rawUnknown, err := verifyTool.InvokableRun(ctx, `{"ticker": "UNKNOWN_TICKER_999"}`)
	if err != nil {
		t.Fatalf("verify_ticker UNKNOWN error: %v", err)
	}

	var unknownRes analyzer.TickerVerificationResult
	if err := json.Unmarshal([]byte(rawUnknown), &unknownRes); err != nil {
		t.Fatalf("unmarshal verify_ticker unknown: %v", err)
	}

	if unknownRes.Found {
		t.Errorf("expected UNKNOWN to not be found, got found=true")
	}
	if unknownRes.Tokenized {
		t.Errorf("expected UNKNOWN to not be tokenized, got tokenized=true")
	}
}

func TestScalpRefactor_CheckPointStoreInterfaces(t *testing.T) {
	if err := godotenv.Load("../../../.env"); err != nil {
		t.Logf("no .env loaded (%v), relying on already-exported environment", err)
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping test")
	}

	db, err := config.NewDatabase(databaseURL)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	repos := repository.NewRegistry(db)
	store := agentsvc.NewPostgresCheckPointStore(repos.AgentCheckpoint)

	// Assert store implements compose.CheckPointStore
	var _ compose.CheckPointStore = store

	// Assert store implements agentsvc.ExtendedCheckPointStore
	var _ agentsvc.ExtendedCheckPointStore = store
}
