package agent_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/horizonlabs/pulsarfi-backend/src/auth"
	"github.com/horizonlabs/pulsarfi-backend/src/config"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	"github.com/horizonlabs/pulsarfi-backend/src/service"
	agentsvc "github.com/horizonlabs/pulsarfi-backend/src/service/agent"
	"github.com/horizonlabs/pulsarfi-backend/src/service/realtime"
	"github.com/joho/godotenv"
)

// Live run of the new Quasar-as-agent flow against the configured database,
// real DeepSeek, and the real chain client: it writes real chats, Tasks and
// on-chain Sub Tasks for the demo wallet. It therefore only runs when
// LIVE_AGENT_FLOW=1 is set on purpose, and it stops at the Arm Card: arming
// needs the wallet's own signature, so no trade is ever executed here.
const liveDemoWallet = "0xd8bf50c157a79260c77b25f89ef713e6c3feda6f"

type captureConn struct {
	incoming chan []byte
	mu       sync.Mutex
	frames   []string
}

func newCaptureConn() *captureConn { return &captureConn{incoming: make(chan []byte, 16)} }

func (c *captureConn) ReadMessage() (int, []byte, error) {
	message, ok := <-c.incoming
	if !ok {
		return 0, nil, io.EOF
	}
	return 1, message, nil
}

func (c *captureConn) WriteMessage(messageType int, data []byte) error {
	if messageType == 1 {
		c.mu.Lock()
		c.frames = append(c.frames, string(data))
		c.mu.Unlock()
	}
	return nil
}

func (c *captureConn) SetReadLimit(int64)                {}
func (c *captureConn) SetReadDeadline(time.Time) error   { return nil }
func (c *captureConn) SetWriteDeadline(time.Time) error  { return nil }
func (c *captureConn) SetPongHandler(func(string) error) {}
func (c *captureConn) Close() error                      { return nil }

func (c *captureConn) subscribe(topic string) {
	payload, _ := json.Marshal(map[string]any{"type": "subscribe", "topics": []string{topic}})
	c.incoming <- payload
	time.Sleep(150 * time.Millisecond)
}

func (c *captureConn) drain() []string {
	time.Sleep(400 * time.Millisecond)
	c.mu.Lock()
	defer c.mu.Unlock()
	frames := c.frames
	c.frames = nil
	return frames
}

type liveSummary struct {
	replyDeltas int
	reply       strings.Builder
	thinking    map[string]int
	reasoning   map[string]int
	toolCalls   []string
	subTasks    []string
	status      []string
	eventTypes  map[string]int
}

func summarize(frames []string) liveSummary {
	summary := liveSummary{thinking: map[string]int{}, eventTypes: map[string]int{}}
	for _, frame := range frames {
		var outer struct {
			Data struct {
				Type string          `json:"type"`
				Data json.RawMessage `json:"data"`
			} `json:"data"`
		}
		if json.Unmarshal([]byte(frame), &outer) != nil || outer.Data.Type == "" {
			continue
		}
		summary.eventTypes[outer.Data.Type]++
		switch outer.Data.Type {
		case "reply_delta":
			var payload struct {
				Delta string `json:"delta"`
			}
			_ = json.Unmarshal(outer.Data.Data, &payload)
			summary.replyDeltas++
			summary.reply.WriteString(payload.Delta)
		case "thinking":
			var payload struct {
				Agent string `json:"agent"`
			}
			_ = json.Unmarshal(outer.Data.Data, &payload)
			summary.thinking[payload.Agent]++
		case "status":
			var payload struct {
				Text string `json:"text"`
			}
			_ = json.Unmarshal(outer.Data.Data, &payload)
			summary.status = append(summary.status, payload.Text)
		case "tool_call":
			var payload struct{ Agent, Tool, Phase string }
			_ = json.Unmarshal(outer.Data.Data, &payload)
			summary.toolCalls = append(summary.toolCalls, fmt.Sprintf("%s/%s:%s", payload.Agent, payload.Tool, payload.Phase))
		case "sub_tasks":
			var entries []struct {
				Agent    string `json:"agent"`
				StepName string `json:"step_name"`
				Status   string `json:"status"`
			}
			_ = json.Unmarshal(outer.Data.Data, &entries)
			summary.subTasks = summary.subTasks[:0]
			for _, entry := range entries {
				summary.subTasks = append(summary.subTasks, fmt.Sprintf("%s/%s=%s", entry.Agent, entry.StepName, entry.Status))
			}
		}
	}
	return summary
}

func logSummary(t *testing.T, label string, card agentsvc.WorkflowCard, summary liveSummary) {
	t.Helper()
	t.Logf("[%s] task_id=%d content_type=%s ui_component=%v", label, card.TaskID, card.ContentType, card.UIComponent)
	t.Logf("[%s] reply: %s", label, strings.TrimSpace(card.Reply))
	t.Logf("[%s] events: %v | reply_delta chunks=%d | thinking=%v", label, summary.eventTypes, summary.replyDeltas, summary.thinking)
	t.Logf("[%s] tool_calls: %v", label, summary.toolCalls)
	t.Logf("[%s] sub_tasks (last snapshot): %v", label, summary.subTasks)
}

