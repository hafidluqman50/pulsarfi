package agent_test

import (
	"context"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/chat"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/checkpoint"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/orchestrator"
	"os"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	agentcore "github.com/horizonlabs/pulsarfi-backend/src/app/core/agent"
	"github.com/horizonlabs/pulsarfi-backend/src/config"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/joho/godotenv"
)

func TestTurnRegistry_OnlyOneTurnPerChat(t *testing.T) {
	registry := agentcore.NewTurnRegistry()
	chatID := uuid.New()

	release, ok := registry.TryStart(chatID)
	if !ok {
		t.Fatal("the first TryStart must succeed")
	}
	if running, startedAt := registry.Status(chatID); !running || startedAt.IsZero() {
		t.Fatalf("expected a running turn with a start time, got running=%v startedAt=%v", running, startedAt)
	}

	if _, ok := registry.TryStart(chatID); ok {
		t.Fatal("a second TryStart for the same chat must be rejected")
	}

	release()
	release() // idempotent

	if running, _ := registry.Status(chatID); running {
		t.Fatal("expected the slot to be free after release")
	}
	if _, ok := registry.TryStart(chatID); !ok {
		t.Fatal("the slot must be reusable after release")
	}
}

func TestTurnRegistry_IsPerChat(t *testing.T) {
	registry := agentcore.NewTurnRegistry()
	first, second := uuid.New(), uuid.New()

	release, ok := registry.TryStart(first)
	if !ok {
		t.Fatal("first chat: expected the slot")
	}
	defer release()

	if _, ok := registry.TryStart(second); !ok {
		t.Fatal("a different chat must not be blocked by the first one")
	}
}

// TestChatTurn_SurvivesACancelledRequest is the regression test for the
// reported bug: a refresh aborts the HTTP request, which cancels
// c.Request.Context(). The turn must keep running on its own context — before
// this fix the very first DB write failed with context.Canceled and nothing
// was persisted.
func TestChatTurn_SurvivesACancelledRequest(t *testing.T) {
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

	checkpoints := checkpoint.NewPostgresCheckPointStore(repos.AgentCheckpoint)
	orch := &orchestrator.Orchestrator{
		Tasks:           repos.AgentTask,
		SubTasks:        repos.AgentSubTask,
		ChatMessages:    repos.AgentChatMessage,
		Quasar:          newAgent(t, "supervisor_agent", []*schema.Message{answer("", "ok")}),
		CheckPointStore: checkpoints,
	}
	orch, err = orchestrator.NewOrchestrator(context.Background(), orch)
	if err != nil {
		t.Fatalf("build orchestrator: %v", err)
	}

	registry := agentcore.NewTurnRegistry()
	svc := &chat.ChatService{
		Chats:           repos.AgentChat,
		ChatMessages:    repos.AgentChatMessage,
		Orchestrator:    orch,
		CheckPointStore: checkpoints,
		Turns:           registry,
	}

	chatID := uuid.New()
	wallet := "0x" + strings.Repeat("e", 40)

	// The caller is already gone — exactly what a refresh does to the request
	// context before the handler has finished.
	requestCtx, cancelRequest := context.WithCancel(context.Background())
	cancelRequest()

	card, err := svc.HandleChatMessage(requestCtx, chatID, wallet, "halo", false)
	if err != nil {
		t.Fatalf("a cancelled request must not cancel the turn, got: %v", err)
	}
	if card.Reply != "ok" {
		t.Fatalf("expected the turn to complete, got reply %q", card.Reply)
	}

	if running, _ := registry.Status(chatID); running {
		t.Fatal("the turn slot must be released when the turn ends")
	}

	messages, err := repos.AgentChatMessage.FindByChatID(context.Background(), chatID)
	if err != nil {
		t.Fatalf("load messages: %v", err)
	}
	if len(messages) != 2 || messages[0].Sender != "user" || messages[1].Sender != "supervisor" {
		t.Fatalf("expected exactly one user row and one supervisor row, got %d rows: %+v", len(messages), messages)
	}
}
