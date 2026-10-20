package agent

import (
	"context"
	"strings"

	"github.com/google/uuid"
)

// pendingInterrupt is the interrupt a chat is currently paused on: its kind
// (questions | arm) and the Eino interrupt id a later resume must target.
type pendingInterrupt struct {
	kind string
	id   string
}

// interruptStore is the single owner of "is this chat paused, and on what".
//
// A pause is recorded in the same agent_checkpoints table as the Eino
// checkpoint itself, under the key "interrupt_id:<chatID>", with the value
// "<kind>|<id>". This type is the only place that knows that layout, so
// ChatService and Orchestrator ask the same question the same way instead of
// each reading a different key.
type interruptStore struct {
	checkpoints ExtendedCheckPointStore
}

func newInterruptStore(checkpoints ExtendedCheckPointStore) *interruptStore {
	return &interruptStore{checkpoints: checkpoints}
}

// Orchestrator.interrupts and ChatService.interrupts are two views of the
// same store; neither keeps a second copy of the bookkeeping.
func (orch *Orchestrator) interrupts() *interruptStore {
	return newInterruptStore(orch.CheckPointStore)
}

func (svc *ChatService) interrupts() *interruptStore {
	return newInterruptStore(svc.CheckPointStore)
}

// pending returns the interrupt a chat is paused on, if any.
func (s *interruptStore) pending(ctx context.Context, chatID uuid.UUID) (pendingInterrupt, bool) {
	if chatID == uuid.Nil {
		return pendingInterrupt{}, false
	}
	stored, found, err := s.checkpoints.GetInterruptID(ctx, chatID.String())
	if err != nil || !found {
		return pendingInterrupt{}, false
	}
	kind, id, ok := strings.Cut(stored, "|")
	if !ok || id == "" {
		return pendingInterrupt{}, false
	}
	return pendingInterrupt{kind: kind, id: id}, true
}

// record notes the interrupt a chat is paused on.
func (s *interruptStore) record(ctx context.Context, chatID uuid.UUID, kind, id string) {
	if chatID == uuid.Nil {
		return
	}
	_ = s.checkpoints.SetInterruptID(ctx, chatID.String(), kind+"|"+id)
}

// isPaused reports whether the chat has a pending interrupt. This is
// equivalent to "has a checkpoint": finishInterrupt keeps the checkpoint
// exactly when it records an interrupt, and clears both otherwise.
func (s *interruptStore) isPaused(ctx context.Context, chatID uuid.UUID) bool {
	_, paused := s.pending(ctx, chatID)
	return paused
}

// clear drops the chat's checkpoint together with its interrupt marker.
func (s *interruptStore) clear(ctx context.Context, chatID uuid.UUID) {
	if chatID == uuid.Nil {
		return
	}
	_ = s.checkpoints.Delete(ctx, chatID.String())
}
