package agentcore

import (
	"encoding/json"

	"github.com/horizonlabs/pulsarfi-backend/src/model"
)

// WorkflowCard is the final result of a turn: what the handler returns and
// what the chat-stream `final` event carries. Messages holds the chat rows
// this call created, so a client can append them without refetching.
type WorkflowCard struct {
	TaskID      int64                    `json:"task_id,omitempty"`
	Reply       string                   `json:"reply"`
	ContentType string                   `json:"content_type"`
	UIComponent *string                  `json:"ui_component,omitempty"`
	UIProps     json.RawMessage          `json:"ui_props,omitempty"`
	Messages    []model.AgentChatMessage `json:"messages,omitempty"`
}
