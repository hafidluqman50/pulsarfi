package agent_test

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/horizonlabs/pulsarfi-backend/src/config"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service"
	agentsvc "github.com/horizonlabs/pulsarfi-backend/src/service/agent"
	"github.com/joho/godotenv"
)

func TestTaskService_ExecuteTaskValidation(t *testing.T) {
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

	if registry.AgentTask == nil {
		t.Fatal("expected registry.AgentTask to be configured")
	}

	ctx := context.Background()
	ownerWallet := "0x" + strings.Repeat("b", 40)
	otherWallet := "0x" + strings.Repeat("c", 40)

	// 1. Create a non-actionable task
	nonActionableTask, err := repos.AgentTask.Create(ctx, repository.AgentTaskCreateInput{
		WalletAddress: ownerWallet,
		IsActionable:  false,
		Summary:       "Non-actionable task",
	})
	if err != nil {
		t.Fatalf("create non-actionable task: %v", err)
	}

	// 2. Create an actionable but un-armed task
	unarmedTask, err := repos.AgentTask.Create(ctx, repository.AgentTaskCreateInput{
		WalletAddress: ownerWallet,
		IsActionable:  true,
		Summary:       "Actionable unarmed task",
	})
	if err != nil {
		t.Fatalf("create unarmed task: %v", err)
	}

	// 3. Create an already executed task
	onChainID := int64(999999)
	now := time.Now()
	executedTask, err := repos.AgentTask.Create(ctx, repository.AgentTaskCreateInput{
		WalletAddress: ownerWallet,
		IsActionable:  true,
		Summary:       "Executed task",
	})
	if err != nil {
		t.Fatalf("create executed task: %v", err)
	}
	_ = repos.AgentTask.SetOnChainTaskID(ctx, executedTask.ID, onChainID)
	_ = repos.AgentTask.SetArmedAt(ctx, executedTask.ID, now)
	_ = repos.AgentTask.SetStatus(ctx, executedTask.ID, "executed")
	_ = repos.AgentTask.SetExecutedAt(ctx, executedTask.ID, now)

	// Test 1: Task Not Found
	_, err = registry.AgentTask.ExecuteTask(ctx, 999999999, ownerWallet)
	if err != agentsvc.ErrTaskNotFound {
		t.Fatalf("expected ErrTaskNotFound, got %v", err)
	}

	// Test 2: Wallet Mismatch
	_, err = registry.AgentTask.ExecuteTask(ctx, unarmedTask.ID, otherWallet)
	if err != agentsvc.ErrWalletMismatch {
		t.Fatalf("expected ErrWalletMismatch, got %v", err)
	}

	// Test 3: Task Not Actionable
	_, err = registry.AgentTask.ExecuteTask(ctx, nonActionableTask.ID, ownerWallet)
	if err != agentsvc.ErrTaskNotActionable {
		t.Fatalf("expected ErrTaskNotActionable, got %v", err)
	}

	// Test 4: Task Not Armed
	_, err = registry.AgentTask.ExecuteTask(ctx, unarmedTask.ID, ownerWallet)
	if err != agentsvc.ErrTaskNotArmed {
		t.Fatalf("expected ErrTaskNotArmed, got %v", err)
	}

	// Test 5: Already Executed Task returns cleanly
	res, err := registry.AgentTask.ExecuteTask(ctx, executedTask.ID, ownerWallet)
	if err != nil {
		t.Fatalf("expected nil error for already executed task, got %v", err)
	}
	if res.Status != "executed" {
		t.Fatalf("expected status executed, got %s", res.Status)
	}
}
