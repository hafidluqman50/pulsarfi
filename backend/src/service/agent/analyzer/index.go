// Package analyzer is the Analyzer role: gathers whatever evidence a Task's
// trigger condition actually requires — news via search + read_article,
// technical/price data, or both for a hybrid condition — and produces a
// conclusion, not raw evidence, about whether that condition is satisfied.
// Replaces the old watcher/ (gathering only) and absorbs the old
// orchestrator/'s evidence-judgment responsibility into the same role,
// since the fix a real trader reads news and technical data together and
// judges it in one pass, not two separate identities handing off a
// snippet. Sizing/deciding an action is Executor's job, not Analyzer's —
// see docs/plans/agent-role-architecture.md §3.
//
// index.go defines the agent ONLY — name, description, system prompt,
// tools. Invocation (prompt assembly, calling the agent, parsing its
// answer) lives in service/agent/supervisor, the only place allowed to
// invoke this role (directly, or as an adk.AgentTool).
package analyzer

import (
	"context"
	"fmt"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
)

// New builds the Analyzer agent. extraTools should include whatever the
// current deployment supports for evidence-gathering (web search,
// read_article, indicator-aware price lookups) — Analyzer is never
// limited to one data source by identity, only by which tools it is
// actually given for a particular Task. get_portfolio_snapshot (built from
// chartReader) always ships — portfolio/chart questions are as core to
// this role as trigger-condition evidence-gathering.
func New(ctx context.Context, chatModel model.ToolCallingChatModel, chartReader ChartDataReader, extraTools ...tool.BaseTool) (adk.Agent, error) {
	chartTool, err := newPortfolioChartTool(chartReader)
	if err != nil {
		return nil, fmt.Errorf("analyzer: build get_portfolio_snapshot tool: %w", err)
	}
	tools := append([]tool.BaseTool{chartTool}, extraTools...)

	analyzerAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "analyzer_agent",
		Description: "Gathers news and/or technical evidence for a Task's trigger condition and concludes whether it is satisfied, citing the specific evidence that drove the conclusion. Also answers portfolio/chart questions.",
		Instruction: instructions,
		Model:       chatModel,
		ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: tools}},
	})
	if err != nil {
		return nil, fmt.Errorf("analyzer: build agent: %w", err)
	}
	return analyzerAgent, nil
}
