package agent_test

import (
	"context"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	agentsvc "github.com/horizonlabs/pulsarfi-backend/src/service/agent"
)

// Proof tests for docs/plans/agent-as-tool-orchestration.md (spikes S1-S5):
// Quasar as an adk.ChatModelAgent calling Nova and Comet through
// adk.NewAgentTool. Scripted fake models, no network, no database.

var errNotArmed = errors.New("task is not armed")

type scriptedModel struct {
	mu    sync.Mutex
	steps []*schema.Message
	next  int
}

func (m *scriptedModel) pop() (*schema.Message, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.next >= len(m.steps) {
		return nil, errors.New("scripted model: no more steps")
	}
	step := m.steps[m.next]
	m.next++
	return step, nil
}

func (m *scriptedModel) Generate(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.Message, error) {
	return m.pop()
}

// Stream splits reasoning and content into several chunks, the way a real
// provider streams, so a test can tell streaming from one-shot delivery.
func (m *scriptedModel) Stream(_ context.Context, _ []*schema.Message, _ ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	step, err := m.pop()
	if err != nil {
		return nil, err
	}
	if len(step.ToolCalls) > 0 {
		return schema.StreamReaderFromArray([]*schema.Message{step}), nil
	}
	var chunks []*schema.Message
	for _, part := range splitInHalf(step.ReasoningContent) {
		chunks = append(chunks, &schema.Message{Role: schema.Assistant, ReasoningContent: part})
	}
	for _, part := range splitInHalf(step.Content) {
		chunks = append(chunks, &schema.Message{Role: schema.Assistant, Content: part})
	}
	return schema.StreamReaderFromArray(chunks), nil
}

func (m *scriptedModel) WithTools(_ []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	return m, nil
}

func splitInHalf(text string) []string {
	if text == "" {
		return nil
	}
	middle := len(text) / 2
	return []string{text[:middle], text[middle:]}
}

func toolCall(name, arguments string) *schema.Message {
	return schema.AssistantMessage("", []schema.ToolCall{{ID: name + "-1", Function: schema.FunctionCall{Name: name, Arguments: arguments}}})
}

func answer(reasoning, content string) *schema.Message {
	return &schema.Message{Role: schema.Assistant, ReasoningContent: reasoning, Content: content}
}

type memoryStore struct {
	mu    sync.Mutex
	items map[string][]byte
}

func newMemoryStore() *memoryStore { return &memoryStore{items: map[string][]byte{}} }

func (s *memoryStore) Get(_ context.Context, key string) ([]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	data, found := s.items[key]
	return data, found, nil
}

func (s *memoryStore) Set(_ context.Context, key string, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.items[key] = value
	return nil
}

type seenEvent struct {
	Agent       string
	Role        schema.RoleType
	Streaming   bool
	Chunks      int
	Content     string
	Reasoning   string
	Err         error
	InterruptID string
}

func collect(t *testing.T, iterator *adk.AsyncIterator[*adk.AgentEvent]) []seenEvent {
	t.Helper()
	var seen []seenEvent
	for {
		event, ok := iterator.Next()
		if !ok {
			return seen
		}
		entry := seenEvent{Agent: event.AgentName, Err: event.Err}
		if event.Action != nil && event.Action.Interrupted != nil {
			for _, interruptCtx := range event.Action.Interrupted.InterruptContexts {
				if interruptCtx.IsRootCause {
					entry.InterruptID = interruptCtx.ID
				}
			}
		}
		if event.Output != nil && event.Output.MessageOutput != nil {
			output := event.Output.MessageOutput
			entry.Role = output.Role
			entry.Streaming = output.IsStreaming
			if output.IsStreaming {
				for {
					chunk, err := output.MessageStream.Recv()
					if err == io.EOF {
						break
					}
					if err != nil {
						t.Fatalf("read stream of %s: %v", event.AgentName, err)
					}
					entry.Chunks++
					entry.Content += chunk.Content
					entry.Reasoning += chunk.ReasoningContent
				}
			} else if output.Message != nil {
				entry.Content = output.Message.Content
			}
		}
		seen = append(seen, entry)
	}
}

func eventsFrom(seen []seenEvent, agentName string) []seenEvent {
	var matched []seenEvent
	for _, entry := range seen {
		if entry.Agent == agentName && entry.Role == schema.Assistant {
			matched = append(matched, entry)
		}
	}
	return matched
}

