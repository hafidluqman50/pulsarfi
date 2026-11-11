package agent_test

import (
    "context"
    "strconv"
    "strings"
    "sync"
    "testing"

    "github.com/cloudwego/eino/schema"
    "github.com/google/uuid"
    agentcore "github.com/horizonlabs/pulsarfi-backend/src/app/core/agent"
    "github.com/horizonlabs/pulsarfi-backend/src/model"
    "github.com/horizonlabs/pulsarfi-backend/src/service/agent/orchestrator"
    "github.com/horizonlabs/pulsarfi-backend/src/service/agent/supervisor"
)

type graphRecorder struct {
    mu    sync.Mutex
    steps []string
}

func (recorder *graphRecorder) Record(_ context.Context, agentName, stepName, status, _, _ string, _ any) (model.AgentSubTask, error) {
    recorder.mu.Lock()
    defer recorder.mu.Unlock()
    recorder.steps = append(recorder.steps, agentName+"/"+stepName+"/"+status)
    return model.AgentSubTask{Agent: agentName, StepName: stepName}, nil
}

func (recorder *graphRecorder) TerminalHash() string {
    recorder.mu.Lock()
    defer recorder.mu.Unlock()
    return strconv.Itoa(len(recorder.steps))
}

func (recorder *graphRecorder) TaskID() int64 { return 5 }

type graphTaskOpener struct {
    recorder *graphRecorder
}

func (opener *graphTaskOpener) CommitTask(_ context.Context, run *agentcore.RunContext, decision agentcore.RouteDecision) error {
    run.TaskID = 5
    run.Decision = decision
    run.Recorder = opener.recorder
    return nil
}

func (opener *graphTaskOpener) UpdateTask(_ context.Context, run *agentcore.RunContext, decision agentcore.RouteDecision) error {
    run.Decision = decision
    return nil
}

func newGraphOrchestrator(t *testing.T, quasarSteps, novaSteps []*schema.Message) (*orchestrator.Orchestrator, *graphRecorder, *budgetCheckpoints) {
    t.Helper()
    ctx := context.Background()
    recorder := &graphRecorder{}
    store := newBudgetCheckpoints()

    quasar, err := supervisor.New(ctx, &scriptedModel{steps: quasarSteps}, &graphTaskOpener{recorder: recorder})
    if err != nil {
        t.Fatalf("build supervisor: %v", err)
    }

    orch, err := orchestrator.NewOrchestrator(ctx, &orchestrator.Orchestrator{
        Quasar:          quasar,
        Nova:            newAgent(t, "analyzer_agent", novaSteps),
        CheckPointStore: store,
    })
    if err != nil {
        t.Fatalf("build orchestrator: %v", err)
    }

    return orch, recorder, store
}

func sendFresh(t *testing.T, orch *orchestrator.Orchestrator, chatID uuid.UUID, message string) orchestrator.OrchestratorResult {
    t.Helper()
    result, err := orch.Send(context.Background(), orchestrator.OrchestratorInput{
        ChatID:    chatID,
        Wallet:    "0xtest",
        RawPrompt: message,
        Messages:  []*schema.Message{schema.UserMessage(message)},
    }, agentcore.FreshTurnIntent())
    if err != nil {
        t.Fatalf("send: %v", err)
    }

    return result
}

func TestOrchestratorGraph_RunsNovaBetweenTwoQuasarRunsAndRepliesLast(t *testing.T) {
    orch, recorder, store := newGraphOrchestrator(t, []*schema.Message{
        toolCall("understand", routeInformation),
        toolCall("open_task", informationalDecision),
        handOff(),
        answer("", "IHSG is up today."),
    }, []*schema.Message{answer("nova thinking", "IHSG closed higher.")})
    chatID := uuid.New()

    result := sendFresh(t, orch, chatID, "Ambilin chart IHSG")

    if result.TaskID != 5 || result.Reply != "IHSG is up today." {
        t.Fatalf("unexpected result: %+v", result)
    }

    want := "supervisor/route_to_analyzer/done, analyzer/gather_evidence/done"
    if got := strings.Join(recorder.steps, ", "); got != want {
        t.Fatalf("sub task steps differ\n got: %s\nwant: %s", got, want)
    }

    if found, _ := store.Has(context.Background(), chatID.String()); found {
        t.Fatal("a finished turn must not leave a graph checkpoint behind")
    }
}

func TestOrchestratorGraph_PausesOnAskUserAndRecordsTheMarker(t *testing.T) {
    orch, _, store := newGraphOrchestrator(t, []*schema.Message{
        toolCall("understand", routeAnalyzeTrade),
        toolCall("open_task", partialTradeDecision),
        toolCall("ask_user", cardQuestions),
    }, nil)
    chatID := uuid.New()

    result := sendFresh(t, orch, chatID, "Beli BBCA")

    if len(result.PendingQuestions) != 2 || result.ContentType != "workflow_card" {
        t.Fatalf("expected the questions card, got %+v", result)
    }

    if result.Reply != "I need a couple of things confirmed." {
        t.Fatalf("the card must keep its framing sentence, got %q", result.Reply)
    }

    stored, found, _ := store.GetInterruptID(context.Background(), chatID.String())
    if !found || !strings.HasPrefix(stored, "questions|") {
        t.Fatalf("expected a questions marker, got %q found=%v", stored, found)
    }

    if found, _ := store.Has(context.Background(), chatID.String()); !found {
        t.Fatal("a paused turn must keep its graph checkpoint")
    }
}

func TestOrchestratorGraph_AwaitArmWaitsForNovaThenPausesOnTheArmCard(t *testing.T) {
    orch, recorder, store := newGraphOrchestrator(t, []*schema.Message{
        toolCall("understand", routeAnalyzeTrade),
        toolCall("open_task", settledTradeDecision),
        toolCall("await_arm", `{"message":"too early"}`),
        toolCall("await_arm", `{"message":"Review the card and click Arm."}`),
    }, []*schema.Message{answer("nova thinking", "BBCAP is tradeable.\n```json\n{\"tradeable\":true,\"entry_price\":\"100\",\"confidence\":\"high\",\"reasoning\":\"ok\"}\n```")})
    chatID := uuid.New()

    result := sendFresh(t, orch, chatID, "Beli BBCA 1 juta")

    if result.Reply != "Review the card and click Arm." {
        t.Fatalf("expected the Arm Card sentence, got %+v steps=%v", result, recorder.steps)
    }

    stored, _, _ := store.GetInterruptID(context.Background(), chatID.String())
    if !strings.HasPrefix(stored, "arm|") {
        t.Fatalf("expected an arm marker, got %q", stored)
    }

    if strings.Join(recorder.steps, ", ") != "supervisor/route_to_analyzer/done, analyzer/gather_evidence/done, supervisor/await_confirmation/done" {
        t.Fatalf("await_arm must come after Nova, got steps %v", recorder.steps)
    }
}
