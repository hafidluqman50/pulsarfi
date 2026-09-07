package agent

import (
	"context"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
)

type RunContext struct {
	TaskID             int64
	OnChainTaskID      *int64
	Wallet             string
	TriggerDescription string
	SourceMessageID    *int64
	Recorder           *SubTaskRecorder
	NestedToolCalls    []ToolCallTrace
	OnSubTask          func(model.AgentSubTask)
	// OnSubTaskStarted fires the instant a step begins, before its actual
	// work runs — never persisted (SubTaskRecorder only ever writes a step
	// once it's done, so the hash chain stays exactly as before), purely an
	// ephemeral live signal so the UI can show "in progress" instead of a
	// step only ever appearing already finished.
	OnSubTaskStarted func(agentName, stepName, label string)
	OnTextDelta      func(string)
}

type runContextKey struct{}

func WithRunContext(ctx context.Context, rc *RunContext) context.Context {
	return context.WithValue(ctx, runContextKey{}, rc)
}

func RunContextFrom(ctx context.Context) (*RunContext, bool) {
	rc, ok := ctx.Value(runContextKey{}).(*RunContext)
	return rc, ok
}
