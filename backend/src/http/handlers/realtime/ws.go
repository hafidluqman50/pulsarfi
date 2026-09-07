package realtime

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"github.com/horizonlabs/pulsarfi-backend/src/auth"
	realtimesvc "github.com/horizonlabs/pulsarfi-backend/src/service/realtime"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	// CORS on the initial HTTP upgrade is already enforced by
	// middleware.CORS() ahead of this handler in the same router; the
	// browser's native WebSocket API sends no preflight and honors no
	// Access-Control-* response headers, so Origin-checking here would only
	// add a false sense of extra protection.
	CheckOrigin: func(r *http.Request) bool { return true },
}

// StreamHandler upgrades the request to a WebSocket and blocks for the
// connection's lifetime. Auth is a query param (?token=), not the usual
// Authorization header — the browser's native WebSocket constructor cannot
// set custom request headers on the upgrade request.
//
// The token itself is optional: a public page (e.g. /markets) connects with
// none at all, and still gets every topic that page's own REST endpoints
// already serve without login — Hub.Serve enforces per-topic, not
// per-connection, auth (authorization_service.go). A token that *was*
// given but fails to parse is still rejected outright here, since that
// signals a broken/expired client session, not a deliberately anonymous one.
func StreamHandler(jwtConfig auth.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		var identity *auth.Claims
		if token := c.Query("token"); token != "" {
			claims, err := auth.ParseAccessToken(jwtConfig, token)
			if err != nil {
				c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid or expired token"})
				return
			}
			identity = claims
		}

		conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
		if err != nil {
			return
		}
		realtimesvc.Default().Serve(conn, identity)
	}
}