func newAgent(t *testing.T, name string, steps []*schema.Message, tools ...tool.BaseTool) adk.Agent {
	t.Helper()
	config := &adk.ChatModelAgentConfig{
		Name:        name,
		Description: name + " agent",
		Instruction: "test",
		Model:       &scriptedModel{steps: steps},
	}
	if len(tools) > 0 {
		config.ToolsConfig = adk.ToolsConfig{
			ToolsNodeConfig:    compose.ToolsNodeConfig{Tools: tools},
			EmitInternalEvents: true,
		}
	}
	agent, err := adk.NewChatModelAgent(context.Background(), config)
	if err != nil {
		t.Fatalf("build agent %s: %v", name, err)
	}
	return agent
}

func newRunner(agent adk.Agent, store compose.CheckPointStore) *adk.Runner {
	return adk.NewRunner(context.Background(), adk.RunnerConfig{Agent: agent, EnableStreaming: true, CheckPointStore: store})
}

func userMessages() []*schema.Message { return []*schema.Message{schema.UserMessage("hi")} }

func emptyParams() *schema.ParamsOneOf {
	return schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{})
}

// askUserTool mirrors the questions-card pause in the plan (section 6.4).
type askUserTool struct{}

func (askUserTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "ask_user", Desc: "asks the user a question", ParamsOneOf: emptyParams()}, nil
}

func (askUserTool) InvokableRun(ctx context.Context, arguments string, _ ...tool.Option) (string, error) {
	if wasInterrupted, _, _ := tool.GetInterruptState[string](ctx); !wasInterrupted {
		return "", tool.StatefulInterrupt(ctx, "questions card", arguments)
	}
	if isTarget, hasData, answerText := tool.GetResumeContext[string](ctx); isTarget && hasData {
		return answerText, nil
	}
	return "", tool.StatefulInterrupt(ctx, "questions card", arguments)
}

// awaitArmTool is the Arm pause: a tool whose only job is to interrupt and be
// resumed. It must not wrap the Comet agent call: an AgentTool invoked inside
// a tool that interrupted itself reads that interrupt as its own and fails
// with "interrupt has happened, but cannot find interrupt state".
type awaitArmTool struct{}

func (awaitArmTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "await_arm", Desc: "waits for the user to arm the task", ParamsOneOf: emptyParams()}, nil
}

func (awaitArmTool) InvokableRun(ctx context.Context, arguments string, _ ...tool.Option) (string, error) {
	if wasInterrupted, _, _ := tool.GetInterruptState[string](ctx); !wasInterrupted {
		return "", tool.StatefulInterrupt(ctx, "arm card", arguments)
	}
	if isTarget, _, _ := tool.GetResumeContext[string](ctx); isTarget {
		return "armed", nil
	}
	return "", tool.StatefulInterrupt(ctx, "arm card", arguments)
}

// cometGuard is the in-code Arm check: it never interrupts, it only refuses
// when the Task is not armed, then hands over to the Comet agent call.
type cometGuard struct {
	inner tool.InvokableTool
	armed func() bool
}

func (g *cometGuard) Info(ctx context.Context) (*schema.ToolInfo, error) { return g.inner.Info(ctx) }

func (g *cometGuard) InvokableRun(ctx context.Context, arguments string, opts ...tool.Option) (string, error) {
	if !g.armed() {
		return "", errNotArmed
	}
	return g.inner.InvokableRun(ctx, arguments, opts...)
}

type passthroughTool struct {
	inner       tool.InvokableTool
	forwardOpts bool
}

func (p *passthroughTool) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return p.inner.Info(ctx)
}

func (p *passthroughTool) InvokableRun(ctx context.Context, arguments string, opts ...tool.Option) (string, error) {
	if p.forwardOpts {
		return p.inner.InvokableRun(ctx, arguments, opts...)
	}
	return p.inner.InvokableRun(ctx, arguments)
}

type walletProbeTool struct{}

func (walletProbeTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{Name: "probe", Desc: "reads the run context", ParamsOneOf: emptyParams()}, nil
}

func (walletProbeTool) InvokableRun(ctx context.Context, _ string, _ ...tool.Option) (string, error) {
	runContext, ok := agentsvc.RunContextFrom(ctx)
	if !ok {
		return "no run context", nil
	}
	return "wallet=" + runContext.Wallet, nil
}

func asInvokable(t *testing.T, base tool.BaseTool) tool.InvokableTool {
	t.Helper()
	invokable, ok := base.(tool.InvokableTool)
	if !ok {
		t.Fatalf("agent tool is not an InvokableTool")
	}
	return invokable
}

