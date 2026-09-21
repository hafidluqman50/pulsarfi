package agent_test

import (
	"context"
	"fmt"
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
	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent"
	"github.com/joho/godotenv"
)

func TestLiveNewsQueryEndpoint(t *testing.T) {
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
	if registry.AgentChat == nil {
		t.Fatal("agent chat service disabled")
	}

	chatID := uuid.New()
	wallet := "0xd8bf50c157a79260c77b25f89ef713e6c3feda6f"

	card, err := registry.AgentChat.HandleChatMessage(
		context.Background(),
		chatID,
		wallet,
		"Check berita IHSG hari ini dong",
		false,
		agent.AgentEventCallbacks{
			OnSubTask: func(row model.AgentSubTask) {
				fmt.Printf("\n[sub_task] %s / %s: %s\n", row.Agent, row.StepName, row.Reasoning)
			},
			OnSubTaskStarted: func(agentName, stepName, label string) {
				fmt.Printf("\n[sub_task_started] %s / %s: %s\n", agentName, stepName, label)
			},
			OnSubTaskFailed: func(agentName, stepName, reason string) {
				fmt.Printf("\n[sub_task_failed] %s / %s: %s\n", agentName, stepName, reason)
			},
			OnToolCall: func(agentName, toolName, phase string) {
				fmt.Printf("\n[tool_call] %s / %s: %s\n", agentName, toolName, phase)
			},
			OnTextDelta: func(delta string) {
				fmt.Print(delta)
			},
		},
	)
	if err != nil {
		t.Fatalf("HandleChatMessage returned ERROR: %v", err)
	}
	fmt.Printf("\nCard Result: content_type=%s, task_id=%d, reply=%s\n", card.ContentType, card.TaskID, card.Reply)
}

func TestLiveNewsQueryHTTPEndpoint(t *testing.T) {
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
	if registry.AgentChat == nil {
		t.Fatal("agent chat service disabled")
	}

	agentHandler.ConfigureServices(registry)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	wallet := "0xd8bf50c157a79260c77b25f89ef713e6c3feda6f"
	r.Use(func(c *gin.Context) {
		c.Set("user", usermw.Claims{
			WalletAddress: wallet,
			Role:          "user",
		})
		c.Next()
	})
	r.POST("/api/v1/agent/chats/:id/messages", agentHandler.PostChatMessageHandler)

	chatID := uuid.New()
	reqBody := `{"message":"Check berita IHSG hari ini dong"}`
	req, err := http.NewRequest("POST", "/api/v1/agent/chats/"+chatID.String()+"/messages", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected HTTP 200, got HTTP %d: %s", w.Code, w.Body.String())
	}
	t.Logf("HTTP endpoint returned %d OK with response: %s", w.Code, w.Body.String())
}

