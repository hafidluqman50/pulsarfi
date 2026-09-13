package chatrequest

import "github.com/gin-gonic/gin"

type MessageRequest struct {
	Message string `json:"message" binding:"required"`
}

func NewMessageRequest(c *gin.Context) (MessageRequest, error) {
	var req MessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return MessageRequest{}, err
	}
	return req, nil
}
