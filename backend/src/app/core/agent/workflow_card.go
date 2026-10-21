package agentcore

import "encoding/json"

// WorkflowCard is the final result of a turn: what the handler returns and
// what the chat-stream `final` event carries.
type WorkflowCard struct {
	TaskID      int64           `json:"task_id,omitempty"`
	Reply       string          `json:"reply"`
	ContentType string          `json:"content_type"`
	UIComponent *string         `json:"ui_component,omitempty"`
	UIProps     json.RawMessage `json:"ui_props,omitempty"`
}
