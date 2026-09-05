package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"
)

// InvokeAgentStructured runs a real Eino Agent (adk.NewChatModelAgent — see
// service/agent/analyzer, .../executor, and .../supervisor, each built with
// Name/Description/Instruction/Model — some equipped with real tools, e.g.
// Executor's get_portfolio_holdings) with a single user message and
// unmarshals its FINAL response message content as JSON into out.
// Retries once on a failed/malformed call, then gives up and calls
// onFailure to decide the safe default — mirrors the bounded-retry-then-
// degrade pattern used for CATAT's liaison node (router/model.ts's
// invokeStructured): a parse failure must never crash the graph, it must
// always resolve to a safe, explicit fallback state instead. Shared so
// every LLM-backed role uses the same retry policy, mirroring
// router/model.ts being shared plumbing imported by multiple
// router/<role>.ts files, not owned by any one role.
func InvokeAgentStructured(ctx context.Context, a adk.Agent, prompt string, out any, onFailure func()) {
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if lastErr = runAgentOnce(ctx, a, prompt, out); lastErr == nil {
			return
		}
	}
	onFailure()
}

// runAgentOnce iterates every event the run produces — not just the first
// — because a tool-equipped agent's first event is the tool-call request
// (empty content), not its answer. The LAST assistant-role message is the
// agent's actual final synthesis after any tool round-trips; that is what
// gets parsed.
func runAgentOnce(ctx context.Context, a adk.Agent, prompt string, out any) error {
	final, _, err := RunAgentWithTrace(ctx, a, prompt)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(final), out)
}

// ToolCallTrace is one adk.AgentTool invocation observed during a run —
// which sub-agent (analyzer_agent, executor_agent) was called, and the
// final text it returned. Used by service/agent/supervisor's driver to
// reconstruct which nodes Supervisor actually routed to this turn, since
// Eino's AgentTool only surfaces this as ordinary tool-role messages in the
// event stream, not as a separate structured log.
type ToolCallTrace struct {
	ToolName string
	Result   string
}

// RunAgentWithTrace runs a (Chat)Agent to completion and returns its final
// assistant message content plus every tool-role message observed along
// the way, in call order. Never returns a partial/empty final text on
// success — an agent run that produces no assistant message is treated as
// an error, matching runAgentOnce's existing contract.
func RunAgentWithTrace(ctx context.Context, a adk.Agent, prompt string) (finalText string, toolCalls []ToolCallTrace, err error) {
	iterator := a.Run(ctx, &adk.AgentInput{
		Messages: []adk.Message{schema.UserMessage(prompt)},
	})

	var final *schema.Message
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
		msg := event.Output.MessageOutput.Message
		if msg == nil {
			continue
		}
		switch event.Output.MessageOutput.Role {
		case schema.Assistant:
			final = msg
		case schema.Tool:
			toolCalls = append(toolCalls, ToolCallTrace{ToolName: msg.ToolName, Result: msg.Content})
		}
	}
	if final == nil {
		return "", nil, fmt.Errorf("agent: no final response message")
	}
	return final.Content, toolCalls, nil
}
