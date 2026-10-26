package agentcore

import (
	"context"

	"github.com/google/uuid"
)

// RunContext is threaded via WithRunContext into every tool of the turn —
// Quasar's own (open_task, ask_user, await_arm) and the nested ones of Nova
// and Comet. It is the only channel that reaches code the agent runtime treats
// as a black box.
//
// It carries identity, persistence and the turn's live Stream. Recorder and
// Stream are ports: this package performs no I/O.
type RunContext struct {
	// identity
	Wallet          string
	ChatID          uuid.UUID
	RawPrompt       string
	SourceMessageID *int64

	// persistence
	OnChainTaskID *int64
	TaskID        int64
	Recorder      SubTaskWriter
	Decision      RouteDecision

	// decision anchor recorded with the Arm Card. executeTrade references
	// DecideOnChainID as its on-chain subTaskId; the turn persists all three
	// into the Task's trigger_description so the Arm click can use them.
	DecideSubTaskID int64
	DecideOnChainID uint64
	DecideHash      string

	// live
	Stream TurnStream
}

type runContextKey struct{}

func WithRunContext(ctx context.Context, run *RunContext) context.Context {
	return context.WithValue(ctx, runContextKey{}, run)
}

func RunContextFrom(ctx context.Context) (*RunContext, bool) {
	run, ok := ctx.Value(runContextKey{}).(*RunContext)
	return run, ok
}
