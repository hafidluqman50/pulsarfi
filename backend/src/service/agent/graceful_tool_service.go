package agent

import (
	"context"
	"encoding/json"

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
	payload, marshalErr := json.Marshal(map[string]string{
		"tool_error": err.Error(),
		"note":       "This specific action could not be completed. Explain this to the user briefly and professionally in your own voice, then continue with anything else you can still help with. This is not fatal.",
	})
	if marshalErr != nil {
		return `{"tool_error":"unknown error"}`, nil
	}
	return string(payload), nil
}
