package agent_test

import (
    "bytes"
    "context"
    "encoding/gob"
    "errors"
    "strings"
    "sync"
    "testing"

    "github.com/cloudwego/eino/adk"
    "github.com/cloudwego/eino/components/tool"
    "github.com/cloudwego/eino/compose"
    "github.com/cloudwego/eino/schema"
    agentcore "github.com/horizonlabs/pulsarfi-backend/src/app/core/agent"
    "github.com/horizonlabs/pulsarfi-backend/src/app/helper"
)

type spikeState struct {
    ChatID   string
    Messages []*schema.Message

    TaskID     int64
    Decision   agentcore.RouteDecision
    Verdict    *helper.NovaVerdict
    NeedsNova  bool
    NovaDone   bool
    Armed      bool
    CometDone  bool
    Replied    bool
    Reply      string
    Visited    []string
}

type spikePause struct {
    InnerInterruptID string
    State            *spikeState
}

func init() {
    schema.RegisterName[*spikeState]("graph_spike_state")
    schema.RegisterName[*spikePause]("graph_spike_pause")
}

type spikeProbe struct {
    mu     sync.Mutex
    events []seenEvent
}

func (probe *spikeProbe) add(seen []seenEvent) {
    probe.mu.Lock()
    defer probe.mu.Unlock()
    probe.events = append(probe.events, seen...)
}

func spikeInterruptIDFrom(seen []seenEvent) string {
    for _, entry := range seen {
        if entry.InterruptID != "" {
            return entry.InterruptID
        }
    }

    return ""
}

func spikeLastAssistantContent(seen []seenEvent) string {
    content := ""
    for _, entry := range seen {
        if entry.Role == schema.Assistant && entry.Content != "" {
            content = entry.Content
        }
    }

    return content
}

func spikeAgentNode(t *testing.T, store compose.CheckPointStore, probe *spikeProbe, build func() adk.Agent) func(context.Context, *spikeState) (*spikeState, error) {
    return func(ctx context.Context, state *spikeState) (*spikeState, error) {
        wasInterrupted, hasSaved, saved := compose.GetInterruptState[*spikePause](ctx)
        var events *adk.AsyncIterator[*adk.AgentEvent]
        if wasInterrupted && hasSaved {
            state = saved.State
            isTarget, hasData, answerText := compose.GetResumeContext[string](ctx)
            if !isTarget || !hasData {
                return state, compose.StatefulInterrupt(ctx, "waiting", saved)
            }

            resumed, err := newRunner(build(), store).ResumeWithParams(ctx, state.ChatID+":quasar", &adk.ResumeParams{Targets: map[string]any{saved.InnerInterruptID: answerText}})
            if err != nil {
                return state, err
            }
            events = resumed
        } else {
            events = newRunner(build(), store).Run(ctx, state.Messages, adk.WithCheckPointID(state.ChatID+":quasar"))
        }

        seen := collect(t, events)
        probe.add(seen)
        if interruptID := spikeInterruptIDFrom(seen); interruptID != "" {
            return state, compose.StatefulInterrupt(ctx, "pause", &spikePause{InnerInterruptID: interruptID, State: state})
        }

        state.Reply = spikeLastAssistantContent(seen)
        return state, nil
    }
}

func spikeSingleNodeGraph(t *testing.T, store compose.CheckPointStore, node func(context.Context, *spikeState) (*spikeState, error)) compose.Runnable[*spikeState, *spikeState] {
    t.Helper()
    graph := compose.NewGraph[*spikeState, *spikeState]()
    if err := graph.AddLambdaNode("quasar", compose.InvokableLambda(node)); err != nil {
        t.Fatalf("add node: %v", err)
    }

    if err := graph.AddEdge(compose.START, "quasar"); err != nil {
        t.Fatalf("add start edge: %v", err)
    }

    if err := graph.AddEdge("quasar", compose.END); err != nil {
        t.Fatalf("add end edge: %v", err)
    }

    runnable, err := graph.Compile(context.Background(), compose.WithCheckPointStore(store))
    if err != nil {
        t.Fatalf("compile graph: %v", err)
    }

    return runnable
}

