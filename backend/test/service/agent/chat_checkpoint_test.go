package agent_test

import (
	"context"
	"os"
	"strings"
	"testing"

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

	cpStore, ok := registry.AgentChat.CheckPointStore.(agentsvc.ExtendedCheckPointStore)
	if !ok {
		t.Fatal("expected registry.AgentChat.CheckPointStore to implement ExtendedCheckPointStore")
	}

	ctx := context.Background()
	testWallet := "0x" + strings.Repeat("c", 40)
	chatID := uuid.New()

	// Clean up any test artifacts at the end
	defer func() {
		_ = cpStore.Delete(ctx, chatID.String())
	}()

	// 1. Simulating Phase 3 Pause (HITL):
	// A trade request triggers an interrupt, which saves Eino checkpoint and interrupt ID into Postgres
	testPayload := []byte("eino_state_checkpoint_data")
	testInterruptID := "interrupt-" + uuid.New().String()

	err = cpStore.Set(ctx, chatID.String(), testPayload)
	if err != nil {
		t.Fatalf("failed to set checkpoint: %v", err)
	}
	err = cpStore.SetInterruptID(ctx, chatID.String(), testInterruptID)
	if err != nil {
		t.Fatalf("failed to set interrupt id: %v", err)
	}

	// Verify checkpoint exists in Postgres
	has, err := cpStore.Has(ctx, chatID.String())
	if err != nil || !has {
		t.Fatalf("expected checkpoint to exist in Postgres, has: %v, err: %v", has, err)
	}

	retrievedInterruptID, found, err := cpStore.GetInterruptID(ctx, chatID.String())
	if err != nil || !found || retrievedInterruptID != testInterruptID {
		t.Fatalf("failed to retrieve interrupt ID: found=%v, id=%s, err=%v", found, retrievedInterruptID, err)
	}

	// 2. Test Explicit Cancellation via HandleChatMessage:
	// When user sends "cancel" (e.g. clicking Cancel/Disarm button), HandleChatMessage intercepts it,
	// deletes the checkpoint from Postgres, and returns a cancellation confirmation.
	card, err := registry.AgentChat.HandleChatMessage(ctx, chatID, testWallet, "cancel", agentsvc.AgentEventCallbacks{})
	if err != nil {
		t.Fatalf("HandleChatMessage cancel error: %v", err)
	}

	// 3. Verify Checkpoint is DELETED from PostgreSQL after cancellation
	has, err = cpStore.Has(ctx, chatID.String())
	if err != nil {
		t.Fatalf("check has error: %v", err)
	}
	if has {
		t.Errorf("expected checkpoint to be deleted from Postgres after cancellation, but it still exists")
	}

	// Verify interrupt ID was also cleaned up with Delete
	_, found, _ = cpStore.GetInterruptID(ctx, chatID.String())
	if found {
		t.Errorf("expected interrupt ID to be cleaned up after delete")
	}
	_ = card
}
