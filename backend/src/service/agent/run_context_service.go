package agent

import (
	"context"

	"github.com/google/uuid"
)

// RunContext is threaded via WithRunContext into every tool of the turn —
// Quasar's own (open_task, ask_user, await_arm) and the nested ones of Nova
// and Comet (get_portfolio_snapshot, submit_trade). It is the only channel
// that reaches code the agent runtime treats as a black box. It is rebuilt
// for every request, never checkpointed: a resumed turn gets its Task back
// from the database (see Orchestrator.restoreRunContext).
type RunContext struct {
	OnChainTaskID *int64
	Wallet        string
	Recorder      *SubTaskRecorder
	Locale        string

	TaskID          int64
	ChatID          uuid.UUID
	RawPrompt       string
	SourceMessageID *int64
	Decision        RouteDecision

	// OnSubTaskStarted fires the instant a step begins, before its actual
	// work runs — never persisted (SubTaskRecorder only ever writes a step
	// once it's done, so the hash chain stays exactly as before), purely an
	// ephemeral live signal so the UI can show "in progress" instead of a
	// step only ever appearing already finished.
	OnSubTaskStarted func(agentName, stepName, label string)
	// OnSubTaskFailed fires when WrapToolGraceful catches a tool error —
	// read by gracefulTool.InvokableRun via RunContextFrom(ctx), which has
	// no other way to reach the live sub-task list. Never persisted.
	OnSubTaskFailed func(agentName, stepName, reason string)
	// CurrentAgent/CurrentStepName identify which step is currently running
	// its tool-calling loop — the only way gracefulTool.InvokableRun (which
	// only knows the failing tool's own name) can attribute a caught
	// failure to the right step.
	CurrentAgent    string
	CurrentStepName string

	live *liveSubTasks
}

type runContextKey struct{}

func WithRunContext(ctx context.Context, rc *RunContext) context.Context {
	return context.WithValue(ctx, runContextKey{}, rc)
}

func RunContextFrom(ctx context.Context) (*RunContext, bool) {
	rc, ok := ctx.Value(runContextKey{}).(*RunContext)
	return rc, ok
}
