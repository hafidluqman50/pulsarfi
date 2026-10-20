package agent_test

import (
	"context"
	"sync"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	agentsvc "github.com/horizonlabs/pulsarfi-backend/src/service/agent"
)

// budgetCheckpoints is a tiny in-memory ExtendedCheckPointStore — this test
// needs no database.
type budgetCheckpoints struct {
	mu   sync.Mutex
	data map[string][]byte
}

func newBudgetCheckpoints() *budgetCheckpoints {
	return &budgetCheckpoints{data: map[string][]byte{}}
}

func interruptKey(id string) string { return "interrupt_id:" + id }

func (s *budgetCheckpoints) Get(_ context.Context, id string) ([]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	v, ok := s.data[id]
	return v, ok, nil
}

func (s *budgetCheckpoints) Set(_ context.Context, id string, v []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[id] = v
	return nil
}

func (s *budgetCheckpoints) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.data, id)
	delete(s.data, interruptKey(id))
	return nil
}

func (s *budgetCheckpoints) Has(_ context.Context, id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.data[id]
	return ok, nil
}

func (s *budgetCheckpoints) GetInterruptID(_ context.Context, id string) (string, bool, error) {
	v, ok, err := s.Get(context.Background(), interruptKey(id))
	return string(v), ok, err
}

func (s *budgetCheckpoints) SetInterruptID(_ context.Context, id, v string) error {
	return s.Set(context.Background(), interruptKey(id), []byte(v))
}

// toolLoopModel always asks for the same tool, so an agent with a small
// MaxIterations exhausts its budget without ever writing a final answer.
type toolLoopModel struct{}

func (toolLoopModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return toolCall("noop", "{}"), nil
}

func (toolLoopModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return schema.StreamReaderFromArray([]*schema.Message{toolCall("noop", "{}")}), nil
}

func (m toolLoopModel) WithTools([]*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

// budgetReplyModel stands in for Quasar's ReplyModel: the one forced final
// answer after the budget is gone.
type budgetReplyModel struct{ reply string }

func (m *budgetReplyModel) Generate(context.Context, []*schema.Message, ...model.Option) (*schema.Message, error) {
	return schema.AssistantMessage(m.reply, nil), nil
}

func (m *budgetReplyModel) Stream(context.Context, []*schema.Message, ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	return schema.StreamReaderFromArray([]*schema.Message{schema.AssistantMessage(m.reply, nil)}), nil
}

type noopTool struct{}

func (noopTool) Info(context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name:        "noop",
		Desc:        "does nothing",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{}),
	}, nil
}

func (noopTool) InvokableRun(context.Context, string, ...tool.Option) (string, error) {
	return "ok", nil
}

// TestToolBudget_TruncatesInsteadOfFailing pins the contract
// (docs/plans/agent-tool-budget-truncation.md): reaching MaxIterations is a
// budget, not a failure. Send must finalize the turn — using the model's own
// words when it wrote none — instead of returning Eino's error, which used to
// become an HTTP 500.
func TestToolBudget_TruncatesInsteadOfFailing(t *testing.T) {
	ctx := context.Background()

	quasar, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:          "supervisor_agent",
		Description:   "test",
		Instruction:   "test",
		Model:         toolLoopModel{},
		ToolsConfig:   adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: []tool.BaseTool{noopTool{}}}},
		MaxIterations: 1,
	})
	if err != nil {
		t.Fatalf("build quasar: %v", err)
	}

	orchestrator := &agentsvc.Orchestrator{
		Quasar:          quasar,
		ReplyModel:      &budgetReplyModel{reply: "ini jawaban dari bukti yang ada"},
		CheckPointStore: newBudgetCheckpoints(),
	}
	orchestrator, err = agentsvc.NewOrchestrator(ctx, orchestrator)
	if err != nil {
		t.Fatalf("build orchestrator: %v", err)
	}

	chatID := uuid.New()
	result, err := orchestrator.Send(ctx, agentsvc.OrchestratorInput{
		ChatID:    chatID,
		Wallet:    "0xtest",
		RawPrompt: "cari berita IHSG",
		Messages:  []*schema.Message{schema.UserMessage("cari berita IHSG")},
	})
	if err != nil {
		t.Fatalf("a tool-budget cap must not fail the turn, got: %v", err)
	}
	if result.Reply != "ini jawaban dari bukti yang ada" {
		t.Fatalf("expected the forced final answer, got %q", result.Reply)
	}
}