func spikeOuterInterruptID(t *testing.T, err error) string {
    t.Helper()
    info, existed := compose.ExtractInterruptInfo(err)
    if !existed {
        t.Fatalf("graph did not return an interrupt: %v", err)
    }

    for _, interruptContext := range info.InterruptContexts {
        if interruptContext.IsRootCause {
            return interruptContext.ID
        }
    }

    t.Fatalf("interrupt has no root cause context: %+v", info)
    return ""
}

func TestGraphSpike_AgentRunsInsideNodeAndStreamsEvents(t *testing.T) {
    store := newMemoryStore()
    probe := &spikeProbe{}
    build := func() adk.Agent {
        return newAgent(t, "quasar", []*schema.Message{answer("quasar is thinking", "hello from quasar")})
    }
    runnable := spikeSingleNodeGraph(t, store, spikeAgentNode(t, store, probe, build))

    state, err := runnable.Invoke(context.Background(), &spikeState{ChatID: "chat-1", Messages: userMessages()}, compose.WithCheckPointID("chat-1"))
    if err != nil {
        t.Fatalf("invoke: %v", err)
    }

    if !strings.Contains(state.Reply, "hello from quasar") {
        t.Fatalf("expected the agent reply in the state, got %q", state.Reply)
    }

    assistant := eventsFrom(probe.events, "quasar")
    if len(assistant) == 0 {
        t.Fatalf("no assistant event from the agent reached the node")
    }

    last := assistant[len(assistant)-1]
    if !last.Streaming || last.Chunks < 2 {
        t.Fatalf("expected a streamed event with several chunks, got streaming=%v chunks=%d", last.Streaming, last.Chunks)
    }

    if !strings.Contains(last.Reasoning, "quasar is") {
        t.Fatalf("expected readable reasoning in the stream, got %q", last.Reasoning)
    }
}

func TestGraphSpike_ToolInterruptInsideNodeSurvivesRestart(t *testing.T) {
    cases := []struct {
        name         string
        tool         tool.BaseTool
        firstSteps   []*schema.Message
        secondSteps  []*schema.Message
        resumeData   string
        wantToolText string
        wantReply    string
    }{
        {
            name:         "ask_user",
            tool:         askUserTool{},
            firstSteps:   []*schema.Message{toolCall("ask_user", `{"question":"budget?"}`)},
            secondSteps:  []*schema.Message{answer("", "budget received")},
            resumeData:   "100000",
            wantToolText: "100000",
            wantReply:    "budget received",
        },
        {
            name:         "await_arm",
            tool:         awaitArmTool{},
            firstSteps:   []*schema.Message{toolCall("await_arm", `{}`)},
            secondSteps:  []*schema.Message{answer("", "ready to execute")},
            resumeData:   "arm-signal",
            wantToolText: "armed",
            wantReply:    "ready to execute",
        },
    }

    for _, testCase := range cases {
        t.Run(testCase.name, func(t *testing.T) {
            store := newMemoryStore()
            firstProbe := &spikeProbe{}
            firstBuild := func() adk.Agent { return newAgent(t, "quasar", testCase.firstSteps, testCase.tool) }
            firstRunnable := spikeSingleNodeGraph(t, store, spikeAgentNode(t, store, firstProbe, firstBuild))

            _, err := firstRunnable.Invoke(context.Background(), &spikeState{ChatID: "chat-1", Messages: userMessages()}, compose.WithCheckPointID("chat-1"))
            outerInterruptID := spikeOuterInterruptID(t, err)

            secondProbe := &spikeProbe{}
            secondBuild := func() adk.Agent { return newAgent(t, "quasar", testCase.secondSteps, testCase.tool) }
            secondRunnable := spikeSingleNodeGraph(t, store, spikeAgentNode(t, store, secondProbe, secondBuild))

            resumeContext := compose.ResumeWithData(context.Background(), outerInterruptID, testCase.resumeData)
            state, err := secondRunnable.Invoke(resumeContext, nil, compose.WithCheckPointID("chat-1"))
            if err != nil {
                t.Fatalf("resume after restart: %v", err)
            }

            sawToolResult := false
            for _, entry := range secondProbe.events {
                if entry.Err != nil {
                    t.Fatalf("resume produced an error event: %v", entry.Err)
                }

                if entry.Role == schema.Tool && entry.Content == testCase.wantToolText {
                    sawToolResult = true
                }
            }

            if !sawToolResult {
                t.Fatalf("tool result %q not delivered: %+v", testCase.wantToolText, secondProbe.events)
            }

            if !strings.Contains(state.Reply, testCase.wantReply) {
                t.Fatalf("expected reply containing %q, got %q", testCase.wantReply, state.Reply)
            }
        })
    }
}

