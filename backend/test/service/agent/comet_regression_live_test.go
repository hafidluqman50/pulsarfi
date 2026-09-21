package agent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/horizonlabs/pulsarfi-backend/src/config"
	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service"
	agentsvc "github.com/horizonlabs/pulsarfi-backend/src/service/agent"
	"github.com/joho/godotenv"
)

// TestCometRegression_BRPT_And_BMRI validates that the analyzer_then_executor
// pipeline (Nova gathering evidence -> Comet primed for execution) experiences
// zero regression for BRPT and BMRI using the live trader wallet.
func TestCometRegression_BRPT_And_BMRI(t *testing.T) {
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

	traderWallet := "0xd8bf50c157a79260c77b25f89ef713e6c3feda6f"
	ctx := context.Background()

	testCases := []struct {
		name           string
		ticker         string
		expectedTicker string
		prompt         string
	}{
		{
			name:           "BRPT",
			ticker:         "BRPT",
			expectedTicker: "BRPTP",
			prompt:         "Tolong analisa saham BRPT dulu ya, lalu beli 50000 IDRX, scalping sekarang",
		},
		{
			name:           "BMRI",
			ticker:         "BMRI",
			expectedTicker: "BMRIP",
			prompt:         "Coba analisa saham BMRI dulu ya, lalu beli 100000 IDRX, scalping sekarang",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			chatID := uuid.New()
			t.Logf("[%s] Sending prompt: %q for trader wallet: %s", tc.name, tc.prompt, traderWallet)

			var subTasksRecorded []string
			var novaEvidenceText string
			card, err := registry.AgentChat.HandleChatMessage(
				ctx,
				chatID,
				traderWallet,
				tc.prompt,
				false,
				agentsvc.AgentEventCallbacks{
					OnSubTask: func(st model.AgentSubTask) {
						subTasksRecorded = append(subTasksRecorded, fmt.Sprintf("%s/%s", st.Agent, st.StepName))
						if st.Agent == "analyzer" && st.StepName == "gather_evidence" {
							novaEvidenceText = st.Reasoning
						}
						t.Logf("[%s subtask] %s/%s status=%s", tc.name, st.Agent, st.StepName, st.Status)
					},
					OnToolCall: func(agentName, toolName, phase string) {
						t.Logf("[%s tool] %s called %s (%s)", tc.name, agentName, toolName, phase)
					},
				},
			)
			if err != nil {
				t.Fatalf("[%s] HandleChatMessage failed: %v", tc.name, err)
			}

			if card.TaskID == 0 {
				t.Fatalf("[%s] Expected actionable task to produce confirmation card with TaskID > 0, got TaskID=0. Reply: %s", tc.name, card.Reply)
			}
			t.Logf("[%s] Confirmation card produced: TaskID=%d, Reply=%s", tc.name, card.TaskID, card.Reply)

			// Verify Task row in database
			task, found, err := repos.AgentTask.FindByID(ctx, card.TaskID)
			if err != nil || !found {
				t.Fatalf("[%s] Task %d not found in database: %v", tc.name, card.TaskID, err)
			}
			if !task.IsActionable {
				t.Errorf("[%s] Expected task to be actionable, got false", tc.name)
			}

			// Verify trigger description has correct ticker and shape
			if task.TriggerDescription == nil {
				t.Fatalf("[%s] Expected trigger description, got nil", tc.name)
			}
			var td map[string]any
			if err := json.Unmarshal([]byte(*task.TriggerDescription), &td); err != nil {
				t.Fatalf("[%s] Failed to parse trigger description: %v", tc.name, err)
			}
			if resolvedTicker, _ := td["resolved_ticker"].(string); resolvedTicker != tc.expectedTicker {
				t.Errorf("[%s] Expected resolved_ticker=%s, got %s", tc.name, tc.expectedTicker, resolvedTicker)
			}
			if shape, _ := td["shape"].(string); shape != "scalp" {
				t.Errorf("[%s] Expected shape=scalp, got %s", tc.name, shape)
			}
			if side, _ := td["side"].(string); side != "buy" {
				t.Errorf("[%s] Expected side=buy, got %s", tc.name, side)
			}

			// Verify Nova's gather_evidence step was recorded
			hasNovaEvidence := false
			for _, st := range subTasksRecorded {
				if st == "analyzer/gather_evidence" {
					hasNovaEvidence = true
					break
				}
			}
			if !hasNovaEvidence {
				t.Errorf("[%s] Expected analyzer/gather_evidence subtask to be executed before Comet", tc.name)
			}
			if strings.TrimSpace(novaEvidenceText) == "" {
				t.Errorf("[%s] Nova evidence reasoning is empty", tc.name)
			} else {
				limit := 120
				if len(novaEvidenceText) < limit {
					limit = len(novaEvidenceText)
				}
				t.Logf("[%s] Nova gathered evidence cleanly: %s...", tc.name, novaEvidenceText[:limit])
			}

			// Verify chat checkpoint was saved in Postgres for the pause before Comet execute
			cpStore, ok := registry.AgentChat.CheckPointStore.(agentsvc.ExtendedCheckPointStore)
			if ok {
				has, err := cpStore.Has(ctx, chatID.String())
				if err != nil || !has {
					t.Errorf("[%s] Expected Eino checkpoint in Postgres before execute pause, has=%v, err=%v", tc.name, has, err)
				} else {
					t.Logf("[%s] Verified Eino checkpoint paused at 'execute' node waiting for on-chain Arm approval", tc.name)
				}
			}
		})
	}
}
