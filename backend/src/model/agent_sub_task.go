package model

import "time"

// AgentSubTask is one link in a Task's hash chain: DecisionHash is computed
// from this row's own Agent/StepName/Reasoning/Output plus the previous
// row's DecisionHash (or the Task's genesis hash, for the first row) — see
// service/agent/hashchain.go. Rows are never updated after being written;
// a wrong step is corrected by appending a new row, never editing an old
// one, or the chain would no longer verify.
//
// Output is a plain string (TEXT column), not a JSON column type,
// deliberately: it must be byte-identical to what was actually hashed at
// write time, and Postgres's JSONB type reformats stored JSON (whitespace,
// key order) in a way that would silently break independent verification.
type AgentSubTask struct {
	ID               int64     `gorm:"column:id;primaryKey"`
	TaskID           int64     `gorm:"column:task_id"`
	StepOrder        int       `gorm:"column:step_order"`
	Agent            string    `gorm:"column:agent"`
	StepName         string    `gorm:"column:step_name"`
	Status           string    `gorm:"column:status"`
	Reasoning        string    `gorm:"column:reasoning"`
	Output           *string   `gorm:"column:output"`
	PrevDecisionHash string    `gorm:"column:prev_decision_hash"`
	DecisionHash     string    `gorm:"column:decision_hash"`
	RecordedOnChain  bool      `gorm:"column:recorded_on_chain"`
	OnChainTxHash    *string   `gorm:"column:on_chain_tx_hash"`
	CreatedAt        time.Time `gorm:"column:created_at;autoCreateTime"`
}

func (AgentSubTask) TableName() string { return "agent_sub_tasks" }
