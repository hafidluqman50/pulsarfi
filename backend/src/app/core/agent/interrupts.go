package agentcore

import "github.com/horizonlabs/pulsarfi-backend/src/contracts"

// QuestionsInterrupt is the payload of the ask_user pause. Message is the
// sentence that introduces the card; it travels with the pause so the user
// never sees a card under an empty bubble.
type QuestionsInterrupt struct {
	Questions []contracts.IntakeField
	Message   string
}

// ArmInterrupt is the payload of the await_arm pause. Message introduces the
// Arm Card, same reason as above.
type ArmInterrupt struct {
	TaskID  int64
	Message string
}
