package taskrequest

import "github.com/gin-gonic/gin"

type ArmRequest struct {
	TotalBudget string `json:"total_budget" binding:"required"`
	DurationSec int64  `json:"duration_sec" binding:"required,gt=0"`
	// TokenAddress is the ERC20 the owner's own approve() covers, and it is
	// side-dependent: IDRX for a Buy, the stock token itself for a Sell
	// (AgentTaskManager.executeTrade pulls a different token per side).
	// Optional at this layer — an informational Task arms without any token
	// at all — but ArmTaskInput has carried the field all along with nothing
	// ever populating it, so it was always empty before this
	// (docs/plans/fix-comet-trade-execution-blockers.md Defect C).
	TokenAddress string `json:"token_address"`
}

func NewArmRequest(c *gin.Context) (ArmRequest, error) {
	var req ArmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return ArmRequest{}, err
	}
	return req, nil
}