func spikeLoopGraph(t *testing.T, maxSteps int, runaway bool) (compose.Runnable[*spikeState, *spikeState], error) {
    t.Helper()
    graph := compose.NewGraph[*spikeState, *spikeState]()
    visit := func(name string, update func(*spikeState)) func(context.Context, *spikeState) (*spikeState, error) {
        return func(_ context.Context, state *spikeState) (*spikeState, error) {
            state.Visited = append(state.Visited, name)
            update(state)
            return state, nil
        }
    }

    entries := func(state *spikeState) int {
        count := 0
        for _, name := range state.Visited {
            if name == "quasar" {
                count++
            }
        }

        return count
    }

    quasarNode := visit("quasar", func(state *spikeState) {
        switch entries(state) {
        case 1:
            state.TaskID = 1
            state.NeedsNova = true
        case 2:
            state.Armed = true
        case 3:
            state.Replied = true
        }
    })
    novaNode := visit("nova", func(state *spikeState) { state.NovaDone = true })
    cometNode := visit("comet", func(state *spikeState) { state.CometDone = true })

    for name, node := range map[string]func(context.Context, *spikeState) (*spikeState, error){"quasar": quasarNode, "nova": novaNode, "comet": cometNode} {
        if err := graph.AddLambdaNode(name, compose.InvokableLambda(node)); err != nil {
            return nil, err
        }
    }

    if err := graph.AddEdge(compose.START, "quasar"); err != nil {
        return nil, err
    }

    branch := compose.NewGraphBranch(func(_ context.Context, state *spikeState) (string, error) {
        if runaway {
            return "nova", nil
        }

        if state.Replied {
            return compose.END, nil
        }

        if state.TaskID != 0 && state.NeedsNova && !state.NovaDone {
            return "nova", nil
        }

        if state.Armed && !state.CometDone {
            return "comet", nil
        }

        return compose.END, nil
    }, map[string]bool{"nova": true, "comet": true, compose.END: true})
    if err := graph.AddBranch("quasar", branch); err != nil {
        return nil, err
    }

    if err := graph.AddEdge("nova", "quasar"); err != nil {
        return nil, err
    }

    if err := graph.AddEdge("comet", "quasar"); err != nil {
        return nil, err
    }

    return graph.Compile(context.Background(), compose.WithMaxRunSteps(maxSteps))
}

func TestGraphSpike_LoopRunsQuasarNovaQuasarCometQuasarAndEnds(t *testing.T) {
    want := []string{"quasar", "nova", "quasar", "comet", "quasar"}
    minimumSteps := 0
    for steps := 1; steps <= 30; steps++ {
        runnable, err := spikeLoopGraph(t, steps, false)
        if err != nil {
            t.Fatalf("compile loop graph with %d steps: %v", steps, err)
        }

        state, err := runnable.Invoke(context.Background(), &spikeState{ChatID: "chat-1"})
        if err != nil {
            continue
        }

        if strings.Join(state.Visited, ",") != strings.Join(want, ",") {
            t.Fatalf("expected visit order %v, got %v", want, state.Visited)
        }

        minimumSteps = steps
        break
    }

    if minimumSteps == 0 {
        t.Fatalf("a full turn did not finish within 30 steps")
    }

    t.Logf("smallest WithMaxRunSteps that lets a full turn finish: %d", minimumSteps)
}

func TestGraphSpike_MaxRunStepsStopsARunawayLoop(t *testing.T) {
    runnable, err := spikeLoopGraph(t, 6, true)
    if err != nil {
        t.Fatalf("compile runaway graph: %v", err)
    }

    _, err = runnable.Invoke(context.Background(), &spikeState{ChatID: "chat-1"})
    if err == nil {
        t.Fatalf("expected the runaway loop to stop with an error")
    }

    if !errors.Is(err, compose.ErrExceedMaxSteps) && !strings.Contains(err.Error(), "exceeds max steps") {
        t.Fatalf("expected the max steps error, got %v", err)
    }
}

