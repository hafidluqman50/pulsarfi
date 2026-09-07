package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

func InvokeAgentStructured(ctx context.Context, a adk.Agent, prompt string, out any, onFailure func()) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if lastErr = runAgentOnce(ctx, a, prompt, out); lastErr == nil {
			return
		}
	}
	onFailure()
}

func runAgentOnce(ctx context.Context, a adk.Agent, prompt string, out any) error {
	final, _, err := RunAgentWithTrace(ctx, a, prompt, nil, "")
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(final), out)
}

type ToolCallTrace struct {
	ToolName string
	Result   string
}

// reasoningEffort is DeepSeek's own reasoning-effort value ("low"/"medium"/
// "high"/"max") to override for this specific call, or "" to leave the
// model's default in place — see docs/plans/dynamic-model-tier-routing.md.
// Confirmed against eino's own source that adk.WithChatModelOptions
// actually reaches the underlying model call (chatmodel.go), not assumed.
func RunAgentWithTrace(ctx context.Context, a adk.Agent, prompt string, onTextDelta func(string), reasoningEffort string) (finalText string, toolCalls []ToolCallTrace, err error) {
	return RunAgentWithHistory(ctx, a, []*schema.Message{schema.UserMessage(prompt)}, onTextDelta, reasoningEffort)
}

func RunAgentWithHistory(ctx context.Context, a adk.Agent, messages []*schema.Message, onTextDelta func(string), reasoningEffort string) (finalText string, toolCalls []ToolCallTrace, err error) {
	agentMessages := make([]adk.Message, len(messages))
	for i, m := range messages {
		agentMessages[i] = m
	}
	var runOpts []adk.AgentRunOption
	if reasoningEffort != "" {
		runOpts = append(runOpts, adk.WithChatModelOptions([]model.Option{einoopenai.WithReasoningEffort(einoopenai.ReasoningEffortLevel(reasoningEffort))}))
	}
	iterator := a.Run(ctx, &adk.AgentInput{
		Messages: agentMessages,
	}, runOpts...)

	var lastAssistant, lastNonEmptyAssistant *schema.Message
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
			var onDelta func(string)
			if out.Role == schema.Assistant {
				onDelta = onTextDelta
			}
			msg, err = drainMessageStream(out.MessageStream, onDelta)
			if err != nil {
				return "", nil, fmt.Errorf("agent: drain message stream: %w", err)
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
			toolCalls = append(toolCalls, ToolCallTrace{ToolName: msg.ToolName, Result: msg.Content})
		}
	}

	final := lastNonEmptyAssistant
	if final == nil {
		final = lastAssistant
	}
	if final == nil {
		return "", nil, fmt.Errorf("agent: no final response message")
	}
	if strings.TrimSpace(final.Content) == "" {
		slog.WarnContext(ctx, "agent: final assistant message has empty content even after preferring non-empty", "tool_calls", len(toolCalls))
	}
	return final.Content, toolCalls, nil
}

func drainMessageStream(stream *schema.StreamReader[*schema.Message], onDelta func(string)) (*schema.Message, error) {
	var chunks []*schema.Message
	for {
		chunk, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		if onDelta != nil && chunk.Content != "" {
			onDelta(chunk.Content)
		}
		chunks = append(chunks, chunk)
	}
	if len(chunks) == 0 {
		return nil, nil
	}
	return schema.ConcatMessageStream(schema.StreamReaderFromArray(chunks))
}
