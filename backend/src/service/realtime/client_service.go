package realtime

import (
	"encoding/json"
	"time"

	"github.com/gorilla/websocket"
	"github.com/horizonlabs/pulsarfi-backend/src/auth"
)

// wsConn is Hub's own narrow view of *websocket.Conn (ISP) — just the
// methods the read/write pumps actually call, so hub_service.go's pub/sub
// logic itself never needs to import gorilla/websocket.
type wsConn interface {
	ReadMessage() (messageType int, p []byte, err error)
	WriteMessage(messageType int, data []byte) error
	SetReadLimit(limit int64)
	SetReadDeadline(t time.Time) error
	SetWriteDeadline(t time.Time) error
	SetPongHandler(h func(appData string) error)
	Close() error
}

type subscribeMessage struct {
	Type   string   `json:"type"`
	Topics []string `json:"topics"`
}

// Serve registers conn with the hub and blocks until the connection closes.
// identity is nil for an anonymous connection (no token given at all) —
// public topics still work for it (isTopicAllowed, authorization_service.go);
// a token that was given but failed to parse is the caller's
// responsibility to reject before ever calling Serve, not this function's.
// The read pump handles subscribe/unsubscribe messages from the client; the
// write pump forwards published updates and sends a periodic ping so an
// idle-but-alive connection is not mistaken for dead by an intermediary
// proxy (docs/plans/realtime-websocket-updates.md §5).
func (h *Hub) Serve(conn wsConn, identity *auth.Claims) {
	c := &client{conn: conn, send: make(chan []byte, sendBuffer), topics: map[string]bool{}, identity: identity}

	h.mu.Lock()
	h.clients[c] = struct{}{}
	h.mu.Unlock()

	done := make(chan struct{})
	go h.writePump(c, done)
	h.readPump(c)
	close(done)
	h.removeClient(c)
}

func (h *Hub) readPump(c *client) {
	defer c.conn.Close()
	c.conn.SetReadLimit(4096)
	_ = c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		return c.conn.SetReadDeadline(time.Now().Add(pongWait))
	})

	for {
		_, raw, err := c.conn.ReadMessage()
		if err != nil {
			return
		}
		var msg subscribeMessage
		if err := json.Unmarshal(raw, &msg); err != nil {
			continue
		}
		switch msg.Type {
		case "subscribe":
			h.subscribe(c, msg.Topics)
		case "unsubscribe":
			h.unsubscribe(c, msg.Topics)
		}
	}
}

func (h *Hub) writePump(c *client, done <-chan struct{}) {
	ticker := time.NewTicker(pingPeriod)
	defer ticker.Stop()
	defer c.conn.Close()

	for {
		select {
		case msg := <-c.send:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.TextMessage, msg); err != nil {
				return
			}
		case <-ticker.C:
			_ = c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-done:
			return
		}
	}
}
