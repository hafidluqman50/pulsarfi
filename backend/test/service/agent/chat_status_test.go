package agent_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/horizonlabs/pulsarfi-backend/src/auth"
	agentsvc "github.com/horizonlabs/pulsarfi-backend/src/service/agent"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/analyzer"
	"github.com/horizonlabs/pulsarfi-backend/src/service/realtime"
)

// docs/plans/chat-status-in-user-language.md and the trusted-sources part of
// docs/plans/news-brief-evidence-cards.md (v1.16). No database, no real model.

type memoryCheckpointStore struct {
	mu        sync.Mutex
	items     map[string][]byte
	interrupt map[string]string
}

func newMemoryCheckpointStore() *memoryCheckpointStore {
	return &memoryCheckpointStore{items: map[string][]byte{}, interrupt: map[string]string{}}
}

func (s *memoryCheckpointStore) Get(_ context.Context, id string) ([]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, found := s.items[id]
	return data, found, nil
}

func (s *memoryCheckpointStore) Set(_ context.Context, id string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[id] = data
	return nil
}

func (s *memoryCheckpointStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, id)
	delete(s.interrupt, id)
	return nil
}

func (s *memoryCheckpointStore) Has(_ context.Context, id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, found := s.items[id]
	return found, nil
}

func (s *memoryCheckpointStore) GetInterruptID(_ context.Context, id string) (string, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	value, found := s.interrupt[id]
	return value, found, nil
}

func (s *memoryCheckpointStore) SetInterruptID(_ context.Context, id, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.interrupt[id] = value
	return nil
}

type recordingModel struct {
	mu    sync.Mutex
	input []*schema.Message
	reply string
	err   error
}

func (m *recordingModel) Generate(_ context.Context, input []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	m.mu.Lock()
	m.input = input
	m.mu.Unlock()
	if m.err != nil {
		return nil, m.err
	}
	return schema.AssistantMessage(m.reply, nil), nil
}

func (m *recordingModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return nil, errors.New("not used")
}

func statusTurn(t *testing.T, replyModel model.BaseChatModel, message string) (result agentsvc.OrchestratorResult, statusTexts []string) {
	t.Helper()
	ctx := context.Background()
	orchestrator := &agentsvc.Orchestrator{
		Quasar:          newAgent(t, "supervisor_agent", []*schema.Message{answer("", "ok")}),
		CheckPointStore: newMemoryCheckpointStore(),
		ReplyModel:      replyModel,
	}
	orchestrator, err := agentsvc.NewOrchestrator(ctx, orchestrator)
	if err != nil {
		t.Fatalf("build orchestrator: %v", err)
	}

	chatID := uuid.New()
	conn := newCaptureConn()
	go realtime.Default().Serve(conn, &auth.Claims{WalletAddress: "0xtest", Role: "user"})
	defer close(conn.incoming)
	conn.subscribe(agentsvc.ChatStreamTopic(chatID))

	result, err = orchestrator.Send(ctx, agentsvc.OrchestratorInput{
		ChatID: chatID, Wallet: "0xtest", RawPrompt: message, Messages: []*schema.Message{schema.UserMessage(message)},
	})
	if err != nil {
		t.Fatalf("Send: %v", err)
	}
	for _, frame := range conn.drain() {
		var outer struct {
			Data struct {
				Type string `json:"type"`
				Data struct {
					Text string `json:"text"`
				} `json:"data"`
			} `json:"data"`
		}
		if json.Unmarshal([]byte(frame), &outer) == nil && outer.Data.Type == "status" {
			statusTexts = append(statusTexts, outer.Data.Data.Text)
		}
	}
	return result, statusTexts
}

// The words come from the model, in whatever language the user wrote; the
// prompt carries the user's message and names no language.
func TestStatus_ComesFromTheModelInTheUsersLanguage(t *testing.T) {
	recorder := &recordingModel{reply: "  Quasarが考えています…  "}
	message := "今日のIHSGのチャートを見せて"

	result, statusTexts := statusTurn(t, recorder, message)

	if result.Reply != "ok" {
		t.Fatalf("the turn itself changed: %q", result.Reply)
	}
	if len(statusTexts) != 1 || statusTexts[0] != "Quasarが考えています…" {
		t.Fatalf("expected the model's phrase, trimmed, as one status event, got %q", statusTexts)
	}
	recorder.mu.Lock()
	defer recorder.mu.Unlock()
	if len(recorder.input) != 2 || recorder.input[1].Content != message {
		t.Fatalf("the user's message must be what the model reads: %+v", recorder.input)
	}
	for _, language := range []string{"Japanese", "Indonesian", "English", "Turkish", "Javanese"} {
		if strings.Contains(recorder.input[0].Content, language) {
			t.Fatalf("the prompt must not name a language, found %q", language)
		}
	}
}

func TestStatus_AFailingModelNeverTouchesTheTurn(t *testing.T) {
	result, statusTexts := statusTurn(t, &recordingModel{err: errors.New("model down")}, "halo")

	if result.Reply != "ok" || len(statusTexts) != 0 {
		t.Fatalf("expected the normal reply and no status, got reply=%q status=%q", result.Reply, statusTexts)
	}
}

func TestStatus_NoModelConfiguredIsFine(t *testing.T) {
	result, statusTexts := statusTurn(t, nil, "halo")

	if result.Reply != "ok" || len(statusTexts) != 0 {
		t.Fatalf("expected the normal reply and no status, got reply=%q status=%q", result.Reply, statusTexts)
	}
}

// Nova's prompt is generated from the one domain list, never a second copy.
func TestTrustedSources_PromptIsGeneratedFromTheList(t *testing.T) {
	section := analyzer.TrustedSourcesInstructions(analyzer.TrustedNewsDomains)
	for _, domain := range analyzer.TrustedNewsDomains {
		if !strings.Contains(section, domain) {
			t.Errorf("the prompt does not name the configured domain %q", domain)
		}
	}
	if !strings.Contains(section, analyzer.ExternalSourceMarker) {
		t.Errorf("the prompt must explain the marker the fallback search puts on external results")
	}

	custom := analyzer.TrustedSourcesInstructions([]string{"example.org"})
	if !strings.Contains(custom, "example.org") || strings.Contains(custom, "kompas.com") {
		t.Errorf("the section must reflect exactly the list it is given, got: %s", custom)
	}
}
