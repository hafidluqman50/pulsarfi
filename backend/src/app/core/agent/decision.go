package agentcore

import "github.com/horizonlabs/pulsarfi-backend/src/contracts"

// RouteDecision is the argument of Quasar's open_task and update_task tools:
// what Quasar understood about the request. It is also what the Task row's
// trigger description is built from.
type RouteDecision struct {
	IsActionable    bool                    `json:"is_actionable" jsonschema_description:"true only if this request could ever result in an on-chain trade; false for anything purely informational (chart, portfolio, news) and while trade parameters are still missing"`
	Summary         string                  `json:"summary" jsonschema_description:"short one-line summary of the request in the exact language or dialect of the user's message, suitable for on-chain storage"`
	Label           string                  `json:"label" jsonschema_description:"short human-readable title for this step, in the exact language or dialect of the user's message"`
	Shape           string                  `json:"shape,omitempty" jsonschema_description:"scalp, swing, investment, or empty when the user has not chosen one"`
	Side            string                  `json:"side,omitempty" jsonschema_description:"buy or sell (canonical English value), or empty when unknown"`
	MentionedTicker string                  `json:"mentioned_ticker,omitempty" jsonschema_description:"canonical PulsarFi ticker for the stock the user named: ALL-CAPS with the P suffix (BRPT resolves to BRPTP), or empty when none was named"`
	BudgetIDRX      string                  `json:"budget_idrx,omitempty" jsonschema_description:"confirmed IDRX budget as plain digits, or empty when not applicable or unknown"`
	SellAmount      string                  `json:"sell_amount,omitempty" jsonschema_description:"for sell: confirmed quantity of stock tokens as plain digits, or a percentage such as 50%, or empty when unknown"`
	Card            *contracts.CardContract `json:"card,omitempty" jsonschema_description:"mandatory for any trade Task, authored entirely in the user's language; see the card contract in your instructions"`
}