func TestGraphSpike_TurnStateRoundTripsThroughGob(t *testing.T) {
    original := &spikeState{
        ChatID:    "chat-1",
        Messages:  userMessages(),
        TaskID:    7,
        Decision:  agentcore.RouteDecision{IsActionable: true, Side: "buy", MentionedTicker: "BRPTP", BudgetIDRX: "100000"},
        Verdict:   &helper.NovaVerdict{Tradeable: true, EntryPrice: "100", Confidence: "high"},
        NeedsNova: true,
        NovaDone:  true,
        Armed:     true,
        Reply:     "done",
        Visited:   []string{"quasar", "nova"},
    }

    var buffer bytes.Buffer
    if err := gob.NewEncoder(&buffer).Encode(original); err != nil {
        t.Fatalf("encode: %v", err)
    }

    decoded := &spikeState{}
    if err := gob.NewDecoder(&buffer).Decode(decoded); err != nil {
        t.Fatalf("decode: %v", err)
    }

    if decoded.TaskID != 7 || decoded.Decision.Side != "buy" || decoded.Verdict == nil || !decoded.Verdict.Tradeable || !decoded.Armed || len(decoded.Messages) != 1 {
        t.Fatalf("state did not survive gob: %+v", decoded)
    }
}

type recordTaskTool struct{}

func (recordTaskTool) Info(_ context.Context) (*schema.ToolInfo, error) {
    return &schema.ToolInfo{Name: "record_task", Desc: "writes the task into the run context", ParamsOneOf: emptyParams()}, nil
}

func (recordTaskTool) InvokableRun(ctx context.Context, _ string, _ ...tool.Option) (string, error) {
    run, ok := agentcore.RunContextFrom(ctx)
    if !ok {
        return "no run context", nil
    }

    run.TaskID = 7
    run.Decision.Side = "buy"
    return "recorded", nil
}

type replyTool struct{}

func (replyTool) Info(_ context.Context) (*schema.ToolInfo, error) {
    return &schema.ToolInfo{Name: "reply", Desc: "ends the turn with a structured reply", ParamsOneOf: emptyParams()}, nil
}

func (replyTool) InvokableRun(_ context.Context, _ string, _ ...tool.Option) (string, error) {
    return "reply-recorded", nil
}

func TestGraphSpike_ToolResultReachesNodeAndReplyEndsTheRun(t *testing.T) {
    model := &scriptedModel{steps: []*schema.Message{
        toolCall("record_task", `{}`),
        toolCall("reply", `{}`),
    }}
    agent, err := adk.NewChatModelAgent(context.Background(), &adk.ChatModelAgentConfig{
        Name:        "quasar",
        Description: "quasar agent",
        Instruction: "test",
        Model:       model,
        ToolsConfig: adk.ToolsConfig{
            ToolsNodeConfig:    compose.ToolsNodeConfig{Tools: []tool.BaseTool{recordTaskTool{}, replyTool{}}},
            ReturnDirectly:     map[string]bool{"reply": true},
            EmitInternalEvents: true,
        },
    })
    if err != nil {
        t.Fatalf("build agent: %v", err)
    }

    store := newMemoryStore()
    probe := &spikeProbe{}
    node := func(ctx context.Context, state *spikeState) (*spikeState, error) {
        run := &agentcore.RunContext{Wallet: "0xabc"}
        events := newRunner(agent, store).Run(agentcore.WithRunContext(ctx, run), state.Messages)
        probe.add(collect(t, events))
        state.TaskID = run.TaskID
        state.Decision = run.Decision
        return state, nil
    }
    runnable := spikeSingleNodeGraph(t, store, node)

    state, err := runnable.Invoke(context.Background(), &spikeState{ChatID: "chat-1", Messages: userMessages()})
    if err != nil {
        t.Fatalf("invoke: %v", err)
    }

    if state.TaskID != 7 || state.Decision.Side != "buy" {
        t.Fatalf("tool result written into the run context did not reach the state: task=%d side=%q", state.TaskID, state.Decision.Side)
    }

    sawReply := false
    for _, entry := range probe.events {
        if entry.Err != nil {
            t.Fatalf("the run asked the model again after reply: %v", entry.Err)
        }

        if entry.Role == schema.Tool && entry.Content == "reply-recorded" {
            sawReply = true
        }
    }

    if !sawReply {
        t.Fatalf("reply tool result missing from the events: %+v", probe.events)
    }
}
