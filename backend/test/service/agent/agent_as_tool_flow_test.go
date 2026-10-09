package agent_test

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/cloudwego/eino/schema"
	"github.com/google/uuid"
	"github.com/horizonlabs/pulsarfi-backend/src/config"
	"github.com/horizonlabs/pulsarfi-backend/src/model"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
	agentsvc "github.com/horizonlabs/pulsarfi-backend/src/service/agent"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/supervisor"
)

// Flow tests for docs/plans/agent-as-tool-orchestration.md section 4: the
// real Orchestrator, ChatService, supervisor tools, SubTaskRecorder and
// Postgres checkpoint store, driven by scripted models and a fake chain. They
// need a disposable Postgres in TEST_DATABASE_URL and deliberately never read
// DATABASE_URL, so they cannot touch a shared database by accident.

type fakeChain struct {
	mu             sync.Mutex
	nextTask       uint64
	nextSubTask    uint64
	createdTasks   int
	markActionable int
}

func (c *fakeChain) CreateTask(_ context.Context, _ string, _ bool, _ string, _ [32]byte) (uint64, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.nextTask++
	c.createdTasks++
	return c.nextTask, nil
}

func (c *fakeChain) MarkActionable(_ context.Context, _ uint64) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.markActionable++
	return nil
}

func (c *fakeChain) RecordSubTasks(_ context.Context, _ uint64, rows []model.AgentSubTask) ([]uint64, string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	ids := make([]uint64, len(rows))
	for i := range rows {
		c.nextSubTask++
		ids[i] = c.nextSubTask
	}
	return ids, "0xfake", nil
}

type flowHarness struct {
	t            *testing.T
	repos        *repository.Registry
	chain        *fakeChain
	store        *agentsvc.PostgresCheckPointStore
	orchestrator *agentsvc.Orchestrator
	chat         *agentsvc.ChatService
	wallet       string
	chatID       uuid.UUID
}

func newFlowHarness(t *testing.T, quasarSteps, novaSteps, cometSteps []*schema.Message) *flowHarness {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not set, skipping flow test")
	}
	ctx := context.Background()
	db, err := config.NewDatabase(databaseURL)
	if err != nil {
		t.Fatalf("connect test database: %v", err)
	}
	repos := repository.NewRegistry(db)
	chain := &fakeChain{}
	store := agentsvc.NewPostgresCheckPointStore(repos.AgentCheckpoint)

	orchestrator := &agentsvc.Orchestrator{
		Tasks:           repos.AgentTask,
		SubTasks:        repos.AgentSubTask,
		ChatMessages:    repos.AgentChatMessage,
		Chain:           chain,
		CheckPointStore: store,
	}
	quasar, err := supervisor.New(ctx, &scriptedModel{steps: quasarSteps},
		newAgent(t, "analyzer_agent", novaSteps), newAgent(t, "executor_agent", cometSteps),
		orchestrator, repos.AgentTask)
	if err != nil {
		t.Fatalf("build supervisor: %v", err)
	}
	orchestrator.Quasar = quasar
	if orchestrator, err = agentsvc.NewOrchestrator(ctx, orchestrator); err != nil {
		t.Fatalf("build orchestrator: %v", err)
	}

	return &flowHarness{
		t: t, repos: repos, chain: chain, store: store, orchestrator: orchestrator,
		chat: &agentsvc.ChatService{
			Chats: repos.AgentChat, ChatMessages: repos.AgentChatMessage,
			Orchestrator: orchestrator, CheckPointStore: store,
		},
		wallet: "0x" + strings.ReplaceAll(uuid.NewString(), "-", "") + "00000000",
		chatID: uuid.New(),
	}
}

func (h *flowHarness) say(message string) agentsvc.WorkflowCard {
	h.t.Helper()
	card, err := h.chat.HandleChatMessage(context.Background(), h.chatID, h.wallet, message, false)
	if err != nil {
		h.t.Fatalf("HandleChatMessage(%q): %v", message, err)
	}
	return card
}

