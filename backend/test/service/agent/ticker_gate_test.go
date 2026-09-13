package agent_test

import (
	"context"
	"os"
	"testing"

	"github.com/horizonlabs/pulsarfi-backend/src/config"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/joho/godotenv"
)

func TestStockRepository_TickerAutoResolution(t *testing.T) {
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
	ctx := context.Background()

	// 1. Check BMRI without "P" suffix resolves to BMRIP
	stock, found, err := repos.Stock.FindByTickerOrIdxTicker(ctx, "BMRI")
	if err != nil {
		t.Fatalf("lookup BMRI error: %v", err)
	}
	if !found {
		t.Fatalf("expected BMRI to be found in stock repository")
	}
	if stock.Ticker != "BMRIP" {
		t.Fatalf("expected stock.Ticker to be BMRIP, got %q", stock.Ticker)
	}

	// 2. Check lower-case bmri resolves to BMRIP
	stockLower, found, err := repos.Stock.FindByTickerOrIdxTicker(ctx, "bmri")
	if err != nil {
		t.Fatalf("lookup bmri error: %v", err)
	}
	if !found || stockLower.Ticker != "BMRIP" {
		t.Fatalf("expected lower-case bmri to resolve to BMRIP, got %q (found=%v)", stockLower.Ticker, found)
	}

	// 3. Check FindByTicker("BMRI") resolves to BMRIP
	stockByTicker, found, err := repos.Stock.FindByTicker(ctx, "BMRI")
	if err != nil {
		t.Fatalf("FindByTicker(BMRI) error: %v", err)
	}
	if !found || stockByTicker.Ticker != "BMRIP" {
		t.Fatalf("expected FindByTicker(BMRI) to resolve to BMRIP, got %q (found=%v)", stockByTicker.Ticker, found)
	}

	// 4. Check FindByTicker("BMRIP") directly resolves to BMRIP
	stockCanonical, found, err := repos.Stock.FindByTicker(ctx, "BMRIP")
	if err != nil {
		t.Fatalf("FindByTicker(BMRIP) error: %v", err)
	}
	if !found || stockCanonical.Ticker != "BMRIP" {
		t.Fatalf("expected FindByTicker(BMRIP) to resolve to BMRIP, got %q (found=%v)", stockCanonical.Ticker, found)
	}

	// 5. Check unlisted ticker returns found=false
	unlisted, found, err := repos.Stock.FindByTickerOrIdxTicker(ctx, "XYZUNLISTED")
	if err != nil {
		t.Fatalf("lookup unlisted error: %v", err)
	}
	if found {
		t.Fatalf("expected XYZUNLISTED to not be found, got %v", unlisted.Ticker)
	}
}
