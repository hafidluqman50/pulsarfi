package agentcore

import (
	"sync"
	"time"

	"github.com/google/uuid"
)

// TurnRegistry answers one question, authoritatively for this process: is a
// turn currently running for this chat? It is the guard that stops a retry (or
// a double submit) from starting a second turn while the first one still runs.
//
// It is in-process on purpose: the backend ships as a single container. If it
// is ever run with several instances, the upgrade path is a DB lease claimed
// with one atomic UPDATE ... RETURNING, not a session-scoped advisory lock
// (which would hold a pooled connection for the whole turn).
type TurnRegistry struct {
	mu     sync.Mutex
	active map[uuid.UUID]time.Time
}

func NewTurnRegistry() *TurnRegistry {
	return &TurnRegistry{active: map[uuid.UUID]time.Time{}}
}

// TryStart claims the chat for one turn. ok is false when a turn is already
// running; release is idempotent and must be deferred by the caller.
func (r *TurnRegistry) TryStart(chatID uuid.UUID) (release func(), ok bool) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, running := r.active[chatID]; running {
		return func() {}, false
	}
	r.active[chatID] = time.Now()

	var once sync.Once
	return func() {
		once.Do(func() {
			r.mu.Lock()
			defer r.mu.Unlock()
			delete(r.active, chatID)
		})
	}, true
}

// Status reports whether the chat has a turn in flight, and when it started.
func (r *TurnRegistry) Status(chatID uuid.UUID) (running bool, startedAt time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	startedAt, running = r.active[chatID]
	return running, startedAt
}