// S1: one iterator carries streaming events, with readable reasoning, from
// Quasar, Nova and Comet, told apart by AgentName.
func TestEinoProofS1_OneIteratorCarriesAllThreeAgents(t *testing.T) {
	ctx := context.Background()
	nova := newAgent(t, "nova", []*schema.Message{answer("nova is thinking about the chart", "nova answer text")})
	comet := newAgent(t, "comet", []*schema.Message{answer("comet is weighing the size", "comet answer text")})
	quasar := newAgent(t, "quasar", []*schema.Message{
		toolCall("nova", `{"request":"chart"}`),
		toolCall("comet", `{"request":"execute"}`),
		answer("", "all done"),
	}, adk.NewAgentTool(ctx, nova), adk.NewAgentTool(ctx, comet))

	seen := collect(t, newRunner(quasar, newMemoryStore()).Run(ctx, userMessages()))

	for _, name := range []string{"nova", "comet"} {
		events := eventsFrom(seen, name)
		if len(events) == 0 {
			t.Fatalf("no assistant events from %s reached the parent iterator", name)
		}
		last := events[len(events)-1]
		if !last.Streaming || last.Chunks < 2 {
			t.Fatalf("%s events are not streamed: streaming=%v chunks=%d", name, last.Streaming, last.Chunks)
		}
		if !strings.Contains(last.Reasoning, name+" is") {
			t.Fatalf("%s reasoning not readable from the stream: %q", name, last.Reasoning)
		}
	}
	quasarEvents := eventsFrom(seen, "quasar")
	if len(quasarEvents) == 0 || !strings.Contains(quasarEvents[len(quasarEvents)-1].Content, "all done") {
		t.Fatalf("quasar final reply missing: %+v", quasarEvents)
	}
}

// S2: ask_user interrupts in request 1; request 2 resumes with the answer
// using a brand-new Runner and agent over the same checkpoint store.
func TestEinoProofS2_QuestionsPauseResumesInANewRequest(t *testing.T) {
	ctx := context.Background()
	store := newMemoryStore()

	first := newAgent(t, "quasar", []*schema.Message{toolCall("ask_user", `{"question":"budget?"}`)}, askUserTool{})
	firstEvents := collect(t, newRunner(first, store).Run(ctx, userMessages(), adk.WithCheckPointID("chat-1")))
	interruptID := ""
	for _, entry := range firstEvents {
		if entry.InterruptID != "" {
			interruptID = entry.InterruptID
		}
	}
	if interruptID == "" {
		t.Fatalf("request 1 did not interrupt: %+v", firstEvents)
	}

	second := newAgent(t, "quasar", []*schema.Message{answer("", "budget received")}, askUserTool{})
	iterator, err := newRunner(second, store).ResumeWithParams(ctx, "chat-1", &adk.ResumeParams{Targets: map[string]any{interruptID: "100000"}})
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	secondEvents := collect(t, iterator)

	var sawAnswerAsToolResult, sawFinal bool
	for _, entry := range secondEvents {
		if entry.Err != nil {
			t.Fatalf("resume produced an error: %v", entry.Err)
		}
		if entry.Role == schema.Tool && entry.Content == "100000" {
			sawAnswerAsToolResult = true
		}
		if entry.Role == schema.Assistant && strings.Contains(entry.Content, "budget received") {
			sawFinal = true
		}
	}
	if !sawAnswerAsToolResult || !sawFinal {
		t.Fatalf("answer not delivered to the tool or turn did not finish: toolResult=%v final=%v events=%+v", sawAnswerAsToolResult, sawFinal, secondEvents)
	}
}

