package agent_test

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/horizonlabs/pulsarfi-backend/src/config"
	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent"
	"github.com/joho/godotenv"
)

// TestOrchestratorLiveConversation exercises the real orchestrator
// (backend/src/service/agent/orchestrator_service.go) against the real
// DeepSeek API and real Postgres — no mocks, per
// docs/plans/agent-orchestration-graph-rebuild.md v2.6's explicit
// requirement. Deliberately a pure-conversation prompt ("path: none"):
// this validates real streaming end to end without opening a Task or
// touching the chain, so it is safe to re-run freely, unlike a prompt that
// would trigger a real on-chain createTask.
//
// Run explicitly: go test ./test/service/agent/... -run TestOrchestratorLiveConversation -v
func TestOrchestratorLiveConversation(t *testing.T) {
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
	if registry.AgentTask == nil {
		t.Fatal("agent task service disabled — check DEEPSEEK_API_KEY/TAVILY_API_KEY/ALCHEMY_RPC_URL/AGENT_WALLET_PRIVATE_KEY/AGENT_TASK_MANAGER_ADDRESS in .env")
	}

	chatID := uuid.New()
	wallet := "0x000000000000000000000000000000LiveTest"

	var deltas []string
	card, err := registry.AgentTask.HandleChatMessage(
		context.Background(),
		chatID,
		wallet,
		"Eh lu kabarnya gimana, hari ini hari apa?",
		agent.AgentEventCallbacks{
			OnSubTask: func(row model.AgentSubTask) {
				fmt.Printf("\n[sub_task] %s / %s: %s\n", row.Agent, row.StepName, row.Reasoning)
			},
			OnSubTaskStarted: func(agentName, stepName, label string) {
				fmt.Printf("\n[sub_task_started] %s / %s: %s\n", agentName, stepName, label)
			},
			OnTextDelta: func(delta string) {
				deltas = append(deltas, delta)
				fmt.Print(delta)
			},
			OnToolCall: func(agentName, toolName, phase string) {
				fmt.Printf("\n[tool_call] %s / %s: %s\n", agentName, toolName, phase)
			},
		},
	)
	if err != nil {
		t.Fatalf("HandleChatMessage failed: %v", err)
	}
	fmt.Println()

	if len(deltas) < 2 {
		t.Fatalf("expected multiple streamed chunks (real token-by-token streaming), got %d — streaming is not actually incremental", len(deltas))
	}
	if card.Reply == "" {
		t.Fatal("final reply is empty")
	}
	if card.TaskID != 0 {
		t.Fatalf("pure conversation should never open a Task, got TaskID=%d", card.TaskID)
	}

	t.Logf("Reply (%d chars, streamed in %d chunks): %s", len(card.Reply), len(deltas), card.Reply)
}
