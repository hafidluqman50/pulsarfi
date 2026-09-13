package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/cloudwego/eino/compose"
	"github.com/google/uuid"
	"github.com/horizonlabs/pulsarfi-backend/src/repository"
)

// PostgresCheckPointStore provides durable persistence for CloudWeGo Eino
// graph execution states in PostgreSQL. Implements compose.CheckPointStore.
type PostgresCheckPointStore struct {
	repo *repository.AgentCheckpointRepository
}

func NewPostgresCheckPointStore(repo *repository.AgentCheckpointRepository) *PostgresCheckPointStore {
	return &PostgresCheckPointStore{repo: repo}
}

// Get loads serialized checkpoint bytes for a checkpoint ID (e.g. chatID).
// Implements compose.CheckPointStore.
func (s *PostgresCheckPointStore) Get(ctx context.Context, checkPointID string) ([]byte, bool, error) {
	cp, found, err := s.repo.FindByID(ctx, checkPointID)
	if err != nil || !found {
		return nil, found, err
	}
	return cp.Data, true, nil
}

// Set stores serialized checkpoint bytes for a checkpoint ID.
// Implements compose.CheckPointStore.
func (s *PostgresCheckPointStore) Set(ctx context.Context, checkPointID string, checkPoint []byte) error {
	return s.repo.Upsert(ctx, checkPointID, checkPoint)
}

// Delete removes a checkpoint by ID (e.g. on user cancellation or completed flow).
// If checkPointID does not have the prefix, it also cleans up any pending trade checkpoint.
func (s *PostgresCheckPointStore) Delete(ctx context.Context, checkPointID string) error {
	_ = s.repo.Delete(ctx, pendingTradeKey(checkPointID))
	return s.repo.Delete(ctx, checkPointID)
}

// Has checks whether an active checkpoint exists for the given ID.
// Checks both direct key and pending trade namespaced key.
func (s *PostgresCheckPointStore) Has(ctx context.Context, checkPointID string) (bool, error) {
	_, found, err := s.repo.FindByID(ctx, checkPointID)
	if err != nil || found {
		return found, err
	}
	_, found, err = s.repo.FindByID(ctx, pendingTradeKey(checkPointID))
	return found, err
}

// PendingTradeCheckpoint holds the in-flight parameters for a trade awaiting
// user confirmation (Human-In-The-Loop). Persisted in Postgres so users can
// navigate away, ask side-questions, or resume later without losing trade context.
type PendingTradeCheckpoint struct {
	ChatID           uuid.UUID     `json:"chat_id"`
	MentionedTicker  string        `json:"mentioned_ticker"`
	ResolvedTicker   string        `json:"resolved_ticker"`
	Shape            string        `json:"shape"`
	Side             string        `json:"side"`
	Summary          string        `json:"summary"`
	PendingQuestions []IntakeField `json:"pending_questions"`
	CreatedAt        time.Time     `json:"created_at"`
}

const pendingTradeKeyPrefix = "pending_trade:"

func pendingTradeKey(checkPointID string) string {
	if checkPointID == "" {
		return ""
	}
	if len(checkPointID) >= len(pendingTradeKeyPrefix) && checkPointID[:len(pendingTradeKeyPrefix)] == pendingTradeKeyPrefix {
		return checkPointID
	}
	return pendingTradeKeyPrefix + checkPointID
}

// GetPendingTrade retrieves and deserializes a PendingTradeCheckpoint if present.
// Uses the namespaced "pending_trade:" key to prevent collision with Eino's internal
// graph execution checkpoints.
func (s *PostgresCheckPointStore) GetPendingTrade(ctx context.Context, checkPointID string) (*PendingTradeCheckpoint, bool, error) {
	data, found, err := s.Get(ctx, pendingTradeKey(checkPointID))
	if err != nil {
		return nil, false, err
	}
	if !found {
		// Fallback for legacy checkpoints saved without prefix
		data, found, err = s.Get(ctx, checkPointID)
		if err != nil || !found {
			return nil, found, err
		}
	}
	var cp PendingTradeCheckpoint
	if err := json.Unmarshal(data, &cp); err != nil {
		return nil, true, fmt.Errorf("unmarshal pending trade checkpoint: %w", err)
	}
	return &cp, true, nil
}

// SetPendingTrade serializes and persists a PendingTradeCheckpoint under the
// namespaced "pending_trade:" key, and purges any un-prefixed legacy row to ensure
// Eino's graph runner does not mistake business domain state for an Eino graph checkpoint.
func (s *PostgresCheckPointStore) SetPendingTrade(ctx context.Context, checkPointID string, cp PendingTradeCheckpoint) error {
	data, err := json.Marshal(cp)
	if err != nil {
		return fmt.Errorf("marshal pending trade checkpoint: %w", err)
	}
	// Purge legacy un-prefixed key if present
	_ = s.repo.Delete(ctx, checkPointID)
	return s.Set(ctx, pendingTradeKey(checkPointID), data)
}

// DeletePendingTrade removes a PendingTradeCheckpoint by ID (including legacy key).
func (s *PostgresCheckPointStore) DeletePendingTrade(ctx context.Context, checkPointID string) error {
	_ = s.repo.Delete(ctx, checkPointID)
	return s.repo.Delete(ctx, pendingTradeKey(checkPointID))
}

// HasPendingTrade checks whether an active pending trade checkpoint exists.
func (s *PostgresCheckPointStore) HasPendingTrade(ctx context.Context, checkPointID string) (bool, error) {
	_, found, err := s.repo.FindByID(ctx, pendingTradeKey(checkPointID))
	if err != nil || found {
		return found, err
	}
	_, found, err = s.repo.FindByID(ctx, checkPointID)
	return found, err
}

var _ compose.CheckPointStore = (*PostgresCheckPointStore)(nil)
