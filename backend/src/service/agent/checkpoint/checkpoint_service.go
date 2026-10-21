package checkpoint

import (
	"context"

	"github.com/cloudwego/eino/compose"
	agentcore "github.com/horizonlabs/pulsarfi-backend/src/app/core/agent"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
)

// PostgresCheckPointStore is the sole pause/resume persistence mechanism for
// the agent orchestrator graph — durable storage for CloudWeGo Eino's own
// compose.CheckPointStore, plus the small amount of extra bookkeeping
// (interrupt ID correlation) needed to target a resume at the right pause
// point later.
type PostgresCheckPointStore struct {
	repo *repository.AgentCheckpointRepository
}

func NewPostgresCheckPointStore(repo *repository.AgentCheckpointRepository) *PostgresCheckPointStore {
	return &PostgresCheckPointStore{repo: repo}
}

// Get loads serialized checkpoint bytes for a checkpoint ID (the chat ID).
func (s *PostgresCheckPointStore) Get(ctx context.Context, checkPointID string) ([]byte, bool, error) {
	cp, found, err := s.repo.FindByID(ctx, checkPointID)
	if err != nil || !found {
		return nil, found, err
	}
	return cp.Data, true, nil
}

// Set stores serialized checkpoint bytes for a checkpoint ID.
func (s *PostgresCheckPointStore) Set(ctx context.Context, checkPointID string, checkPoint []byte) error {
	return s.repo.Upsert(ctx, checkPointID, checkPoint)
}

// Delete removes a checkpoint and its associated interrupt ID.
func (s *PostgresCheckPointStore) Delete(ctx context.Context, checkPointID string) error {
	_ = s.repo.Delete(ctx, interruptKey(checkPointID))
	return s.repo.Delete(ctx, checkPointID)
}

// Has reports whether an active checkpoint exists for the given ID.
func (s *PostgresCheckPointStore) Has(ctx context.Context, checkPointID string) (bool, error) {
	_, found, err := s.repo.FindByID(ctx, checkPointID)
	return found, err
}

func interruptKey(checkPointID string) string {
	return "interrupt_id:" + checkPointID
}

// GetInterruptID retrieves the Eino interrupt ID a paused checkpoint is
// waiting to be resumed with.
func (s *PostgresCheckPointStore) GetInterruptID(ctx context.Context, checkPointID string) (string, bool, error) {
	data, found, err := s.Get(ctx, interruptKey(checkPointID))
	if err != nil || !found {
		return "", found, err
	}
	return string(data), true, nil
}

// SetInterruptID records which Eino interrupt ID a checkpoint is currently
// paused on, so a later resume call knows what to target.
func (s *PostgresCheckPointStore) SetInterruptID(ctx context.Context, checkPointID string, interruptID string) error {
	return s.Set(ctx, interruptKey(checkPointID), []byte(interruptID))
}

var _ compose.CheckPointStore = (*PostgresCheckPointStore)(nil)
var _ agentcore.ExtendedCheckPointStore = (*PostgresCheckPointStore)(nil)
