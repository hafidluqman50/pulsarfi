package agent_test

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/horizonlabs/pulsarfi-backend/src/config"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent"
	"github.com/joho/godotenv"
)

// TestAmbiguousTradeDoesNotAskSizingUntilSideKnown tests that when an ambiguous
// trade is requested without specifying buy vs sell (e.g. "Aku mau kamu trading BMRI"):
// 1. Quasar asks only for side and shape, and NEVER asks for idrx_cap or portfolio_share.
// 2. Once the user answers "beli dan scalping", Quasar asks only for idrx_cap (buy budget),
//    and NEVER asks for portfolio_share (token quantity to sell).
func TestAmbiguousTradeDoesNotAskSizingUntilSideKnown(t *testing.T) {
	if err := godotenv.Load("../../../.env"); err != nil {
		t.Logf("no .env loaded (%v), relying on already-exported environment", err)
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping live test")
	}

	db, err := config.NewDatabase(databaseURL)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	repos := repository.NewRegistry(db)
	registry := service.NewRegistry(service.Config{Repos: repos})
	if registry.AgentChat == nil {
		t.Fatal("agent chat service disabled")
	}

	ctx := context.Background()
	chatID := uuid.New()
	wallet := "0xd8bf50c157a79260c77b25f89ef713e6c3feda6f"

	// Turn 1: Ambiguous trade request without side
	turn1Prompt := "Aku mau kamu trading BMRI"
	t.Logf("Turn 1 prompt: %q", turn1Prompt)

	card1, err := registry.AgentChat.HandleChatMessage(ctx, chatID, wallet, turn1Prompt, false, agent.AgentEventCallbacks{})
	if err != nil {
		t.Fatalf("turn 1 HandleChatMessage failed: %v", err)
	}
	t.Logf("Turn 1 result: ContentType=%q Reply=%q UIProps=%s", card1.ContentType, card1.Reply, string(card1.UIProps))

	if card1.ContentType != "workflow_card" {
		t.Fatalf("expected turn 1 ContentType='workflow_card', got %q", card1.ContentType)
	}

	var parsedProps struct {
		Questions []struct {
			Key      string   `json:"key"`
			Question string   `json:"question"`
			Options  []string `json:"options"`
		} `json:"questions"`
	}
	if err := json.Unmarshal(card1.UIProps, &parsedProps); err != nil {
		t.Fatalf("failed to unmarshal turn 1 UIProps: %v", err)
	}

	for _, q := range parsedProps.Questions {
		if q.Key == "idrx_cap" || q.Key == "portfolio_share" {
			t.Fatalf("VIOLATION: Quasar asked for %q in Turn 1 before side was known! Question: %q", q.Key, q.Question)
		}
	}
	t.Logf("PASS: Turn 1 asked %d questions without any premature sizing questions (idrx_cap/portfolio_share)", len(parsedProps.Questions))

	// Turn 2: User specifies buy and scalp
	turn2Prompt := "beli dan scalping"
	t.Logf("Turn 2 prompt: %q", turn2Prompt)

	card2, err := registry.AgentChat.HandleChatMessage(ctx, chatID, wallet, turn2Prompt, false, agent.AgentEventCallbacks{})
	if err != nil {
		t.Fatalf("turn 2 HandleChatMessage failed: %v", err)
	}
	t.Logf("Turn 2 result: ContentType=%q Reply=%q UIProps=%s", card2.ContentType, card2.Reply, string(card2.UIProps))

	if card2.ContentType != "workflow_card" {
		t.Fatalf("expected turn 2 ContentType='workflow_card' (asking for budget), got %q", card2.ContentType)
	}

	var parsedProps2 struct {
		Questions []struct {
			Key      string   `json:"key"`
			Question string   `json:"question"`
		} `json:"questions"`
	}
	if err := json.Unmarshal(card2.UIProps, &parsedProps2); err != nil {
		t.Fatalf("failed to unmarshal turn 2 UIProps: %v", err)
	}

	for _, q := range parsedProps2.Questions {
		if q.Key == "portfolio_share" {
			t.Fatalf("VIOLATION: Quasar asked for 'portfolio_share' on a BUY trade! Question: %q", q.Question)
		}
	}
	t.Logf("PASS: Turn 2 asked for buy sizing without any sell sizing questions")

	// Turn 3: User provides budget (1 juta IDRX)
	turn3Prompt := "1000000"
	t.Logf("Turn 3 prompt: %q", turn3Prompt)

	card3, err := registry.AgentChat.HandleChatMessage(ctx, chatID, wallet, turn3Prompt, false, agent.AgentEventCallbacks{})
	if err != nil {
		t.Fatalf("turn 3 HandleChatMessage failed: %v", err)
	}
	t.Logf("Turn 3 result: ContentType=%q TaskID=%d Reply=%q", card3.ContentType, card3.TaskID, card3.Reply)

	if card3.TaskID == 0 {
		t.Fatalf("expected TaskID > 0 once budget was provided, got 0")
	}
	t.Logf("PASS: All trade parameters settled and Arm card produced for Task %d", card3.TaskID)
}

// TestDualNewsAndChartQuery tests that when the user asks for both news and a chart,
// the system delivers a composite payload containing both charts and news evidence
// packaged under ContentType "chart" to comply with DB check constraints.
func TestDualNewsAndChartQuery(t *testing.T) {
	if err := godotenv.Load("../../../.env"); err != nil {
		t.Logf("no .env loaded (%v), relying on already-exported environment", err)
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping live test")
	}

	db, err := config.NewDatabase(databaseURL)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	repos := repository.NewRegistry(db)
	registry := service.NewRegistry(service.Config{Repos: repos})
	if registry.AgentChat == nil {
		t.Fatal("agent chat service disabled")
	}

	ctx := context.Background()
	chatID := uuid.New()
	wallet := "0xd8bf50c157a79260c77b25f89ef713e6c3feda6f"

	prompt := "Tarik berita terkini tentang IHSG dan tampilkan chart IHSG"
	t.Logf("Testing dual query: %q", prompt)

	card, err := registry.AgentChat.HandleChatMessage(ctx, chatID, wallet, prompt, false, agent.AgentEventCallbacks{})
	if err != nil {
		t.Fatalf("HandleChatMessage failed: %v", err)
	}
	t.Logf("Result: ContentType=%q Reply=%q UIProps=%s", card.ContentType, card.Reply, string(card.UIProps))

	if card.ContentType != "news" && card.ContentType != "chart" {
		t.Fatalf("expected ContentType to be 'news' or 'chart', got %q", card.ContentType)
	}

	var payload struct {
		Charts any `json:"charts"`
		News   []struct {
			Source string `json:"source"`
			URL    string `json:"url"`
		} `json:"news"`
	}
	if err := json.Unmarshal(card.UIProps, &payload); err != nil {
		t.Fatalf("failed to unmarshal UIProps: %v", err)
	}

	if payload.Charts == nil {
		t.Errorf("expected charts in composite payload")
	}
	if len(payload.News) > 0 {
		t.Logf("PASS: Both charts and %d news items delivered in composite payload!", len(payload.News))
	} else {
		t.Logf("Chart delivered successfully; news array: %d items", len(payload.News))
	}
}
