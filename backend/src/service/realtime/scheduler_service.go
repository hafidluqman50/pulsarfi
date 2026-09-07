package realtime

import (
	"context"
	"log/slog"
	"time"
)

// SchedulePublish runs fetch once per interval and publishes its result to
// topic — the fetch happens exactly once per interval no matter how many
// clients are subscribed, replacing what used to be one HTTP poll per
// client (docs/plans/realtime-websocket-updates.md §3). Skips the fetch
// entirely whenever nobody is currently subscribed to topic, so an idle
// topic costs nothing. Stops when ctx is cancelled.
func (h *Hub) SchedulePublish(ctx context.Context, topic string, interval time.Duration, fetch func(ctx context.Context) (any, error)) {
	publishOnce := func() {
		if !h.hasSubscribers(topic) {
			return
		}
		data, err := fetch(ctx)
		if err != nil {
			slog.Error("realtime: scheduled fetch failed", "topic", topic, "error", err)
			return
		}
		h.Publish(topic, data)
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				publishOnce()
			}
		}
	}()
}

func (h *Hub) hasSubscribers(topic string) bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.topics[topic]) > 0
}
