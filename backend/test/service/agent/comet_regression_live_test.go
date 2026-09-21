package agent_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/horizonlabs/pulsarfi-backend/src/config"
	agentHandler "github.com/horizonlabs/pulsarfi-backend/src/http/handlers/agent"
	usermw "github.com/horizonlabs/pulsarfi-backend/src/http/middleware/user"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service"
	agentsvc "github.com/horizonlabs/pulsarfi-backend/src/service/agent"
	"github.com/joho/godotenv"
)

// TestCometRegression_BRPT_And_BMRI_HttpEndpoint tests the full pipeline
// through the real Gin HTTP router endpoint (POST /api/v1/agent/chats/:id/messages)
// with the live trader wallet to guarantee zero regression on HTTP status codes,
// serialization, Nova evidence gathering, and Arm confirmation card generation.
func TestCometRegression_BRPT_And_BMRI_HttpEndpoint(t *testing.T) {
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

	agentHandler.ConfigureServices(registry)

	traderWallet := "0xd8bf50c157a79260c77b25f89ef713e6c3feda6f"
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("user", usermw.Claims{
			WalletAddress: traderWallet,
			Role:          "user",
		})
		c.Next()
	})
	r.POST("/api/v1/agent/chats/:id/messages", agentHandler.PostChatMessageHandler)

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
			t.Logf("[%s] Sending HTTP POST to /api/v1/agent/chats/%s/messages for trader wallet: %s", tc.name, chatID, traderWallet)

			reqPayload, _ := json.Marshal(map[string]string{
				"message": tc.prompt,
			})
			req, err := http.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/agent/chats/"+chatID.String()+"/messages", strings.NewReader(string(reqPayload)))
			if err != nil {
				t.Fatalf("[%s] Failed to build HTTP request: %v", tc.name, err)
			}
			req.Header.Set("Content-Type", "application/json")

			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != http.StatusOK {
				t.Fatalf("[%s] Expected HTTP 200 OK from endpoint, got HTTP %d: %s", tc.name, w.Code, w.Body.String())
			}

			var resp struct {
				Data struct {
					TaskID int64  `json:"task_id"`
					Reply  string `json:"reply"`
				} `json:"data"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
				t.Fatalf("[%s] Failed to unmarshal endpoint response: %v, raw: %s", tc.name, err, w.Body.String())
			}

			if resp.Data.TaskID == 0 {
				t.Fatalf("[%s] Expected actionable task to produce confirmation card with TaskID > 0, got TaskID=0. Reply: %s", tc.name, resp.Data.Reply)
			}
			t.Logf("[%s] HTTP 200 OK! TaskID=%d, Reply=%s", tc.name, resp.Data.TaskID, resp.Data.Reply)

			// Verify Task row in database
			task, found, err := repos.AgentTask.FindByID(ctx, resp.Data.TaskID)
			if err != nil || !found {
				t.Fatalf("[%s] Task %d not found in database: %v", tc.name, resp.Data.TaskID, err)
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

			// Verify subtasks recorded by Nova in database
			subTasks, err := repos.AgentSubTask.FindByTaskID(ctx, resp.Data.TaskID)
			if err != nil || len(subTasks) == 0 {
				t.Fatalf("[%s] No subtasks found in database for task %d", tc.name, resp.Data.TaskID)
			}
			var foundNovaGather bool
			for _, st := range subTasks {
				t.Logf("[%s DB SubTask] %s/%s status=%s", tc.name, st.Agent, st.StepName, st.Status)
				if st.Agent == "analyzer" && st.StepName == "gather_evidence" {
					foundNovaGather = true
					if strings.TrimSpace(st.Reasoning) == "" {
						t.Errorf("[%s] Nova gather_evidence reasoning in DB is empty", tc.name)
					} else {
						t.Logf("[%s] Nova evidence reasoning: %s...", tc.name, st.Reasoning[:min(100, len(st.Reasoning))])
					}
				}
			}
			if !foundNovaGather {
				t.Errorf("[%s] Expected analyzer/gather_evidence subtask row in database", tc.name)
			}

			// Verify Eino checkpoint paused before Comet execute
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

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
