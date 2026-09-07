package realtime

import (
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/horizonlabs/pulsarfi-backend/src/auth"
)

const (
	writeWait  = 10 * time.Second
	pongWait   = 60 * time.Second
	pingPeriod = (pongWait * 9) / 10
	sendBuffer = 16
)

type client struct {
	conn     wsConn
	send     chan []byte
	topics   map[string]bool
	identity *auth.Claims // nil for an anonymous connection (no token given at all)
}

// Hub is an in-process topic pub/sub broker for pushing data to WebSocket
// clients, replacing per-client HTTP polling (see
// docs/plans/realtime-websocket-updates.md). A publisher calls Publish once;
// every subscriber of that topic gets the update fanned out from memory, so
// N subscribers no longer means N times the backend work. A single Go
// process is enough here — backend/fly.toml runs one machine with no
// autoscale — a multi-machine deployment would need a shared broker (e.g.
// Postgres LISTEN/NOTIFY) instead of this in-memory map.
type Hub struct {
	mu          sync.RWMutex
	clients     map[*client]struct{}
	topics      map[string]map[*client]struct{}
	lastPayload map[string][]byte
}

func NewHub() *Hub {
	return &Hub{
		clients:     map[*client]struct{}{},
		topics:      map[string]map[*client]struct{}{},
		lastPayload: map[string][]byte{},
	}
}

var defaultHub = NewHub()

// Default is the process-wide Hub. A package-level singleton rather than a
// dependency threaded through every call site that might want to publish
// (SubTaskRecorder, TaskService, public services) — those already carry
// enough constructor parameters, and this process only ever runs one Hub.
func Default() *Hub { return defaultHub }

// Publish is a convenience wrapper around Default().Publish.
func Publish(topic string, data any) { defaultHub.Publish(topic, data) }

// Publish marshals data once and fans it out to every current subscriber of
// topic. A slow/stuck client's full send buffer is dropped rather than
// blocking the publisher — the fallback poll on the frontend (§4 of the
// plan doc) is what recovers a client that missed an update this way.
func (h *Hub) Publish(topic string, data any) {
	payload, err := json.Marshal(map[string]any{"type": "update", "topic": topic, "data": data})
	if err != nil {
		slog.Error("realtime: marshal publish payload", "topic", topic, "error", err)
		return
	}

	h.mu.Lock()
	h.lastPayload[topic] = payload
	subs := h.topics[topic]
	targets := make([]*client, 0, len(subs))
	for c := range subs {
		targets = append(targets, c)
	}
	h.mu.Unlock()

	for _, c := range targets {
		select {
		case c.send <- payload:
		default:
			slog.Warn("realtime: client send buffer full, dropping update", "topic", topic)
		}
	}
}

// subscribe adds c to every requested topic c's identity is actually
// allowed to read (isTopicAllowed, authorization_service.go) — a topic the
// caller isn't allowed to see is silently dropped from the subscription
// rather than closing the connection, since one WS connection multiplexes
// many topics and a request for one disallowed topic should not cost the
// others. For each topic actually granted, if a value has already been
// published to it before, immediately sends that cached value — otherwise a
// client that subscribes right after a scheduled fetch would wait up to a
// full interval for its first update (docs/plans/realtime-websocket-updates.md
// §3), the same "last message on a channel" convenience a managed pub/sub
// service like Ably gives for free.
func (h *Hub) subscribe(c *client, topics []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, topic := range topics {
		if !isTopicAllowed(topic, c.identity) {
			continue
		}
		if h.topics[topic] == nil {
			h.topics[topic] = map[*client]struct{}{}
		}
		h.topics[topic][c] = struct{}{}
		c.topics[topic] = true

		if cached, ok := h.lastPayload[topic]; ok {
			select {
			case c.send <- cached:
			default:
			}
		}
	}
}

func (h *Hub) unsubscribe(c *client, topics []string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for _, topic := range topics {
		if subs, ok := h.topics[topic]; ok {
			delete(subs, c)
			if len(subs) == 0 {
				delete(h.topics, topic)
			}
		}
		delete(c.topics, topic)
	}
}

func (h *Hub) removeClient(c *client) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for topic := range c.topics {
		if subs, ok := h.topics[topic]; ok {
			delete(subs, c)
			if len(subs) == 0 {
				delete(h.topics, topic)
			}
		}
	}
	delete(h.clients, c)
}
