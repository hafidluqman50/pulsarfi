package helper

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
)

// NovaVerdict is Nova's structured, machine-parsed conclusion on an
// actionable request. It is advisory only: Comet is given it as context for
// sizing and timing, but the user's own instruction and approved budget are
// the final authority.
type NovaVerdict struct {
	Tradeable  bool   `json:"tradeable"`
	EntryPrice string `json:"entry_price"`
	ExitPrice  string `json:"exit_price,omitempty"`
	Confidence string `json:"confidence"`
	Reasoning  string `json:"reasoning"`
}

// FormatNovaVerdict renders the verdict as plain lines Quasar can pass along
// to Comet in its request, since Comet only ever sees what Quasar writes.
func FormatNovaVerdict(verdict *NovaVerdict) string {
	if verdict == nil {
		return ""
	}
	return fmt.Sprintf(
		"Nova's structured verdict:\n- Tradeable: %v\n- Entry price: %s\n- Confidence: %s\n- Reasoning: %s",
		verdict.Tradeable, verdict.EntryPrice, verdict.Confidence, verdict.Reasoning,
	)
}

// BuildAnalyzerRequest is the request Nova receives: the user's instruction
// plus, for an actionable request, the ticker verification and the verdict
// block Nova must end its reply with.
func BuildAnalyzerRequest(userInstruction, shape, ticker, side string, isActionable bool) string {
	if !isActionable {
		return fmt.Sprintf("User instruction: %s\n\nAnswer using your available tools. This is an informational request only — no trade is being evaluated, do not verify tradeability or output a trade verdict.", userInstruction)
	}
	return fmt.Sprintf(
		"Trading shape: %s\nTicker: %s\nSide: %s\nUser instruction: %s\n\n"+
			"Verify whether %s is tokenized and tradeable on PulsarFi using verify_ticker. "+
			"Gather relevant market evidence, live pool price, and technical indicators for %s matching trading shape %s.\n"+
			"Conclude your reply with a fenced ```json block stating whether tradeable is true or false, entry_price, confidence, and reasoning.",
		shape, ticker, side, userInstruction, ticker, ticker, shape,
	)
}

// ParseNovaVerdict extracts the last fenced ```json block from Nova's reply
// and unmarshals it into a NovaVerdict. Everything before the block is
// Nova's normal prose (shown to the user unchanged). A missing or
// unparseable block fails closed — tradeable:false — never a green light.
func ParseNovaVerdict(reply string) (*NovaVerdict, string) {
	lastIdx := strings.LastIndex(reply, "```json")
	if lastIdx == -1 {
		lastIdx = strings.LastIndex(reply, "```")
	}
	if lastIdx == -1 {
		return &NovaVerdict{
			Tradeable:  false,
			Reasoning:  "Nova reply did not contain a structured JSON verdict block.",
			Confidence: "low",
		}, reply
	}

	prose := strings.TrimSpace(reply[:lastIdx])
	block := stripFence(reply[lastIdx:])

	var verdict NovaVerdict
	if err := json.Unmarshal([]byte(block), &verdict); err != nil {
		slog.Warn("helper: unmarshal nova verdict failed, failing closed", "error", err, "raw", block)
		return &NovaVerdict{
			Tradeable:  false,
			Reasoning:  "Unparseable Nova verdict block.",
			Confidence: "low",
		}, prose
	}

	return &verdict, prose
}

func stripFence(raw string) string {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	return strings.TrimSpace(raw)
}
