package agent

import "context"

// RunContext carries the current Task's per-run data through one Supervisor
// invocation, down into whatever tools Analyzer/Executor call along the
// way — Eino's adk.Agent.Run threads ctx through every nested AgentTool
// call, so a value set here at the top is visible to a tool several levels
// deep (e.g. Executor's submit_trade) without rebuilding any agent per Task.
// Agents themselves (Supervisor/Analyzer/Executor) are built once at
// startup and reused across every Task; only this run-scoped data changes
// per invocation.
type RunContext struct {
	TaskID             int64
	OnChainTaskID      *int64 // nil until ArmTask succeeds — submit_trade refuses to fire while nil
	Wallet             string
	TriggerDescription string
	SourceMessageID    *int64 // the chat message this run started from, if any — create_task links a new Task to it
	Recorder           *SubTaskRecorder
}

type runContextKey struct{}

func WithRunContext(ctx context.Context, rc *RunContext) context.Context {
	return context.WithValue(ctx, runContextKey{}, rc)
}

func RunContextFrom(ctx context.Context) (*RunContext, bool) {
	rc, ok := ctx.Value(runContextKey{}).(*RunContext)
	return rc, ok
}
