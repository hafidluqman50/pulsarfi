package agentcore

import (
	"context"

	"github.com/cloudwego/eino/compose"
	"github.com/horizonlabs/pulsarfi-backend/src/model"
)

// SubTaskWriter is the recorder port. RunContext carries one; the domain does
// not care that the implementation writes Postgres and the chain.
type SubTaskWriter interface {
	Record(ctx context.Context, agentName, stepName, status, reasoning, label string, output any) (model.AgentSubTask, error)
	TerminalHash() string
	TaskID() int64
}

// TurnStream is the live-signal port. The orchestrator's implementation
// publishes over the chat WebSocket; the domain only knows the four calls.
type TurnStream interface {
	StepStarted(agentName, stepName, label string)
	SetCurrent(agentName, stepName string)
	StepFailed(reason string)
	StepDone(row model.AgentSubTask)
}

// ChainSubTaskRecorder is the narrow chain capability the recorder needs.
type ChainSubTaskRecorder interface {
	RecordSubTasks(ctx context.Context, onChainTaskID uint64, rows []model.AgentSubTask) ([]uint64, string, error)
}

// ExtendedCheckPointStore is Eino's checkpoint store plus the bookkeeping the
// agent needs. The Postgres implementation lives in service/agent/checkpoint.
type ExtendedCheckPointStore interface {
	compose.CheckPointStore
	Delete(ctx context.Context, checkPointID string) error
	Has(ctx context.Context, checkPointID string) (bool, error)
	GetInterruptID(ctx context.Context, checkPointID string) (string, bool, error)
	SetInterruptID(ctx context.Context, checkPointID string, interruptID string) error
}