var intakeAnswers = map[string]string{
	"shape": "scalp", "ticker": "BBCAP", "side": "beli", "idrx_cap": "1000000",
	"portfolio_share": "100%", "consult_nova": "ya, analisis dulu", "horizon": "7 hari",
	"strategy": "sekaligus", "exit_policy": "tutup posisi",
}

func answerFor(card agentsvc.WorkflowCard) string {
	var payload struct {
		Questions []struct{ Key, Question string } `json:"questions"`
	}
	_ = json.Unmarshal(card.UIProps, &payload)
	var lines []string
	for _, question := range payload.Questions {
		lines = append(lines, fmt.Sprintf("%s: %s", question.Question, intakeAnswers[question.Key]))
	}
	return strings.Join(lines, "\n")
}

func TestLiveAgentAsTool_Scenarios(t *testing.T) {
	if os.Getenv("LIVE_AGENT_FLOW") != "1" {
		t.Skip("LIVE_AGENT_FLOW=1 not set: this test writes to the configured database and chain")
	}
	if err := godotenv.Load("../../../.env"); err != nil {
		t.Logf("no .env loaded (%v), relying on already-exported environment", err)
	}
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set")
	}
	db, err := config.NewDatabase(databaseURL)
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	repos := repository.NewRegistry(db)
	registry := service.NewRegistry(service.Config{Repos: repos})
	if registry.AgentChat == nil {
		t.Fatal("agent chat service disabled: check the DeepSeek, Alchemy, agent wallet and task manager settings in .env")
	}

	ctx := context.Background()
	conn := newCaptureConn()
	go realtime.Default().Serve(conn, &auth.Claims{WalletAddress: liveDemoWallet, Role: "user"})
	defer close(conn.incoming)

	say := func(chatID uuid.UUID, message string) (agentsvc.WorkflowCard, liveSummary) {
		card, err := registry.AgentChat.HandleChatMessage(ctx, chatID, liveDemoWallet, message, false)
		if err != nil {
			t.Fatalf("HandleChatMessage(%q): %v", message, err)
		}
		return card, summarize(conn.drain())
	}

	t.Run("A greeting opens no task", func(t *testing.T) {
		chatID := uuid.New()
		conn.subscribe(agentsvc.ChatStreamTopic(chatID))
		card, summary := say(chatID, "Halo, apa kabar?")
		logSummary(t, "A", card, summary)
		if card.TaskID != 0 {
			t.Errorf("a greeting must not open a Task, got %d", card.TaskID)
		}
		if strings.TrimSpace(card.Reply) == "" || summary.replyDeltas == 0 {
			t.Errorf("expected a streamed reply, got reply=%q chunks=%d", card.Reply, summary.replyDeltas)
		}
	})

	t.Run("B information goes through a task and Nova", func(t *testing.T) {
		chatID := uuid.New()
		conn.subscribe(agentsvc.ChatStreamTopic(chatID))
		card, summary := say(chatID, "Ambilin chart IHSG dong")
		logSummary(t, "B", card, summary)
		if card.TaskID == 0 {
			t.Errorf("an information request must still open an on-chain Task")
		}
		if summary.thinking["analyzer"] == 0 {
			t.Errorf("expected Nova's thinking to stream, got %v", summary.thinking)
		}
		if card.ContentType != "chart" && card.ContentType != "news" {
			t.Errorf("expected a chart or news card, got %q", card.ContentType)
		}
	})

	t.Run("D trade: questions card, answer, Nova, Arm Card", func(t *testing.T) {
		chatID := uuid.New()
		conn.subscribe(agentsvc.ChatStreamTopic(chatID))

		card, summary := say(chatID, "Mau beli BBCA")
		logSummary(t, "D1", card, summary)
		taskID := card.TaskID
		if taskID == 0 {
			t.Fatalf("the Task must exist before the questions card")
		}
		if strings.TrimSpace(card.Reply) == "" {
			t.Errorf("the questions card must come with an introducing sentence, got an empty reply")
		}

		for round := 2; round <= 4 && card.ContentType == "workflow_card"; round++ {
			answer := answerFor(card)
			t.Logf("[D%d] answering the questions card with:\n%s", round, answer)
			card, summary = say(chatID, answer)
			logSummary(t, fmt.Sprintf("D%d", round), card, summary)
		}

		if card.ContentType == "workflow_card" {
			t.Fatalf("Quasar kept asking after 3 answers")
		}
		task, found, _ := repos.AgentTask.FindByID(ctx, taskID)
		if !found || !task.IsActionable {
			t.Errorf("after the answers the Task must be actionable, found=%v", found)
		}
		stored, hasPause, _ := registry.AgentChat.CheckPointStore.GetInterruptID(ctx, chatID.String())
		if !hasPause || !strings.HasPrefix(stored, "arm|") {
			t.Fatalf("expected the Arm pause to be waiting, got %q", stored)
		}
		checkpoint, hasCheckpoint, _ := registry.AgentChat.CheckPointStore.Get(ctx, chatID.String())
		if !hasCheckpoint || len(checkpoint) == 0 {
			t.Fatalf("the Arm pause has no checkpoint data: it could not be resumed")
		}
		t.Logf("[D] ARM PAUSE KEPT ALIVE: chat_id=%s task_id=%d checkpoint_bytes=%d interrupt=%s", chatID, taskID, len(checkpoint), strings.SplitN(stored, "|", 2)[0])
		rows, _ := repos.AgentSubTask.FindByTaskID(ctx, taskID)
		var steps []string
		for _, row := range rows {
			steps = append(steps, row.Agent+"/"+row.StepName)
		}
		t.Logf("[D] persisted sub task chain for task %d: %v", taskID, steps)
		analyzerAt, secondDecisionAt, decisions := -1, -1, 0
		for i, step := range steps {
			switch step {
			case "supervisor/route_decision":
				decisions++
				if decisions == 2 {
					secondDecisionAt = i
				}
			case "supervisor/route_to_analyzer":
				analyzerAt = i
			}
		}
		if analyzerAt != -1 && (secondDecisionAt == -1 || analyzerAt < secondDecisionAt) {
			t.Errorf("Nova was asked before the Task was updated: %v", steps)
		}
	})
}

