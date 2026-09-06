package public_test

import (
	"context"
	"os"
	"testing"

	"github.com/horizonlabs/pulsarfi-backend/src/config"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
	publicsvc "github.com/horizonlabs/pulsarfi-backend/src/service/public"
)

func TestNetWorthVsIndexUsesHistoricalIndexValue(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping live-DB test")
	}

	db, err := config.NewDatabase(databaseURL)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}

	reader := &publicsvc.PortfolioChartReader{
		Transactions: &repository.StockTransactionRepository{DB: db},
		Price:        &publicsvc.PriceService{Price: external.NewPriceService()},
	}

	data, err := reader.Fetch(context.Background(), publicsvc.LensNetWorthVsIndex, "0xd8bf50c157a79260c77b25f89ef713e6c3feda6f", "1Y")
	if err != nil {
		t.Fatalf("fetch net_worth_vs_index: %v", err)
	}

	points, ok := data.([]publicsvc.NetWorthVsIndexPoint)
	if !ok {
		t.Fatalf("unexpected data type %T", data)
	}
	if len(points) == 0 {
		t.Fatal("expected at least one point for the demo investor wallet")
	}

	seen := map[float64]bool{}
	for _, p := range points {
		seen[p.IndexValue] = true
		t.Logf("date=%s netWorthIdr=%s indexValue=%v", p.Date, p.NetWorthIDR, p.IndexValue)
	}
	if len(seen) <= 1 && len(points) > 1 {
		t.Fatalf("indexValue is identical across all %d points (%v), the flat-benchmark bug is back", len(points), points[0].IndexValue)
	}
}

func TestPriceLineHistoryWorksForAnyRealIDXTicker(t *testing.T) {
	priceSvc := &publicsvc.PriceService{Price: external.NewPriceService()}

	points, err := priceSvc.PriceLineHistory(context.Background(), "VKTR", "1M")
	if err != nil {
		t.Fatalf("VKTR is a real IDX ticker (PT VKTR Teknologi Mobilitas Tbk), not tokenized by PulsarFi, must still chart via Yahoo directly: %v", err)
	}
	if len(points) == 0 {
		t.Fatal("expected real VKTR.JK price history from Yahoo, got zero points")
	}
	t.Logf("VKTR: %d points, first=%+v last=%+v", len(points), points[0], points[len(points)-1])
}
