package custodian

import (
	"strconv"

	"github.com/gin-gonic/gin"
	custodianMiddleware "github.com/horizonlabs/pulsarfi-backend/src/http/middleware/custodian"
	"github.com/horizonlabs/pulsarfi-backend/src/http/response"
)

func ListWalletVerificationsHandler(c *gin.Context) {
	status := c.Query("status") // pending | approved | rejected | ""
	records, err := repos.WalletVerification.FindAll(c.Request.Context(), status)
	if err != nil {
		response.InternalError(c, "failed to fetch verifications")
		return
	}
	response.OK(c, "verifications retrieved", records)
}

type updateVerificationBody struct {
	Status string `json:"status" binding:"required,oneof=approved rejected"`
}

func UpdateWalletVerificationHandler(c *gin.Context) {
	claims, ok := custodianMiddleware.Get(c)
	if !ok {
		response.Unauthorized(c, "unauthorized")
		return
	}

	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "invalid id")
		return
	}

	var req updateVerificationBody
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	custodian, found, err := repos.Custodian.FindByWalletAddress(c.Request.Context(), claims.WalletAddress)
	if err != nil || !found {
		response.InternalError(c, "failed to lookup custodian")
		return
	}

	if err := repos.WalletVerification.UpdateStatus(c.Request.Context(), id, req.Status, &custodian.ID); err != nil {
		response.InternalError(c, "failed to update verification")
		return
	}

	response.OK(c, "verification updated", nil)
}