func liveHarness(t *testing.T) (*service.Registry, *captureConn) {
	t.Helper()
	if os.Getenv("LIVE_AGENT_FLOW") != "1" {
		t.Skip("LIVE_AGENT_FLOW=1 not set: this test uses the configured database, model and chain")
	}
	if err := godotenv.Load("../../../.env"); err != nil {
		t.Logf("no .env loaded (%v), relying on already-exported environment", err)
	}
	db, err := config.NewDatabase(os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatalf("connect db: %v", err)
	}
	registry := service.NewRegistry(service.Config{Repos: repository.NewRegistry(db)})
	if registry.AgentChat == nil {
		t.Fatal("agent chat service disabled: check the .env settings")
	}
	conn := newCaptureConn()
	go realtime.Default().Serve(conn, &auth.Claims{WalletAddress: liveDemoWallet, Role: "user"})
	t.Cleanup(func() { close(conn.incoming) })
	return registry, conn
}

// The placeholder words must come from the model in the user's own language.
func TestLiveAgentAsTool_StatusFollowsTheUsersLanguage(t *testing.T) {
	registry, conn := liveHarness(t)
	for _, message := range []string{"こんにちは、元気ですか？", "Halo, lagi apa nih?", "Merhaba, nasılsın?"} {
		chatID := uuid.New()
		conn.subscribe(agentsvc.ChatStreamTopic(chatID))
		card, err := registry.AgentChat.HandleChatMessage(context.Background(), chatID, liveDemoWallet, message, false)
		if err != nil {
			t.Fatalf("HandleChatMessage(%q): %v", message, err)
		}
		summary := summarize(conn.drain())
		t.Logf("[%q] status=%q | reply=%q", message, summary.status, strings.TrimSpace(card.Reply))
		if len(summary.status) != 1 || strings.TrimSpace(summary.status[0]) == "" {
			t.Errorf("expected exactly one status phrase for %q, got %q", message, summary.status)
		}
	}
}

// Nova must stay on the trusted domains: every read_article refusal shows up
// as a failed analyzer step in the live list.
func TestLiveAgentAsTool_NovaStaysOnTrustedDomains(t *testing.T) {
	registry, conn := liveHarness(t)
	for _, message := range []string{"Cek berita saham BBCA hari ini dong", "Tarik berita terbaru soal IHSG dan saham BBRI"} {
		chatID := uuid.New()
		conn.subscribe(agentsvc.ChatStreamTopic(chatID))
		card, err := registry.AgentChat.HandleChatMessage(context.Background(), chatID, liveDemoWallet, message, false)
		if err != nil {
			t.Fatalf("HandleChatMessage(%q): %v", message, err)
		}
		summary := summarize(conn.drain())
		failed := 0
		for _, step := range summary.subTasks {
			if strings.HasSuffix(step, "=failed") {
				failed++
			}
		}
		t.Logf("[%q] task=%d content_type=%s failed_steps=%d tool_calls=%v", message, card.TaskID, card.ContentType, failed, summary.toolCalls)
		t.Logf("[%q] sub_tasks=%v", message, summary.subTasks)
	}
}
