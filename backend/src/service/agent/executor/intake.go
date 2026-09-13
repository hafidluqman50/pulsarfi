package executor

import (
	"github.com/horizonlabs/pulsarfi-backend/src/contracts"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent"
)

func RequiredIntake(in agent.IntakeContext) []agent.IntakeField {
	tickerField := agent.IntakeField{
		Key:      "ticker",
		Question: "Which listed stock token do you want to trade?",
		Why:      "Must be an approved tokenized asset on PulsarFi (e.g. BMRIP).",
	}
	sideField := agent.IntakeField{
		Key:      "side",
		Question: "Buy or sell direction?",
		Why:      "Buy and sell are opposite directions.",
		Options:  []string{"buy", "sell"},
	}

	fields := []agent.IntakeField{tickerField, sideField}

	switch in.Side {
	case contracts.TradeSideBuy:
		fields = append(fields, agent.IntakeField{
			Key:      "idrx_cap",
			Question: "What is your maximum IDRX budget cap?",
			Why:      "Strict ceiling for this Task. Comet sizes trades within this limit.",
		})
	case contracts.TradeSideSell:
		fields = append(fields, agent.IntakeField{
			Key:      "portfolio_share",
			Question: "What share of your token position do you want to sell?",
			Why:      "Calculated from your verified on-chain wallet balance.",
		})
	}

	if in.Shape == agent.ShapeSwing || in.Shape == agent.ShapeInvestment {
		fields = append(fields,
			agent.IntakeField{
				Key:      "horizon",
				Question: "How long should this position and permission remain active?",
				Why:      "Becomes the on-chain permission expiry duration.",
			},
			agent.IntakeField{
				Key:      "strategy",
				Question: "Execution entry strategy: all at once or staged (DCA)?",
				Why:      "Staged allows Comet to tranche entries over time within budget.",
				Options:  []string{"all at once", "staged (DCA)"},
			},
			agent.IntakeField{
				Key:      "exit_policy",
				Question: "If price consolidates until expiry, close position or leave open?",
				Why:      "Defines behavior upon permission expiration.",
				Options:  []string{"close all", "leave open"},
			},
		)
	}
	return fields
}