func (h *flowHarness) steps(taskID int64) []string {
	h.t.Helper()
	rows, err := h.repos.AgentSubTask.FindByTaskID(context.Background(), taskID)
	if err != nil {
		h.t.Fatalf("load sub tasks: %v", err)
	}
	var names []string
	for i, row := range rows {
		names = append(names, row.Agent+"/"+row.StepName)
		if i > 0 && row.PrevDecisionHash != rows[i-1].DecisionHash {
			h.t.Fatalf("hash chain broken between step %d (%s) and %d (%s)", i-1, rows[i-1].StepName, i, row.StepName)
		}
	}
	return names
}

func (h *flowHarness) pendingKind() string {
	stored, found, err := h.store.GetInterruptID(context.Background(), h.chatID.String())
	if err != nil || !found {
		return ""
	}
	kind, _, _ := strings.Cut(stored, "|")
	return kind
}

func (h *flowHarness) arm(taskID int64) {
	h.t.Helper()
	if err := h.repos.AgentTask.SetArmedAt(context.Background(), taskID, time.Now()); err != nil {
		h.t.Fatalf("arm task: %v", err)
	}
}

func equalSteps(t *testing.T, got, want []string) {
	t.Helper()
	if strings.Join(got, ", ") != strings.Join(want, ", ") {
		t.Fatalf("sub task steps differ\n got: %v\nwant: %v", got, want)
	}
}

const (
	informationalDecision = `{"is_actionable":false,"summary":"IHSG chart","label":"IHSG chart"}`
	partialTradeDecision  = `{"is_actionable":false,"summary":"Buy BBCA","label":"Buy BBCA","side":"buy","mentioned_ticker":"BBCAP"}`
	settledTradeDecision  = `{"is_actionable":true,"summary":"Buy 1000000 IDRX of BBCAP","label":"Buy BBCAP","shape":"scalp","side":"buy","mentioned_ticker":"BBCAP","budget_idrx":"1000000"}`
	cardQuestions         = `{"message":"I need a couple of things confirmed.","questions":[{"key":"shape","question":"Which style?","options":["scalp","swing","investment"]},{"key":"idrx_cap","question":"How much?"}]}`
)

func withContent(content string, calls ...*schema.Message) *schema.Message {
	merged := calls[0]
	merged.Content = content
	return merged
}

// Scenario A: plain conversation opens no Task and calls no tool.
func TestFlowA_GreetingOpensNoTask(t *testing.T) {
	h := newFlowHarness(t, []*schema.Message{answer("", "Halo! Ada yang bisa dibantu?")}, nil, nil)

	card := h.say("Halo")

	if card.TaskID != 0 || card.Reply != "Halo! Ada yang bisa dibantu?" {
		t.Fatalf("unexpected card: %+v", card)
	}
	if h.chain.createdTasks != 0 {
		t.Fatalf("a greeting created %d on-chain tasks", h.chain.createdTasks)
	}
}

// Scenario B: an information request still opens an on-chain Task, then asks Nova.
func TestFlowB_InformationOpensATaskAndCallsNova(t *testing.T) {
	h := newFlowHarness(t, []*schema.Message{
		toolCall("open_task", informationalDecision),
		toolCall("analyzer_agent", `{"request":"IHSG chart"}`),
		answer("", "IHSG is up today."),
	}, []*schema.Message{answer("nova thinking", "IHSG closed higher.")}, nil)

	card := h.say("Ambilin chart IHSG")

	if card.TaskID == 0 || card.Reply != "IHSG is up today." {
		t.Fatalf("unexpected card: %+v", card)
	}
	equalSteps(t, h.steps(card.TaskID), []string{
		"supervisor/understand_request", "supervisor/route_decision",
		"supervisor/route_to_analyzer", "analyzer/gather_evidence",
	})
	task, _, _ := h.repos.AgentTask.FindByID(context.Background(), card.TaskID)
	if task.IsActionable {
		t.Fatal("an information task must not be actionable")
	}
	if found, _ := h.store.Has(context.Background(), h.chatID.String()); found {
		t.Fatal("a finished turn must not leave a checkpoint behind")
	}
}

