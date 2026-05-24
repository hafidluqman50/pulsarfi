package external

import (
	"sync"
	"time"
)

type EventLevel string

const (
	LevelInfo  EventLevel = "INFO"
	LevelOK    EventLevel = "OK"
	LevelWarn  EventLevel = "WARN"
	LevelError EventLevel = "ERROR"
)

type StreamEvent struct {
	Time    string     `json:"time"`
	Level   EventLevel `json:"level"`
	Tag     string     `json:"tag"`
	Message string     `json:"message"`
}

func newStreamEvent(level EventLevel, tag, message string) StreamEvent {
	return StreamEvent{
		Time:    time.Now().Format("15:04:05.000"),
		Level:   level,
		Tag:     tag,
		Message: message,
	}
}

type broadcaster struct {
	mu      sync.Mutex
	clients map[chan StreamEvent]struct{}
}

func newBroadcaster() *broadcaster {
	return &broadcaster{clients: make(map[chan StreamEvent]struct{})}
}

func (b *broadcaster) subscribe() (chan StreamEvent, func()) {
	ch := make(chan StreamEvent, 16)
	b.mu.Lock()
	b.clients[ch] = struct{}{}
	b.mu.Unlock()

	unsubscribe := func() {
		b.mu.Lock()
		delete(b.clients, ch)
		close(ch)
		b.mu.Unlock()
	}
	return ch, unsubscribe
}

// emit broadcasts to all clients. Slow clients are dropped, never blocked.
func (b *broadcaster) emit(event StreamEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.clients {
		select {
		case ch <- event:
		default:
		}
	}
}
