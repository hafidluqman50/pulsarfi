package agent_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/horizonlabs/pulsarfi-backend/src/config"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service"
	agentsvc "github.com/horizonlabs/pulsarfi-backend/src/service/agent"
	"github.com/joho/godotenv"
)

func TestChatService_CheckpointPauseResumeCancelAndKeepAlive(t *testing.T) {
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

	if registry.AgentChat == nil {
		t.Fatal("expected registry.AgentChat to be configured")
	}
	if registry.AgentChat.CheckPointStore == nil {
		t.Fatal("expected registry.AgentChat.CheckPointStore to be configured")
	}

	ctx := context.Background()
	testWallet := "0x" + strings.Repeat("c", 40)
	chatID := uuid.New()

	// Clean up any test artifacts at the end
	defer func() {
		_ = registry.AgentChat.CheckPointStore.Delete(ctx, chatID.String())
	}()

	// 1. Simulating Phase 3 Pause (HITL):
	// A trade request triggers needs_input, which saves PendingTradeCheckpoint into Postgres
	pendingCP := agentsvc.PendingTradeCheckpoint{
		ChatID:           chatID,
		MentionedTicker:  "BRPTP",
		ResolvedTicker:   "BRPTP",
		Shape:            "swing",
		Side:             "beli",
		Summary:          "Beli BRPTP swing 20% modal",
		PendingQuestions: []agentsvc.IntakeField{agentsvc.ShapeField},
		CreatedAt:        time.Now(),
	}

	err = registry.AgentChat.CheckPointStore.SetPendingTrade(ctx, chatID.String(), pendingCP)
	if err != nil {
		t.Fatalf("failed to set pending trade checkpoint: %v", err)
	}

	// Verify checkpoint exists in Postgres
	has, err := registry.AgentChat.CheckPointStore.Has(ctx, chatID.String())
	if err != nil || !has {
		t.Fatalf("expected checkpoint to exist in Postgres, has: %v, err: %v", has, err)
	}

	retrieved, found, err := registry.AgentChat.CheckPointStore.GetPendingTrade(ctx, chatID.String())
	if err != nil || !found || retrieved == nil {
		t.Fatalf("failed to retrieve pending trade: found=%v, err=%v", found, err)
	}
	if retrieved.ResolvedTicker != "BRPTP" {
		t.Errorf("expected ticker 'BRPTP', got %q", retrieved.ResolvedTicker)
	}

	// 2. Test Side-Question Keep-Alive & Eino Graph Run Non-Collision:
	// When user sends a side-question while a pending trade checkpoint is active,
	// Orchestrator.Run must NOT fail with "[GraphRunError] no tasks to execute".
	// The pending trade checkpoint must remain intact with a reminder appended.
	sideCard, err := registry.AgentChat.HandleChatMessage(ctx, chatID, testWallet, "Apa kabar?", agentsvc.AgentEventCallbacks{})
	if err != nil {
		t.Fatalf("HandleChatMessage during active checkpoint failed: %v", err)
	}
	if sideCard.Reply == "" {
		t.Fatal("expected non-empty reply from Quasar for side question")
	}
	if !strings.Contains(sideCard.Reply, "menunggu konfirmasi") {
		t.Errorf("expected side question reply to contain pending trade reminder footnote, got: %q", sideCard.Reply)
	}

	// Verify checkpoint stays alive in Postgres after side question
	has, err = registry.AgentChat.CheckPointStore.Has(ctx, chatID.String())
	if err != nil || !has {
		t.Fatalf("expected checkpoint to remain active after side question, has=%v, err=%v", has, err)
	}

	// 3. Test Explicit Cancellation via HandleChatMessage:
	// When user sends "cancel" (e.g. clicking Cancel/Disarm button), HandleChatMessage intercepts it,
	// deletes the checkpoint from Postgres, and returns a cancellation reply.
	card, err := registry.AgentChat.HandleChatMessage(ctx, chatID, testWallet, "cancel", agentsvc.AgentEventCallbacks{})
	if err != nil {
		t.Fatalf("HandleChatMessage cancel error: %v", err)
	}

	if !strings.Contains(strings.ToLower(card.Reply), "cancelled") {
		t.Errorf("expected reply to mention 'cancelled', got: %q", card.Reply)
	}
	if !strings.Contains(card.Reply, "BRPTP") {
		t.Errorf("expected reply to mention ticker 'BRPTP', got: %q", card.Reply)
	}

	// 4. Verify Checkpoint is DELETED from PostgreSQL after cancellation
	has, err = registry.AgentChat.CheckPointStore.Has(ctx, chatID.String())
	if err != nil {
		t.Fatalf("check has error: %v", err)
	}
	if has {
		t.Errorf("expected checkpoint to be deleted from Postgres after cancellation, but it still exists")
	}
}
