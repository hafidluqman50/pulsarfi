package chatrequest

import "github.com/gin-gonic/gin"

type MessageRequest struct {
	Message string `json:"message" binding:"required"`
	// Hidden marks a message that is real chat history (persisted, fed to
	// the LLM as context, included in every read) but must never render as
	// a bubble in the UI — the compiled key:value answer a clarifying-
	// questions card sends, not something the user typed by hand.
	Hidden bool `json:"hidden"`
}

func NewMessageRequest(c *gin.Context) (MessageRequest, error) {
	var req MessageRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return MessageRequest{}, err
	}
	return req, nil
}
