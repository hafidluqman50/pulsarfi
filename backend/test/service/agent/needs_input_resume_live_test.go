package agent_test

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/horizonlabs/pulsarfi-backend/src/config"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent"
	"github.com/joho/godotenv"
)

// TestNeedsInputThenResumeProducesArmCard reproduces the exact live crash:
// an incomplete scalp buy prompt (missing budget) pauses on needs_input,
// then the user's next message in the SAME chat answers it. This is the
// resume path (ChatService.resumeOrRun -> Orchestrator.ResumeRoute) that
// TestDirectBuyPromptProducesConfirmationCard's single-message prompts never
// exercise.
//
// Run explicitly: go test ./test/service/agent/... -run TestNeedsInputThenResumeProducesArmCard -v
func TestNeedsInputThenResumeProducesArmCard(t *testing.T) {
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

	firstPrompt := "Ayo beli BRPT scalping"
	t.Logf("Turn 1 (should pause on needs_input, missing budget): %q", firstPrompt)

	card1, err := registry.AgentChat.HandleChatMessage(ctx, chatID, wallet, firstPrompt, false, agent.AgentEventCallbacks{})
	if err != nil {
		t.Fatalf("turn 1 HandleChatMessage failed (this is the live panic if it reoccurs): %v", err)
	}
	t.Logf("Turn 1 result: ContentType=%q TaskID=%d Reply=%q PendingQuestions=%+v", card1.ContentType, card1.TaskID, card1.Reply, card1.UIProps)

	if card1.TaskID != 0 {
		t.Fatalf("turn 1 should NOT have committed a Task yet (budget still missing), got TaskID=%d", card1.TaskID)
	}
	if card1.ContentType != "workflow_card" {
		t.Fatalf("expected turn 1 to pause with a workflow_card (clarifying question), got ContentType=%q Reply=%q", card1.ContentType, card1.Reply)
	}

	secondPrompt := "5000000"
	t.Logf("Turn 2 (answers the budget question, same chat, should resume): %q", secondPrompt)

	card2, err := registry.AgentChat.HandleChatMessage(ctx, chatID, wallet, secondPrompt, false, agent.AgentEventCallbacks{})
	if err != nil {
		t.Fatalf("turn 2 HandleChatMessage failed (THIS is the resume path that panicked live): %v", err)
	}
	t.Logf("Turn 2 result: ContentType=%q TaskID=%d Reply=%q", card2.ContentType, card2.TaskID, card2.Reply)

	if card2.TaskID == 0 {
		t.Fatalf("FAILED: turn 2 should have committed a Task and shown the Arm Card once budget was answered, got TaskID=0, Reply=%q", card2.Reply)
	}

	task, found, err := repos.AgentTask.FindByID(ctx, card2.TaskID)
	if err != nil || !found {
		t.Fatalf("Task %d not found in database: %v", card2.TaskID, err)
	}
	if !task.IsActionable {
		t.Errorf("expected task to be actionable, got %v", task.IsActionable)
	}

	t.Logf("SUCCESS: needs_input -> resume -> Task T-%d committed with Arm Card, no panic.", card2.TaskID)
}
