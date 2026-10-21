package agentcore

// TurnKind is what a chat message turns out to be, once the chat's pending
// interrupt is taken into account.
type TurnKind int

const (
	FreshTurn             TurnKind = iota // no pause -> start a new turn
	AnswerQuestions                       // paused on ask_user -> this message IS the answer
	SideQuestionDuringArm                 // paused on await_arm -> new turn, arm pause stays alive
	CancelTurn                            // paused + explicit cancellation
)

// TurnIntent is the resolved message: its kind, plus the interrupt it answers
// when it answers one (so the pending marker is read exactly once).
type TurnIntent struct {
	Kind        TurnKind
	InterruptID string
}

// FreshTurnIntent is the intent for a message that starts a new turn.
func FreshTurnIntent() TurnIntent { return TurnIntent{Kind: FreshTurn} }
