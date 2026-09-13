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

func TestChatServiceDecomposition(t *testing.T) {
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
	if registry.AgentTask == nil {
		t.Fatal("expected registry.AgentTask to be configured")
	}

	ctx := context.Background()
	testWallet := "0x" + strings.Repeat("a", 40)
	chatID := uuid.New()

	// 1. Verify Create and List chats via ChatService
	chat, err := repos.AgentChat.FindOrCreate(ctx, chatID, testWallet, "Phase 1 Test Thread")
	if err != nil {
		t.Fatalf("create chat thread: %v", err)
	}
	if chat.ID != chatID {
		t.Fatalf("expected chat ID %v, got %v", chatID, chat.ID)
	}

	chats, err := registry.AgentChat.ListChats(ctx, testWallet)
	if err != nil {
		t.Fatalf("list chats: %v", err)
	}
	found := false
	for _, c := range chats {
		if c.ID == chatID {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected chat %v in list, but not found", chatID)
	}

	// 2. Verify GetChatMessages via ChatService
	msg, err := repos.AgentChatMessage.Create(ctx, repository.AgentChatMessageCreateInput{
		ChatID:      chatID,
		Sender:      "user",
		ContentType: "text",
		Content:     "Hello from phase 1 test",
	})
	if err != nil {
		t.Fatalf("create message: %v", err)
	}

	messages, err := registry.AgentChat.GetChatMessages(ctx, chatID, testWallet)
	if err != nil {
		t.Fatalf("get chat messages: %v", err)
	}
	if len(messages) == 0 {
		t.Fatalf("expected at least 1 message, got 0")
	}
	if messages[len(messages)-1].ID != msg.ID {
		t.Errorf("expected message ID %v, got %v", msg.ID, messages[len(messages)-1].ID)
	}

	// 3. Verify Wallet mismatch protection
	_, err = registry.AgentChat.GetChatMessages(ctx, chatID, "0x"+strings.Repeat("b", 40))
	if err != agentsvc.ErrWalletMismatch {
		t.Errorf("expected ErrWalletMismatch, got %v", err)
	}
}