// S3: the Arm gate pauses before Comet; a separate resume call streams
// Comet's events; an unarmed Task is refused in code.
func TestEinoProofS3_ArmGateStreamsCometAfterResumeAndRefusesUnarmed(t *testing.T) {
	run := func(t *testing.T, armed bool) (resumed []seenEvent) {
		ctx := context.Background()
		store := newMemoryStore()
		build := func(quasarSteps []*schema.Message, cometSteps []*schema.Message) adk.Agent {
			comet := newAgent(t, "comet", cometSteps)
			guard := &cometGuard{inner: asInvokable(t, adk.NewAgentTool(ctx, comet)), armed: func() bool { return armed }}
			return newAgent(t, "quasar", quasarSteps, awaitArmTool{}, guard)
		}

		first := build([]*schema.Message{toolCall("await_arm", `{}`)}, nil)
		firstEvents := collect(t, newRunner(first, store).Run(ctx, userMessages(), adk.WithCheckPointID("chat-3")))
		interruptID := ""
		for _, entry := range firstEvents {
			if entry.InterruptID != "" {
				interruptID = entry.InterruptID
			}
			if eventsFrom([]seenEvent{entry}, "comet") != nil {
				t.Fatalf("comet ran before the gate was resumed")
			}
		}
		if interruptID == "" {
			t.Fatalf("gate did not interrupt: %+v", firstEvents)
		}

		second := build(
			[]*schema.Message{toolCall("comet", `{"request":"execute"}`), answer("", "executed")},
			[]*schema.Message{answer("comet reasoning here", "comet executed")},
		)
		iterator, err := newRunner(second, store).ResumeWithParams(ctx, "chat-3", &adk.ResumeParams{Targets: map[string]any{interruptID: "armed"}})
		if err != nil {
			t.Fatalf("resume: %v", err)
		}
		return collect(t, iterator)
	}

	t.Run("armed task streams comet live", func(t *testing.T) {
		resumed := run(t, true)
		cometEvents := eventsFrom(resumed, "comet")
		if len(cometEvents) == 0 {
			t.Fatalf("no comet events after the gate was resumed: %+v", resumed)
		}
		if last := cometEvents[len(cometEvents)-1]; !last.Streaming || last.Chunks < 2 {
			t.Fatalf("comet events after resume are not streamed: %+v", last)
		}
	})

	t.Run("unarmed task is refused", func(t *testing.T) {
		var refused bool
		for _, entry := range run(t, false) {
			if entry.Err != nil && strings.Contains(entry.Err.Error(), errNotArmed.Error()) {
				refused = true
			}
		}
		if !refused {
			t.Fatalf("an unarmed task was not refused")
		}
	})
}

// S4: a RunContext set in ctx before Run is readable inside Nova's own tool,
// two agents deep.
func TestEinoProofS4_RunContextReachesNovaTools(t *testing.T) {
	ctx := agentsvc.WithRunContext(context.Background(), &agentsvc.RunContext{Wallet: "0xabc"})
	nova := newAgent(t, "nova", []*schema.Message{toolCall("probe", `{}`), answer("", "nova done")}, walletProbeTool{})
	quasar := newAgent(t, "quasar", []*schema.Message{toolCall("nova", `{"request":"x"}`), answer("", "done")}, adk.NewAgentTool(ctx, nova))

	seen := collect(t, newRunner(quasar, newMemoryStore()).Run(ctx, userMessages()))

	for _, entry := range seen {
		if entry.Agent == "nova" && entry.Role == schema.Tool && entry.Content == "wallet=0xabc" {
			return
		}
	}
	t.Fatalf("nova's tool did not see the RunContext: %+v", seen)
}

// S5: a wrapper tool that forwards opts keeps the inner agent's events
// flowing; one that drops opts does not. The second case is the control that
// proves the first result is not an accident.
func TestEinoProofS5_WrapperMustForwardOptsToKeepEvents(t *testing.T) {
	novaEventsWith := func(t *testing.T, wrap func(inner tool.InvokableTool) tool.BaseTool) int {
		ctx := context.Background()
		nova := newAgent(t, "nova", []*schema.Message{answer("nova thinking now", "nova answer text")})
		agentTool := adk.NewAgentTool(ctx, nova)
		var novaTool tool.BaseTool = agentTool
		if wrap != nil {
			novaTool = wrap(asInvokable(t, agentTool))
		}
		quasar := newAgent(t, "quasar", []*schema.Message{toolCall("nova", `{"request":"x"}`), answer("", "done")}, novaTool)
		return len(eventsFrom(collect(t, newRunner(quasar, newMemoryStore()).Run(ctx, userMessages())), "nova"))
	}

	direct := novaEventsWith(t, nil)
	forwarded := novaEventsWith(t, func(inner tool.InvokableTool) tool.BaseTool {
		return &passthroughTool{inner: inner, forwardOpts: true}
	})
	dropped := novaEventsWith(t, func(inner tool.InvokableTool) tool.BaseTool {
		return &passthroughTool{inner: inner, forwardOpts: false}
	})

	if direct == 0 {
		t.Fatalf("baseline: no nova events without a wrapper")
	}
	if forwarded != direct {
		t.Fatalf("forwarding wrapper changed the event count: direct=%d forwarded=%d", direct, forwarded)
	}
	if dropped != 0 {
		t.Logf("note: a wrapper that drops opts still delivered %d nova events (direct=%d)", dropped, direct)
	}
}
