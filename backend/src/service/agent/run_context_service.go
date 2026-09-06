package agent

import (
	"context"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
)

type RunContext struct {
	TaskID                 int64
	OnChainTaskID          *int64
	LastKnownTaskID        *int64
	LastKnownOnChainTaskID *int64
	Wallet                 string
	TriggerDescription     string
	SourceMessageID        *int64
	Recorder               *SubTaskRecorder
	NestedToolCalls        []ToolCallTrace
	OnSubTask              func(model.AgentSubTask)
	OnTextDelta            func(string)
}

type runContextKey struct{}

func WithRunContext(ctx context.Context, rc *RunContext) context.Context {
	return context.WithValue(ctx, runContextKey{}, rc)
}

func RunContextFrom(ctx context.Context) (*RunContext, bool) {
	rc, ok := ctx.Value(runContextKey{}).(*RunContext)
	return rc, ok
}
