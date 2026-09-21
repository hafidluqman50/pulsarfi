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

// TestCometRegression_ScalpSuite tests the full matrix of on-chain scalping trades:
//  1. Multi-turn Clarifying Questions flow (ambiguous scalp -> questions card -> answer selection -> Arm Card)
//  2. Direct Buy (executor_only, 1,000,000 IDRX, skips Nova analysis)
//  3. Direct Sell (executor_only, 500 tokens, skips Nova analysis)
//  4. Analyze-then-Sell (analyzer_then_executor, 500 tokens, runs Nova analysis before Arm Card)
func TestCometRegression_ScalpSuite(t *testing.T) {
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

	// 1. Clarifying Questions Multi-Turn Intake with Option Selection
	t.Run("ClarifyingQuestions_AnswerSelection_Buy", func(t *testing.T) {
		chatID := uuid.New()
		t.Logf("[ClarifyingQuestions] Turn 1: Sending ambiguous scalp request for BRPT")

		reqBody1, _ := json.Marshal(map[string]any{
			"message": "Mau scalping saham BRPT dong",
		})
		req1, _ := http.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/agent/chats/"+chatID.String()+"/messages", strings.NewReader(string(reqBody1)))
		req1.Header.Set("Content-Type", "application/json")

		w1 := httptest.NewRecorder()
		r.ServeHTTP(w1, req1)

		if w1.Code != http.StatusOK {
			t.Fatalf("[ClarifyingQuestions] Turn 1 expected HTTP 200, got %d: %s", w1.Code, w1.Body.String())
		}

		var resp1 struct {
			Data struct {
				TaskID      int64           `json:"task_id"`
				ContentType string          `json:"content_type"`
				UIComponent *string         `json:"ui_component"`
				Reply       string          `json:"reply"`
				UIProps     json.RawMessage `json:"ui_props"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w1.Body.Bytes(), &resp1); err != nil {
			t.Fatalf("[ClarifyingQuestions] Turn 1 unmarshal error: %v", err)
		}

		if resp1.Data.ContentType != "workflow_card" {
			t.Fatalf("[ClarifyingQuestions] Expected ContentType=workflow_card, got %s", resp1.Data.ContentType)
		}
		if resp1.Data.UIComponent == nil || *resp1.Data.UIComponent != "clarifying_questions" {
			t.Fatalf("[ClarifyingQuestions] Expected UIComponent=clarifying_questions, got %v", resp1.Data.UIComponent)
		}
		// Under Quasar v2.0 clean routing architecture:
		// When intent is recognized at needs_input, a Task is opened immediately in Turn 1
		// with is_actionable = false, and later updated in Turn 2.
		if resp1.Data.TaskID != 0 {
			task1, found, err := repos.AgentTask.FindByID(ctx, resp1.Data.TaskID)
			if err != nil || !found {
				t.Fatalf("[ClarifyingQuestions] Task %d not found in DB: %v", resp1.Data.TaskID, err)
			}
			if task1.IsActionable {
				t.Errorf("[ClarifyingQuestions] Expected Turn 1 task.IsActionable=false (needs_input stage), got true")
			}
		}
		t.Logf("[ClarifyingQuestions] Turn 1 correctly paused with clarifying questions: %s", string(resp1.Data.UIProps))

		// Turn 2: User answers by selecting options
		t.Logf("[ClarifyingQuestions] Turn 2: Sending intake answer selection (side: buy, 1,000,000 IDRX, analyze first)")
		reqBody2, _ := json.Marshal(map[string]any{
			"message": "side: buy\nidrx_cap: 1000000\nconsult_nova: ya, analisis dulu",
			"hidden":  true,
		})
		req2, _ := http.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/agent/chats/"+chatID.String()+"/messages", strings.NewReader(string(reqBody2)))
		req2.Header.Set("Content-Type", "application/json")

		w2 := httptest.NewRecorder()
		r.ServeHTTP(w2, req2)

		if w2.Code != http.StatusOK {
			t.Fatalf("[ClarifyingQuestions] Turn 2 expected HTTP 200, got %d: %s", w2.Code, w2.Body.String())
		}

		var resp2 struct {
			Data struct {
				TaskID int64  `json:"task_id"`
				Reply  string `json:"reply"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w2.Body.Bytes(), &resp2); err != nil {
			t.Fatalf("[ClarifyingQuestions] Turn 2 unmarshal error: %v", err)
		}

		if resp2.Data.TaskID == 0 {
			t.Fatalf("[ClarifyingQuestions] Turn 2 should commit/activate a Task and show Arm Card, got TaskID=0: %s", resp2.Data.Reply)
		}
		if resp1.Data.TaskID != 0 && resp2.Data.TaskID != resp1.Data.TaskID {
			t.Errorf("[ClarifyingQuestions] Expected Turn 2 to update the same TaskID %d, got %d", resp1.Data.TaskID, resp2.Data.TaskID)
		}
		t.Logf("[ClarifyingQuestions] Turn 2 SUCCESS! TaskID=%d", resp2.Data.TaskID)

		task, found, err := repos.AgentTask.FindByID(ctx, resp2.Data.TaskID)
		if err != nil || !found {
			t.Fatalf("[ClarifyingQuestions] Task %d not found in DB: %v", resp2.Data.TaskID, err)
		}
		if !task.IsActionable {
			t.Errorf("[ClarifyingQuestions] Expected task.IsActionable=true")
		}
	})

	// 2. Direct Buy (executor_only, 1,000,000 IDRX, no Nova analysis)
	t.Run("DirectBuy_NoAnalysis", func(t *testing.T) {
		chatID := uuid.New()
		prompt := "Beli BRPT 1000000 IDRX, scalping langsung eksekusi tanpa analisa"
		t.Logf("[DirectBuy] Sending: %s", prompt)

		reqBody, _ := json.Marshal(map[string]any{
			"message": prompt,
		})
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/agent/chats/"+chatID.String()+"/messages", strings.NewReader(string(reqBody)))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("[DirectBuy] Expected HTTP 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp struct {
			Data struct {
				TaskID int64  `json:"task_id"`
				Reply  string `json:"reply"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("[DirectBuy] Unmarshal error: %v", err)
		}

		if resp.Data.TaskID == 0 {
			t.Fatalf("[DirectBuy] Expected TaskID > 0 for direct scalp buy, got 0: %s", resp.Data.Reply)
		}
		t.Logf("[DirectBuy] SUCCESS! TaskID=%d", resp.Data.TaskID)

		task, found, err := repos.AgentTask.FindByID(ctx, resp.Data.TaskID)
		if err != nil || !found {
			t.Fatalf("[DirectBuy] Task %d not found in DB", resp.Data.TaskID)
		}
		if !task.IsActionable {
			t.Errorf("[DirectBuy] Expected task.IsActionable=true")
		}

		var td map[string]any
		if task.TriggerDescription != nil {
			_ = json.Unmarshal([]byte(*task.TriggerDescription), &td)
		}
		if resolvedTicker, _ := td["resolved_ticker"].(string); resolvedTicker != "BRPTP" {
			t.Errorf("[DirectBuy] Expected resolved_ticker=BRPTP, got %s", resolvedTicker)
		}
		if side, _ := td["side"].(string); side != "buy" {
			t.Errorf("[DirectBuy] Expected side=buy, got %s", side)
		}
	})

	// 3. Direct Sell (executor_only, 500 tokens, no Nova analysis)
	t.Run("DirectSell_NoAnalysis", func(t *testing.T) {
		chatID := uuid.New()
		prompt := "Jual 500 token BRPT, scalping langsung eksekusi sekarang tanpa analisa"
		t.Logf("[DirectSell] Sending: %s", prompt)

		reqBody, _ := json.Marshal(map[string]any{
			"message": prompt,
		})
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/agent/chats/"+chatID.String()+"/messages", strings.NewReader(string(reqBody)))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("[DirectSell] Expected HTTP 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp struct {
			Data struct {
				TaskID int64  `json:"task_id"`
				Reply  string `json:"reply"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("[DirectSell] Unmarshal error: %v", err)
		}

		if resp.Data.TaskID == 0 {
			t.Fatalf("[DirectSell] Expected TaskID > 0 for direct scalp sell, got 0: %s", resp.Data.Reply)
		}
		t.Logf("[DirectSell] SUCCESS! TaskID=%d", resp.Data.TaskID)

		task, found, err := repos.AgentTask.FindByID(ctx, resp.Data.TaskID)
		if err != nil || !found {
			t.Fatalf("[DirectSell] Task %d not found in DB", resp.Data.TaskID)
		}
		if !task.IsActionable {
			t.Errorf("[DirectSell] Expected task.IsActionable=true")
		}

		var td map[string]any
		if task.TriggerDescription != nil {
			_ = json.Unmarshal([]byte(*task.TriggerDescription), &td)
		}
		if resolvedTicker, _ := td["resolved_ticker"].(string); resolvedTicker != "BRPTP" {
			t.Errorf("[DirectSell] Expected resolved_ticker=BRPTP, got %s", resolvedTicker)
		}
		if side, _ := td["side"].(string); side != "sell" {
			t.Errorf("[DirectSell] Expected side=sell, got %s", side)
		}
	})

	// 4. Analyze-then-Sell (analyzer_then_executor, 500 tokens, Nova runs first)
	t.Run("AnalyzeThenSell", func(t *testing.T) {
		chatID := uuid.New()
		prompt := "Tolong analisa dulu saham BRPT ya, lalu jual 500 token BRPT scalping sekarang"
		t.Logf("[AnalyzeThenSell] Sending: %s", prompt)

		reqBody, _ := json.Marshal(map[string]any{
			"message": prompt,
		})
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "/api/v1/agent/chats/"+chatID.String()+"/messages", strings.NewReader(string(reqBody)))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("[AnalyzeThenSell] Expected HTTP 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp struct {
			Data struct {
				TaskID int64  `json:"task_id"`
				Reply  string `json:"reply"`
			} `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
			t.Fatalf("[AnalyzeThenSell] Unmarshal error: %v", err)
		}

		if resp.Data.TaskID == 0 {
			t.Fatalf("[AnalyzeThenSell] Expected TaskID > 0 for analyze then sell, got 0: %s", resp.Data.Reply)
		}
		t.Logf("[AnalyzeThenSell] SUCCESS! TaskID=%d", resp.Data.TaskID)

		task, found, err := repos.AgentTask.FindByID(ctx, resp.Data.TaskID)
		if err != nil || !found {
			t.Fatalf("[AnalyzeThenSell] Task %d not found in DB", resp.Data.TaskID)
		}
		if !task.IsActionable {
			t.Errorf("[AnalyzeThenSell] Expected task.IsActionable=true")
		}

		var td map[string]any
		if task.TriggerDescription != nil {
			_ = json.Unmarshal([]byte(*task.TriggerDescription), &td)
		}
		if resolvedTicker, _ := td["resolved_ticker"].(string); resolvedTicker != "BRPTP" {
			t.Errorf("[AnalyzeThenSell] Expected resolved_ticker=BRPTP, got %s", resolvedTicker)
		}
		if side, _ := td["side"].(string); side != "sell" {
			t.Errorf("[AnalyzeThenSell] Expected side=sell, got %s", side)
		}

		// Verify Nova executed gather_evidence
		subTasks, err := repos.AgentSubTask.FindByTaskID(ctx, resp.Data.TaskID)
		if err != nil || len(subTasks) == 0 {
			t.Fatalf("[AnalyzeThenSell] No subtasks found in database for task %d", resp.Data.TaskID)
		}
		var foundNovaGather bool
		for _, st := range subTasks {
			if st.Agent == "analyzer" && st.StepName == "gather_evidence" {
				foundNovaGather = true
				break
			}
		}
		if !foundNovaGather {
			t.Errorf("[AnalyzeThenSell] Expected analyzer/gather_evidence subtask row in database")
		}
	})
}
