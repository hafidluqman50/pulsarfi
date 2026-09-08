package agent

import "context"

// RunContext is threaded via WithRunContext into Nova's/Comet's own nested
// tool calls (get_portfolio_snapshot, submit_trade) — the only channel that
// survives the call chain into code the orchestrator treats as a black box
// (docs/plans/agent-orchestration-graph-rebuild.md §3). Trimmed in v2.7 of
// that plan to only the fields actually read inside analyzer/executor,
// confirmed by grep per field, not per file — OnChainTaskID (submit_trade's
// budget check), Wallet (get_portfolio_snapshot), Recorder (submit_trade's
// own decide/execute rows), OnSubTaskStarted (submit_trade's live
// decide/execute-starting signal). TaskID, TriggerDescription,
// SourceMessageID, NestedToolCalls, OnSubTask, and OnTextDelta were set by
// orchestrator_service.go but never read back through RunContext by
// anything live — orchestratorTurn already carries its own copies of that
// same data and uses those directly instead.
type RunContext struct {
	OnChainTaskID *int64
	Wallet        string
	Recorder      *SubTaskRecorder
	// OnSubTaskStarted fires the instant a step begins, before its actual
	// work runs — never persisted (SubTaskRecorder only ever writes a step
	// once it's done, so the hash chain stays exactly as before), purely an
	// ephemeral live signal so the UI can show "in progress" instead of a
	// step only ever appearing already finished.
	OnSubTaskStarted func(agentName, stepName, label string)
}

type runContextKey struct{}

func WithRunContext(ctx context.Context, rc *RunContext) context.Context {
	return context.WithValue(ctx, runContextKey{}, rc)
}

func RunContextFrom(ctx context.Context) (*RunContext, bool) {
	rc, ok := ctx.Value(runContextKey{}).(*RunContext)
	return rc, ok
}
