package realtime

import (
	"strings"

	"github.com/horizonlabs/pulsarfi-backend/src/auth"
)

// isTopicAllowed mirrors each topic's own REST equivalent's auth
// requirement (docs/plans/realtime-websocket-updates.md §9) — a public page
// like /markets has no login at all, and its data (market-stocks,
// protocol-stats, reserves) is already served unauthenticated via
// GET /api/v1/public/*, so gating the WebSocket behind a mandatory JWT would
// silently regress that page back to permanent polling for every logged-out
// visitor, the exact audience most likely to trigger the rate-limit bug
// this feature exists to fix. identity is nil for an anonymous connection.
func isTopicAllowed(topic string, identity *auth.Claims) bool {
	switch {
	case topic == "market-stocks", topic == "protocol-stats", topic == "reserves":
		// Matches GET /api/v1/public/stocks, /public/stats, /public/reserves — no auth.
		return true
	case strings.HasPrefix(topic, "agent-task-reasoning:"):
		// Matches GET /api/v1/agent/tasks/:id/reasoning — deliberately public,
		// independently verifiable by anyone (see task_service.go's own doc comment).
		return true
	case strings.HasPrefix(topic, "agent-task-trades:"):
		// Matches GET /api/v1/agent/tasks/:id/trades — any authenticated wallet.
		return identity != nil
	case strings.HasPrefix(topic, "custodian-"):
		// Matches the custodian router's own custodianMiddleware.Auth — role must be "custodian".
		return identity != nil && identity.Role == "custodian"
	default:
		return false
	}
}
