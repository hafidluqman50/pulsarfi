package agent

import (
	"strings"

	"github.com/horizonlabs/pulsarfi-backend/src/contracts"
)

type TradeShape string

const (
	ShapeUnknown    TradeShape = ""
	ShapeScalp      TradeShape = "scalp"
	ShapeSwing      TradeShape = "swing"
	ShapeInvestment TradeShape = "investment"
)

func ParseTradeShape(s string) TradeShape {
	switch TradeShape(s) {
	case ShapeScalp:
		return ShapeScalp
	case ShapeSwing:
		return ShapeSwing
	case ShapeInvestment:
		return ShapeInvestment
	default:
		return ShapeUnknown
	}
}

type IntakeContext struct {
	Shape TradeShape
	Side  contracts.TradeSide
}

func ParseSide(s string) contracts.TradeSide {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "beli", "buy":
		return contracts.TradeSideBuy
	case "jual", "sell":
		return contracts.TradeSideSell
	default:
		return ""
	}
}

type IntakeField struct {
	Key      string   `json:"key"`
	Question string   `json:"question"`
	Why      string   `json:"why,omitempty"`
	Options  []string `json:"options,omitempty"`
}

var ShapeField = IntakeField{
	Key:      "shape",
	Question: "Select trading horizon & shape: scalp, swing, or investment",
	Why:      "Determines holding duration and required execution parameters.",
	Options:  []string{"scalp", "swing", "investment"},
}

func GetShapeFieldSet() []IntakeField {
	return []IntakeField{ShapeField}
}

func CompileIntake(unanswered []string, fieldSets ...[]IntakeField) []IntakeField {
	want := make(map[string]bool, len(unanswered))
	for _, key := range unanswered {
		want[key] = true
	}

	seen := make(map[string]bool)
	compiled := make([]IntakeField, 0, len(unanswered))
	for _, set := range fieldSets {
		for _, field := range set {
			if seen[field.Key] || !want[field.Key] {
				continue
			}
			seen[field.Key] = true
			compiled = append(compiled, field)
		}
	}
	return compiled
}
