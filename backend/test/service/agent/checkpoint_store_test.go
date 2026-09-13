package agent_test

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/cloudwego/eino/compose"
	"github.com/google/uuid"
	"github.com/horizonlabs/pulsarfi-backend/src/config"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	agentsvc "github.com/horizonlabs/pulsarfi-backend/src/service/agent"
	"github.com/joho/godotenv"
)

func TestPostgresCheckPointStore_CRUD(t *testing.T) {
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
	store := agentsvc.NewPostgresCheckPointStore(repos.AgentCheckpoint)
	ctx := context.Background()

	cpID := "test-cp-" + uuid.New().String()
	testPayload := []byte("checkpoint_binary_payload_v1")

	// 1. Clean up before & after test
	defer func() {
		_ = store.Delete(ctx, cpID)
	}()

	// 2. Verify non-existent returns false
	got, found, err := store.Get(ctx, cpID)
	if err != nil {
		t.Fatalf("unexpected error getting non-existent checkpoint: %v", err)
	}
	if found {
		t.Fatalf("expected found=false for non-existent checkpoint, got true")
	}
	if got != nil {
		t.Fatalf("expected nil data for non-existent checkpoint, got %v", got)
	}

	has, err := store.Has(ctx, cpID)
	if err != nil {
		t.Fatalf("unexpected error checking Has for non-existent: %v", err)
	}
	if has {
		t.Fatalf("expected has=false for non-existent, got true")
	}

	// 3. Set checkpoint
	if err := store.Set(ctx, cpID, testPayload); err != nil {
		t.Fatalf("failed to set checkpoint: %v", err)
	}

	// 4. Verify Has and Get
	has, err = store.Has(ctx, cpID)
	if err != nil {
		t.Fatalf("Has error: %v", err)
	}
	if !has {
		t.Fatalf("expected has=true after Set, got false")
	}

	got, found, err = store.Get(ctx, cpID)
	if err != nil {
		t.Fatalf("Get error: %v", err)
	}
	if !found {
		t.Fatalf("expected found=true after Set, got false")
	}
	if !bytes.Equal(got, testPayload) {
		t.Fatalf("expected payload %q, got %q", string(testPayload), string(got))
	}

	// 5. Upsert / Overwrite with new payload
	updatedPayload := []byte("checkpoint_binary_payload_v2_updated")
	if err := store.Set(ctx, cpID, updatedPayload); err != nil {
		t.Fatalf("failed to update checkpoint: %v", err)
	}

	got, found, err = store.Get(ctx, cpID)
	if err != nil {
		t.Fatalf("Get error after update: %v", err)
	}
	if !found {
		t.Fatalf("expected found=true after update, got false")
	}
	if !bytes.Equal(got, updatedPayload) {
		t.Fatalf("expected updated payload %q, got %q", string(updatedPayload), string(got))
	}

	// 6. Delete checkpoint
	if err := store.Delete(ctx, cpID); err != nil {
		t.Fatalf("Delete error: %v", err)
	}

	has, err = store.Has(ctx, cpID)
	if err != nil {
		t.Fatalf("Has error after delete: %v", err)
	}
	if has {
		t.Fatalf("expected has=false after delete, got true")
	}

	got, found, err = store.Get(ctx, cpID)
	if err != nil {
		t.Fatalf("Get error after delete: %v", err)
	}
	if found {
		t.Fatalf("expected found=false after delete, got true")
	}
	if got != nil {
		t.Fatalf("expected nil data after delete, got %v", got)
	}
}

func TestPostgresCheckPointStore_EinoGraphCompile(t *testing.T) {
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
	store := agentsvc.NewPostgresCheckPointStore(repos.AgentCheckpoint)
	ctx := context.Background()

	// Verify that an Eino Graph compiles cleanly with our PostgresCheckPointStore
	g := compose.NewGraph[string, string]()
	err = g.AddLambdaNode("echo", compose.InvokableLambda(func(ctx context.Context, in string) (string, error) {
		return "echo:" + in, nil
	}))
	if err != nil {
		t.Fatalf("add echo node: %v", err)
	}

	if err := g.AddEdge(compose.START, "echo"); err != nil {
		t.Fatalf("add start edge: %v", err)
	}
	if err := g.AddEdge("echo", compose.END); err != nil {
		t.Fatalf("add end edge: %v", err)
	}

	runnable, err := g.Compile(ctx, compose.WithCheckPointStore(store))
	if err != nil {
		t.Fatalf("compile graph with PostgresCheckPointStore: %v", err)
	}

	cpID := "eino-test-" + uuid.New().String()
	defer func() {
		_ = store.Delete(ctx, cpID)
	}()

	out, err := runnable.Invoke(ctx, "hello", compose.WithCheckPointID(cpID))
	if err != nil {
		t.Fatalf("invoke runnable with checkpoint: %v", err)
	}
	if out != "echo:hello" {
		t.Fatalf("expected 'echo:hello', got %q", out)
	}
}

func TestPostgresCheckPointStore_StatefulInterruptResume(t *testing.T) {
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
	store := agentsvc.NewPostgresCheckPointStore(repos.AgentCheckpoint)
	ctx := context.Background()

	g := compose.NewGraph[string, string]()
	err = g.AddLambdaNode("step", compose.InvokableLambda(func(ctx context.Context, in string) (string, error) {
		if in == "ask_confirmation" {
			return "", compose.StatefulInterrupt(ctx, "needs_confirmation", "pending_trade_details")
		}
		wasInterrupted, hasState, state := compose.GetInterruptState[string](ctx)
		if wasInterrupted && hasState {
			return "resumed_with_" + state + "_and_input_" + in, nil
		}
		return "processed_" + in, nil
	}))
	if err != nil {
		t.Fatalf("add lambda node: %v", err)
	}
	if err := g.AddEdge(compose.START, "step"); err != nil {
		t.Fatalf("add start edge: %v", err)
	}
	if err := g.AddEdge("step", compose.END); err != nil {
		t.Fatalf("add end edge: %v", err)
	}

	runnable, err := g.Compile(ctx, compose.WithCheckPointStore(store))
	if err != nil {
		t.Fatalf("compile graph: %v", err)
	}

	cpID := "eino-interrupt-" + uuid.New().String()
	defer func() {
		_ = store.Delete(ctx, cpID)
	}()

	// 1. First turn: asks confirmation -> stateful interrupt
	_, err = runnable.Invoke(ctx, "ask_confirmation", compose.WithCheckPointID(cpID))
	if err == nil {
		t.Fatal("expected interrupt error, got nil")
	}
	_, ok := compose.ExtractInterruptInfo(err)
	if !ok {
		t.Fatalf("expected ExtractInterruptInfo=true, err: %v", err)
	}

	// 2. Assert checkpoint row exists in Postgres
	has, err := store.Has(ctx, cpID)
	if err != nil {
		t.Fatalf("check has error: %v", err)
	}
	if !has {
		t.Fatalf("expected checkpoint to be saved in Postgres for %s, but found none", cpID)
	}

	// 3. Second turn: resume from checkpoint with user response
	result, err := runnable.Invoke(ctx, "confirmed_answer", compose.WithCheckPointID(cpID))
	if err != nil {
		t.Fatalf("expected successful resume from checkpoint, got: %v", err)
	}
	expected := "resumed_with_pending_trade_details_and_input_"
	if result != expected {
		t.Fatalf("expected %q, got %q", expected, result)
	}
}
