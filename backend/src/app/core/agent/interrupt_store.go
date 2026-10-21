package agentcore

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

const (
	InterruptQuestions = "questions"
	InterruptArm       = "arm"
)

// PendingInterrupt is the interrupt a chat is currently paused on: its kind
// (questions | arm) and the Eino interrupt id a later resume must target.
type PendingInterrupt struct {
	Kind string
	ID   string
}

// InterruptStore is the single owner of "is this chat paused, and on what".
// It is pure policy over the store port: a pause is recorded under
// "interrupt_id:<chatID>" as "<kind>|<id>".
type InterruptStore struct {
	checkpoints ExtendedCheckPointStore
}

func NewInterruptStore(checkpoints ExtendedCheckPointStore) *InterruptStore {
	return &InterruptStore{checkpoints: checkpoints}
}

// Pending returns the interrupt a chat is paused on, if any.
func (s *InterruptStore) Pending(ctx context.Context, chatID uuid.UUID) (PendingInterrupt, bool) {
	if chatID == uuid.Nil {
		return PendingInterrupt{}, false
	}
	stored, found, err := s.checkpoints.GetInterruptID(ctx, chatID.String())
	if err != nil || !found {
		return PendingInterrupt{}, false
	}
	kind, id, ok := strings.Cut(stored, "|")
	if !ok || id == "" {
		return PendingInterrupt{}, false
	}
	return PendingInterrupt{Kind: kind, ID: id}, true
}

// Record notes the interrupt a chat is paused on.
func (s *InterruptStore) Record(ctx context.Context, chatID uuid.UUID, kind, id string) {
	if chatID == uuid.Nil {
		return
	}
	_ = s.checkpoints.SetInterruptID(ctx, chatID.String(), kind+"|"+id)
}

// IsPaused reports whether the chat has a pending interrupt. This is
// equivalent to "has a checkpoint": finishInterrupt keeps the checkpoint
// exactly when it records an interrupt, and clears both otherwise.
func (s *InterruptStore) IsPaused(ctx context.Context, chatID uuid.UUID) bool {
	_, paused := s.Pending(ctx, chatID)
	return paused
}

// Clear drops the chat's checkpoint together with its interrupt marker.
func (s *InterruptStore) Clear(ctx context.Context, chatID uuid.UUID) {
	if chatID == uuid.Nil {
		return
	}
	_ = s.checkpoints.Delete(ctx, chatID.String())
}
