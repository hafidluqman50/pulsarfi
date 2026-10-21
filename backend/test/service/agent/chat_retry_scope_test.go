package agent_test

import (
	"context"
	"errors"
	agentcore "github.com/horizonlabs/pulsarfi-backend/src/app/core/agent"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/chat"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/checkpoint"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/orchestrator"
	"os"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/horizonlabs/pulsarfi-backend/src/config"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/joho/godotenv"
)

// TestChatRetry_ScopesToTheChatsOwnLastMessage pins the retry contract
// (docs/plans/agent-turn-pipeline-refactor.md): whether a chat can be retried
// is a function of that chat's own last persisted row, nothing else. A failed
// turn deliberately persists no supervisor bubble, so after a failure the
// user's own message is still last — which is what keeps RetryLastMessage (and
// the frontend's Retry button) working for exactly that chat id.
func TestChatRetry_ScopesToTheChatsOwnLastMessage(t *testing.T) {
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
	svc := &chat.ChatService{
		Chats:           repos.AgentChat,
		ChatMessages:    repos.AgentChatMessage,
		Orchestrator:    orch,
		CheckPointStore: checkpoints,
	}

	ctx := context.Background()
	wallet := "0x" + strings.Repeat("d", 40)

	// 1. A chat that does not exist has nothing to retry.
	if _, err := svc.RetryLastMessage(ctx, uuid.New(), wallet); !errors.Is(err, agentcore.ErrNothingToRetry) {
		t.Fatalf("unknown chat: expected ErrNothingToRetry, got %v", err)
	}

	// 2. A chat whose last row is a supervisor message has nothing to retry —
	//    exactly the state a settled turn leaves behind.
	settled := uuid.New()
	if _, err := repos.AgentChat.FindOrCreate(ctx, settled, wallet, "settled chat"); err != nil {
		t.Fatalf("create settled chat: %v", err)
	}
	seed(ctx, t, repos, settled, "user", "beli BUMIP 100000")
	seed(ctx, t, repos, settled, "supervisor", "ok")
	if _, err := svc.RetryLastMessage(ctx, settled, wallet); !errors.Is(err, agentcore.ErrNothingToRetry) {
		t.Fatalf("settled chat: expected ErrNothingToRetry, got %v", err)
	}

	// 3. A chat whose last row is the user's own — the state a *failed* turn
	//    leaves behind, since no failure bubble is persisted — re-runs the turn
	//    and settles with a supervisor reply.
	failed := uuid.New()
	if _, err := repos.AgentChat.FindOrCreate(ctx, failed, wallet, "failed chat"); err != nil {
		t.Fatalf("create failed chat: %v", err)
	}
	seed(ctx, t, repos, failed, "user", "beli BUMIP 100000")

	card, err := svc.RetryLastMessage(ctx, failed, wallet)
	if err != nil {
		t.Fatalf("retry on a user-last chat should re-run the turn, got %v", err)
	}
	if card.Reply != "ok" {
		t.Fatalf("expected the re-run turn's reply, got %q", card.Reply)
	}
	messages, err := repos.AgentChatMessage.FindByChatID(ctx, failed)
	if err != nil {
		t.Fatalf("load chat messages: %v", err)
	}
	if last := messages[len(messages)-1]; last.Sender != "supervisor" {
		t.Fatalf("expected the retry to settle with a supervisor reply, last sender is %q", last.Sender)
	}

	// 4. Retrying a chat never touches another chat's rows: the settled chat
	//    still ends on its own supervisor row.
	if messages, err := repos.AgentChatMessage.FindByChatID(ctx, settled); err != nil {
		t.Fatalf("reload settled chat: %v", err)
	} else if last := messages[len(messages)-1]; last.Sender != "supervisor" {
		t.Fatalf("a retry on another chat changed this chat's last row: %q", last.Sender)
	}
}

func seed(ctx context.Context, t *testing.T, repos *repository.Registry, chatID uuid.UUID, sender, content string) {
	t.Helper()
	if _, err := repos.AgentChatMessage.Create(ctx, repository.AgentChatMessageCreateInput{
		ChatID: chatID, Sender: sender, ContentType: "text", Content: content,
	}); err != nil {
		t.Fatalf("seed %s message: %v", sender, err)
	}
}
