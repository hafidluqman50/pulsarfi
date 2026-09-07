package agent

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/cloudwego/eino/components/tool"
)

type gracefulTool struct {
	tool.InvokableTool
}

func WrapToolGraceful(t tool.InvokableTool) tool.InvokableTool {
	return &gracefulTool{InvokableTool: t}
}

func (g *gracefulTool) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	result, err := g.InvokableTool.InvokableRun(ctx, argumentsInJSON, opts...)
	if err == nil {
		return result, nil
	}

	// The whole point of this wrapper is to hide the failure from the rest
	// of the run (Supervisor gets a soft tool_error, not a crash) — but
	// silently was too silent: nothing was ever logged either, so a real
	// failure (e.g. Analyzer's own run erroring mid-turn) left zero trace
	// anywhere an operator could find, only a graceful-sounding note in the
	// conversation itself. Found live: a task's own recorded steps showed
	// Supervisor retried analyzer_agent after a silent first failure, and
	// the server's own stdout had nothing at all for it.
	toolName := "unknown_tool"
	if info, infoErr := g.Info(ctx); infoErr == nil && info != nil {
		toolName = info.Name
	}
	slog.ErrorContext(ctx, "agent: tool call failed, degrading gracefully", "tool", toolName, "error", err, "arguments", argumentsInJSON)

	payload, marshalErr := json.Marshal(map[string]string{
		"tool_error": err.Error(),
		"note":       "This specific action could not be completed. Explain this to the user briefly and professionally in your own voice, then continue with anything else you can still help with. This is not fatal.",
	})
	if marshalErr != nil {
		return `{"tool_error":"unknown error"}`, nil
	}
	return string(payload), nil
}
