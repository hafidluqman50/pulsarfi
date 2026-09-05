package taskrequest

import "github.com/gin-gonic/gin"

type ArmRequest struct {
	TotalBudget string `json:"total_budget" binding:"required"`
	DurationSec int64  `json:"duration_sec" binding:"required,gt=0"`
}

func NewArmRequest(c *gin.Context) (ArmRequest, error) {
	var req ArmRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return ArmRequest{}, err
	}
	return req, nil
}
