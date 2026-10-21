package agentcore

import "github.com/horizonlabs/pulsarfi-backend/src/model"

// ExecuteTaskResult is what resuming the Arm pause produces: the Task id, the
// resulting status, the supervisor's reply, and the trades recorded.
type ExecuteTaskResult struct {
	TaskID int64              `json:"task_id"`
	Status string             `json:"status"`
	Reply  string             `json:"reply"`
	Trades []model.AgentTrade `json:"trades,omitempty"`
}
