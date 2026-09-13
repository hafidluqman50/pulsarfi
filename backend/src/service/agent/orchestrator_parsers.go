package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/callbacks"
	"github.com/cloudwego/eino/components"
	"github.com/cloudwego/eino/schema"
)

// toolCallResult mirrors one tool call Nova (or Comet) made internally —
// captured by runRoleAgent's own event drain, never by a merged
// "NestedToolCalls" concept the old supervisor-tool-call design needed
// (Nova is a graph node here, not a tool something else calls, so there is
// only ever one layer to look at).
type toolCallResult struct {
	ToolName string
	Result   string
}

// newsEvidenceItem mirrors one item of Nova's own "evidence" array (its own
// JSON contract, analyzer/instructions.go) — re-parsed here purely to
// surface it as its own citation card, never to re-derive or alter what
// Nova actually concluded.
type newsEvidenceItem struct {
	Source      string `json:"source"`
	URL         string `json:"url,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
	Excerpt     string `json:"excerpt,omitempty"`
	ImageURL    string `json:"image_url,omitempty"`
}

func extractNewsEvidence(raw string) []newsEvidenceItem {
	var parsed struct {
		Evidence []newsEvidenceItem `json:"evidence"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil
	}
	return parsed.Evidence
}

// classifyReply restores the chart/news content_type classification that
// task_service.go's old buildWorkflowCard did before the v2.2 wiring
// deleted it (a real, user-visible regression — a chart/news request
// after that point rendered as a plain, uninformative text-only reply,
// found live: "gua nunggu chart gak ada ini"). Only ever looks at Nova's
// own tool calls — Comet's own tools (get_portfolio_holdings, submit_trade)
// never produce chart-shaped or news-shaped output.
func classifyReply(turn *orchestratorTurn) (contentType string, uiProps json.RawMessage) {
	// The confirmation card takes precedence over everything else: this turn
	// deliberately produced no Task and no analysis, so there is nothing
	// chart- or news-shaped to classify anyway.
	if len(turn.PendingQuestions) > 0 {
		card := turn.Decision.Card
		if payload, err := json.Marshal(map[string]any{
			"questions": turn.PendingQuestions,
			"card":      card,
		}); err == nil {
			return "workflow_card", payload
		}
	}

	var chartPayloads []json.RawMessage
	for _, tc := range turn.AnalyzerToolCalls {
		if tc.ToolName == "get_portfolio_snapshot" || tc.ToolName == "get_stock_chart" {
			chartPayloads = append(chartPayloads, json.RawMessage(tc.Result))
		}
	}
	if len(chartPayloads) == 1 {
		return "chart", chartPayloads[0]
	}
	if len(chartPayloads) > 1 {
		if payload, err := json.Marshal(chartPayloads); err == nil {
			return "chart", payload
		}
	}

	if evidence := extractNewsEvidence(turn.AnalyzerReply); len(evidence) > 0 {
		if payload, err := json.Marshal(evidence); err == nil {
			return "news", payload
		}
	}

	return "text", nil
}

func parseRouteDecision(raw string) (routeDecision, bool) {
	raw = strings.TrimSpace(raw)
	raw = strings.TrimPrefix(raw, "```json")
	raw = strings.TrimPrefix(raw, "```")
	raw = strings.TrimSuffix(raw, "```")
	raw = strings.TrimSpace(raw)

	var decision routeDecision
	if err := json.Unmarshal([]byte(raw), &decision); err != nil {
		return routeDecision{}, false
	}
	if decision.Path == "" {
		return routeDecision{}, false
	}
	return decision, true
}

// analyzerConfirmed defensively reads Nova's own condition_met field —
// Nova's reply is prose-or-JSON depending on how it chose to answer, never
// enforced strictly by its own tool schema, same defensive-parse pattern
// already established elsewhere in this codebase (task_service.go's
// unwrapReplyJSON).
func analyzerConfirmed(raw string) bool {
	var parsed struct {
		ConditionMet bool `json:"condition_met"`
	}
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return false
	}
	return parsed.ConditionMet
}

// drainAssistantThinking reads a streaming Assistant-role message chunk by
// chunk, forwarding each non-empty piece live via onThinking the instant it
// arrives, and returns the fully concatenated text once the stream ends.
func drainAssistantThinking(stream *schema.StreamReader[*schema.Message], onThinking func(delta string)) (string, error) {
	defer stream.Close()
	var full strings.Builder
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		if chunk.Content == "" {
			continue
		}
		full.WriteString(chunk.Content)
		if onThinking != nil {
			onThinking(chunk.Content)
		}
	}
	return full.String(), nil
}

// runRoleAgent runs Nova or Comet's own, unmodified adk.Agent to
// completion and returns its final reply text — a fresh, local replacement
// for llm_service.go's RunAgentWithTrace, kept independent of that file on
// purpose.
func runRoleAgent(ctx context.Context, a adk.Agent, prompt string, onToolCall func(toolName, phase string), onThinking func(delta string)) (string, []toolCallResult, error) {
	var runOpts []adk.AgentRunOption
	if onToolCall != nil {
		handler := callbacks.NewHandlerBuilder().
			OnStartFn(func(ctx context.Context, info *callbacks.RunInfo, input callbacks.CallbackInput) context.Context {
				if info.Component == components.ComponentOfTool {
					onToolCall(info.Name, "start")
				}
				return ctx
			}).
			OnEndFn(func(ctx context.Context, info *callbacks.RunInfo, output callbacks.CallbackOutput) context.Context {
				if info.Component == components.ComponentOfTool {
					onToolCall(info.Name, "end")
				}
				return ctx
			}).
			Build()
		runOpts = append(runOpts, adk.WithCallbacks(handler))
	}

	iterator := a.Run(ctx, &adk.AgentInput{Messages: []*schema.Message{schema.UserMessage(prompt)}, EnableStreaming: true}, runOpts...)

	var lastAssistant, lastNonEmptyAssistant *schema.Message
	var toolCalls []toolCallResult
	for {
		event, ok := iterator.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return "", nil, event.Err
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		out := event.Output.MessageOutput

		var msg *schema.Message
		if out.IsStreaming {
			if out.Role == schema.Assistant {
				content, err := drainAssistantThinking(out.MessageStream, onThinking)
				if err != nil {
					return "", nil, fmt.Errorf("orchestrator: drain assistant thinking stream: %w", err)
				}
				msg = &schema.Message{Role: schema.Assistant, Content: content}
			} else {
				concatenated, err := schema.ConcatMessageStream(out.MessageStream)
				if err != nil {
					return "", nil, fmt.Errorf("orchestrator: concat message stream: %w", err)
				}
				msg = concatenated
			}
		} else {
			msg = out.Message
		}
		if msg == nil {
			continue
		}

		switch out.Role {
		case schema.Assistant:
			lastAssistant = msg
			if strings.TrimSpace(msg.Content) != "" {
				lastNonEmptyAssistant = msg
			}
		case schema.Tool:
			toolCalls = append(toolCalls, toolCallResult{ToolName: msg.ToolName, Result: msg.Content})
		}
	}

	final := lastNonEmptyAssistant
	if final == nil {
		final = lastAssistant
	}
	if final == nil {
		return "", nil, fmt.Errorf("orchestrator: no final response from agent")
	}
	return final.Content, toolCalls, nil
}
