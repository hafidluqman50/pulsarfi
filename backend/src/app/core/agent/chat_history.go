package agentcore

import (
	"github.com/cloudwego/eino/schema"
	"github.com/horizonlabs/pulsarfi-backend/src/model"
)

// ChatHistory turns stored chat messages into the conversation the model
// reads: the user's messages as user turns, everything else as assistant.
func ChatHistory(messages []model.AgentChatMessage) []*schema.Message {
	history := make([]*schema.Message, len(messages))
	for i, message := range messages {
		if message.Sender == "user" {
			history[i] = schema.UserMessage(message.Content)
		} else {
			history[i] = schema.AssistantMessage(message.Content, nil)
		}
	}
	return history
}