// Task-first and Arm guards: a refused call returns a tool_error, runs
// nothing, and the turn carries on.
func TestFlowGuards_TaskFirstAndArmedAreEnforcedInCode(t *testing.T) {
	h := newFlowHarness(t, []*schema.Message{
		toolCall("analyzer_agent", `{"request":"too early"}`),
		toolCall("open_task", settledTradeDecision),
		toolCall("executor_agent", `{"request":"execute now"}`),
		answer("", "Please arm the task first."),
	}, []*schema.Message{answer("", "must not run")}, []*schema.Message{answer("", "must not run")})

	card := h.say("Beli BBCA 1 juta")

	if card.Reply != "Please arm the task first." {
		t.Fatalf("unexpected reply: %q", card.Reply)
	}
	equalSteps(t, h.steps(card.TaskID), []string{"supervisor/understand_request", "supervisor/route_decision"})
}

// Nova's request is built from the Task, so a trade still being configured
// must be settled with update_task before Nova is asked.
func TestFlowGuards_NovaWaitsForASettledTrade(t *testing.T) {
	h := newFlowHarness(t, []*schema.Message{
		toolCall("open_task", partialTradeDecision),
		toolCall("analyzer_agent", `{"request":"evaluate BBCAP"}`),
		answer("", "Let me confirm the details first."),
	}, []*schema.Message{answer("", "must not run")}, nil)

	card := h.say("Beli BBCA")

	equalSteps(t, h.steps(card.TaskID), []string{"supervisor/understand_request", "supervisor/route_decision"})
}

// Scenarios D and the Arm pause: questions card, resume with the answer,
// Nova, Arm Card, then ExecuteTask's resume streams Comet. The Sub Task hash
// chain must come out unbroken and in the order the graph used to record.
func TestFlowD_QuestionsThenAnalysisThenArmThenExecute(t *testing.T) {
	h := newFlowHarness(t, []*schema.Message{
		toolCall("open_task", partialTradeDecision),
		toolCall("ask_user", cardQuestions),
		toolCall("update_task", settledTradeDecision),
		toolCall("analyzer_agent", `{"request":"evaluate BBCAP"}`),
		withContent("BBCA looks fine. Review the card and click Arm.", toolCall("await_arm", `{"message":"ignored, the model already wrote text"}`)),
		toolCall("executor_agent", `{"request":"buy 1000000 IDRX of BBCAP"}`),
		answer("", "Done. Tx 0xabc"),
	}, []*schema.Message{answer("nova thinking", "BBCAP is tradeable.\n```json\n{\"tradeable\":true,\"entry_price\":\"100\",\"confidence\":\"high\",\"reasoning\":\"ok\"}\n```")},
		[]*schema.Message{answer("comet thinking", "executed")})

	first := h.say("Beli BBCA")
	if first.ContentType != "workflow_card" || first.UIComponent == nil || *first.UIComponent != "clarifying_questions" {
		t.Fatalf("turn 1 should raise the questions card, got %+v", first)
	}
	if first.TaskID == 0 || first.Reply != "I need a couple of things confirmed." {
		t.Fatalf("turn 1 should open the Task before the card and keep the framing sentence: %+v", first)
	}
	if h.pendingKind() != "questions" {
		t.Fatalf("turn 1 should be paused on the questions card, pending=%q", h.pendingKind())
	}

	second := h.say("scalp, 1000000, analyze first")
	if second.TaskID != first.TaskID || second.Reply != "BBCA looks fine. Review the card and click Arm." {
		t.Fatalf("turn 2 should resume the same Task and point at the Arm Card: %+v", second)
	}
	if h.pendingKind() != "arm" {
		t.Fatalf("turn 2 should be paused on the Arm Card, pending=%q", h.pendingKind())
	}
	if h.chain.createdTasks != 1 || h.chain.markActionable != 1 {
		t.Fatalf("one on-chain Task and one markActionable expected, got create=%d mark=%d", h.chain.createdTasks, h.chain.markActionable)
	}

	h.arm(first.TaskID)
	result, err := h.orchestrator.ResumeExecute(context.Background(), first.TaskID, h.wallet)
	if err != nil {
		t.Fatalf("ResumeExecute: %v", err)
	}
	if result.Reply != "Done. Tx 0xabc" {
		t.Fatalf("unexpected execution reply: %q", result.Reply)
	}

	equalSteps(t, h.steps(first.TaskID), []string{
		"supervisor/understand_request", "supervisor/route_decision", "supervisor/route_decision",
		"supervisor/route_to_analyzer", "analyzer/gather_evidence",
		"supervisor/await_confirmation", "supervisor/route_to_executor", "executor/decide",
	})
	if found, _ := h.store.Has(context.Background(), h.chatID.String()); found {
		t.Fatal("the checkpoint must be gone once the execution finished")
	}
}

