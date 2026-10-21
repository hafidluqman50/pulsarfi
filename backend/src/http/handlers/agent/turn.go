package agent

import (
	"errors"
	agentcore "github.com/horizonlabs/pulsarfi-backend/src/app/core/agent"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	usermw "github.com/horizonlabs/pulsarfi-backend/src/http/middleware/user"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
)

// GetTurnStatusHandler answers the one question a client that has just mounted
// (reload, route return, second tab) cannot answer locally: is a turn already
// running for this chat? The `turn` WebSocket events are best-effort live
// signals; this endpoint is the source of truth.
func GetTurnStatusHandler(c *gin.Context) {
	if !ensureChatService(c) {
		return
	}
	claims, ok := usermw.Get(c)
	if !ok {
		response.Unauthorized(c, "authentication required")
		return
	}
	chatID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		response.BadRequest(c, "invalid chat id")
		return
	}

	running, startedAt, err := chatSvc.TurnStatus(c.Request.Context(), chatID, claims.WalletAddress)
	if errors.Is(err, agentcore.ErrWalletMismatch) {
		response.Forbidden(c, "chat does not belong to the authenticated wallet")
		return
	}
	if err != nil {
		response.InternalError(c, "failed to fetch turn status")
		return
	}

	data := gin.H{"running": running}
	if !startedAt.IsZero() {
		data["started_at"] = startedAt
	}
	response.OK(c, "turn status", data)
}
