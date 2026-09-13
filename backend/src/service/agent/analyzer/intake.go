package analyzer

import (
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent"
)

func RequiredIntake(in agent.IntakeContext) []agent.IntakeField {
	if in.Shape != agent.ShapeScalp {
		return nil
	}
	return []agent.IntakeField{
		{
			Key:      "consult_nova",
			Question: "Should Nova analyse market sentiment before execution?",
			Why:      "If not, Comet executes directly based on on-chain price and balance.",
			Options:  []string{"yes, analyse first", "no, execute directly"},
		},
	}
}