// Scenario H: a chat message while the Arm Card is open and the Task is not
// armed must neither resume the Arm pause nor destroy it.
func TestFlowH_ChatWhileArmPendingDoesNotResumeOrDestroyThePause(t *testing.T) {
	h := newFlowHarness(t, []*schema.Message{
		toolCall("open_task", settledTradeDecision),
		toolCall("await_arm", `{"message":"Review the card and click Arm."}`),
		answer("", "The card is waiting for you."),
		toolCall("executor_agent", `{"request":"buy 1000000 IDRX of BBCAP"}`),
		answer("", "Done. Tx 0xabc"),
	}, nil, []*schema.Message{answer("", "executed")})

	first := h.say("Beli BBCA 1 juta, langsung eksekusi")
	if h.pendingKind() != "arm" {
		t.Fatalf("expected the Arm pause, pending=%q", h.pendingKind())
	}
	if first.Reply != "Review the card and click Arm." {
		t.Fatalf("the Arm Card must carry its introducing sentence even when the model wrote no text: %q", first.Reply)
	}

	chatter := h.say("sudah belum?")
	if chatter.Reply != "The card is waiting for you." {
		t.Fatalf("a chat message while Arm is pending should be an ordinary turn: %+v", chatter)
	}
	if h.pendingKind() != "arm" {
		t.Fatalf("the Arm pause must survive a chat message, pending=%q", h.pendingKind())
	}
	equalSteps(t, h.steps(first.TaskID), []string{
		"supervisor/understand_request", "supervisor/route_decision", "supervisor/await_confirmation",
	})

	h.arm(first.TaskID)
	if _, err := h.orchestrator.ResumeExecute(context.Background(), first.TaskID, h.wallet); err != nil {
		t.Fatalf("the Arm pause should still resume after the chat message: %v", err)
	}
}

// ResumeExecute with no pending pause (what a recurring tranche hits after
// the first execution) is an explicit error, never a silent reply.
func TestFlowResumeExecuteWithoutPendingPauseIsAnExplicitError(t *testing.T) {
	h := newFlowHarness(t, []*schema.Message{
		toolCall("open_task", informationalDecision),
		answer("", "ok"),
	}, nil, nil)
	card := h.say("tolong catat ini")
	h.arm(card.TaskID)

	_, err := h.orchestrator.ResumeExecute(context.Background(), card.TaskID, h.wallet)

	if err == nil || !strings.Contains(err.Error(), agentsvc.ErrNoPendingExecution.Error()) {
		t.Fatalf("expected ErrNoPendingExecution, got %v", err)
	}
}
