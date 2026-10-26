package agent_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/horizonlabs/pulsarfi-backend/src/config"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/joho/godotenv"
)

func newTestRepos(t *testing.T) *repository.Registry {
	t.Helper()
	_ = godotenv.Load("../../../.env")
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Fatal("DATABASE_URL is required for the instant execution integration test")
	}
	db, err := config.NewDatabase(dsn)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	return repository.NewRegistry(db)
}

func TestInstantExecution_ClaimAndDecide(t *testing.T) {
	repos := newTestRepos(t)
	ctx := context.Background()

	wallet := "0x0000000000000000000000000000000000000001"
	trigger := `{"side":"buy","resolved_ticker":"BRPTP","confirmed_budget_idrx":"1000","card":{"arm_title_ready":"ready"}}`
	task, err := repos.AgentTask.Create(ctx, repository.AgentTaskCreateInput{
		WalletAddress:      wallet,
		IsActionable:       true,
		Summary:            "integration test task",
		TriggerDescription: &trigger,
	})
	if err != nil {
		t.Fatalf("create task: %v", err)
	}
	t.Cleanup(func() {
		repos.AgentTask.DB.Exec("DELETE FROM agent_tasks WHERE id = ?", task.ID)
	})

	claimed, err := repos.AgentTask.ClaimForExecution(ctx, task.ID, wallet)
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if claimed.Status != "armed" {
		t.Fatalf("claimed status = %q, want armed", claimed.Status)
	}

	if _, err := repos.AgentTask.ClaimForExecution(ctx, task.ID, wallet); !errors.Is(err, repository.ErrExecutionClaimLost) {
		t.Fatalf("second claim error = %v, want ErrExecutionClaimLost", err)
	}

	if err := repos.AgentTask.SetExecutionDecide(ctx, task.ID, 42, 4242, "0xabc", 0); err != nil {
		t.Fatalf("set execution decide: %v", err)
	}

	stored, found, err := repos.AgentTask.FindByID(ctx, task.ID)
	if err != nil || !found {
		t.Fatalf("reload task: found=%v err=%v", found, err)
	}
	var payload map[string]any
	if err := json.Unmarshal([]byte(*stored.TriggerDescription), &payload); err != nil {
		t.Fatalf("parse trigger: %v", err)
	}
	if payload["resolved_ticker"] != "BRPTP" {
		t.Fatalf("resolved_ticker was dropped: %v", payload)
	}
	decide, ok := payload["execution_decide"].(map[string]any)
	if !ok {
		t.Fatalf("execution_decide missing: %v", payload)
	}
	if decide["on_chain_id"] != float64(4242) {
		t.Fatalf("on_chain_id = %v, want 4242", decide["on_chain_id"])
	}
	if decide["reasoning_hash"] != "0xabc" {
		t.Fatalf("reasoning_hash = %v, want 0xabc", decide["reasoning_hash"])
	}
}
