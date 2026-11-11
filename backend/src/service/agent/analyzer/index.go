package analyzer

import (
	"context"
	"errors"
	"fmt"
	helper "github.com/horizonlabs/pulsarfi-backend/src/app/helper"
	"log/slog"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"
	agentcore "github.com/horizonlabs/pulsarfi-backend/src/app/core/agent"
	publicsvc "github.com/horizonlabs/pulsarfi-backend/src/service/public"
)

func New(ctx context.Context, chatModel model.ToolCallingChatModel, chartReader publicsvc.ChartDataReader, priceSvc *publicsvc.PriceService, extraTools ...tool.BaseTool) (adk.Agent, error) {
	portfolioSnapshotTool, err := newPortfolioSnapshotTool(chartReader)
	if err != nil {
		return nil, fmt.Errorf("analyzer: build get_portfolio_snapshot tool: %w", err)
	}
	stockChartTool, err := newStockChartTool(priceSvc)
	if err != nil {
		return nil, fmt.Errorf("analyzer: build get_stock_chart tool: %w", err)
	}
	verifyTickerTool, err := newVerifyTickerTool(priceSvc)
	if err != nil {
		return nil, fmt.Errorf("analyzer: build verify_ticker tool: %w", err)
	}
	readArticleTool, webSearchTool, err := NewNewsTools()
	if err != nil {
		return nil, fmt.Errorf("analyzer: build news tools: %w", err)
	}
	baseTools := []tool.BaseTool{portfolioSnapshotTool, stockChartTool, verifyTickerTool, readArticleTool, webSearchTool}
	tools := append(baseTools, extraTools...)

	analyzerAgent, err := adk.NewChatModelAgent(ctx, &adk.ChatModelAgentConfig{
		Name:        "analyzer_agent",
		Description: "Gathers news and/or technical evidence for a Task's trigger condition and concludes whether it is satisfied, citing the specific evidence that drove the conclusion. Also answers portfolio/chart questions.",
		Instruction: instructions + TrustedSourcesInstructions(TrustedNewsDomains),
		Model:       chatModel,
		ToolsConfig: adk.ToolsConfig{ToolsNodeConfig: compose.ToolsNodeConfig{Tools: helper.WrapToolsGraceful(tools)}},
		// Bounded at 8 iterations: strictly well below 20 to conserve on-chain
		// ETH gas, while giving enough headroom for multi-step news research
		// and follow-up reference queries without hitting premature
		// ErrExceedMaxIterations / HTTP 500 errors.
		MaxIterations: 8,
	})
	if err != nil {
		return nil, fmt.Errorf("analyzer: build agent: %w", err)
	}
	return analyzerAgent, nil
}

// runner: the runner of Nova.
// step: runs one agent turn for a graph node.
func NewNode(runner *adk.Runner, step agentcore.StepFunc) agentcore.NodeFunc {
    return func(ctx context.Context, state *agentcore.TurnState) (*agentcore.TurnState, error) {
        run, ok := agentcore.RunContextFrom(ctx)
        if !ok {
            return state, errors.New("analyzer: no run context bound to this node")
        }

        if state == nil {
            return nil, errors.New("analyzer: node started without state")
        }

        decision := run.Decision
        if decision.Side != "" && !decision.IsActionable {
            return skipNova(state), nil
        }

        if err := recordNovaRoute(ctx, run); err != nil {
            return state, err
        }

        request := helper.BuildAnalyzerRequest(novaRequestText(state, run), decision.Shape, decision.MentionedTicker, decision.Side, decision.IsActionable)
        run.Stream.StepStarted("analyzer", "gather_evidence", decision.Label)
        run.Stream.SetCurrent("analyzer", "gather_evidence")

        result, reply, err := step(ctx, runner, "nova", state, []adk.Message{schema.UserMessage(request)})
        if err != nil {
            recordNovaFailure(ctx, run, err)
            return result, fmt.Errorf("analyzer: nova run: %w", err)
        }

        return recordNovaFindings(ctx, run, result, reply)
    }
}

func novaRequestText(state *agentcore.TurnState, run *agentcore.RunContext) string {
    if state.NovaRequest != "" {
        return state.NovaRequest
    }

    return run.RawPrompt
}

func skipNova(state *agentcore.TurnState) *agentcore.TurnState {
    state.NovaDone = true
    note := "System note: Nova was not run because the trade parameters are not settled yet. Ask the user for what is missing with ask_user, then update_task."
    state.Messages = append(state.Messages, schema.UserMessage(note))
    return state
}

func recordNovaRoute(ctx context.Context, run *agentcore.RunContext) error {
    label := run.Decision.Label
    run.Stream.StepStarted("supervisor", "route_to_analyzer", label)
    if _, err := run.Recorder.Record(ctx, "supervisor", "route_to_analyzer", "done", "Routed to Nova for evaluation.", label, nil); err != nil {
        return fmt.Errorf("analyzer: record route_to_analyzer: %w", err)
    }

    return nil
}

func recordNovaFailure(ctx context.Context, run *agentcore.RunContext, cause error) {
    if errors.Is(cause, agentcore.ErrToolBudgetExhausted) {
        return
    }

    if _, err := run.Recorder.Record(ctx, "analyzer", "gather_evidence", "failed", cause.Error(), run.Decision.Label, nil); err != nil {
        slog.ErrorContext(ctx, "analyzer: record gather_evidence failed step", "chat_id", run.ChatID.String(), "error", err)
    }
}

func recordNovaFindings(ctx context.Context, run *agentcore.RunContext, state *agentcore.TurnState, reply string) (*agentcore.TurnState, error) {
    prose := reply
    var verdict *helper.NovaVerdict
    if run.Decision.IsActionable {
        verdict, prose = helper.ParseNovaVerdict(reply)
    }

    if _, err := run.Recorder.Record(ctx, "analyzer", "gather_evidence", "done", prose, run.Decision.Label, nil); err != nil {
        return state, fmt.Errorf("analyzer: record gather_evidence: %w", err)
    }

    findings := prose
    if verdict != nil {
        findings = prose + "\n\n" + helper.FormatNovaVerdict(verdict)
    }

    state.NovaDone = true
    state.NovaFindings = findings
    state.Messages = append(state.Messages, schema.UserMessage(novaNote(state, findings)))
    return state, nil
}

func novaNote(state *agentcore.TurnState, findings string) string {
    if state.Route == agentcore.RouteAnalyzerThenExecutor {
        return fmt.Sprintf("System note: Nova finished analysing Task %d. Her findings and verdict:\n\n%s\n\nGo on: call await_arm with the sentence that points the user to the Arm Card.", state.TaskID, findings)
    }

    return fmt.Sprintf("System note: Nova finished analysing Task %d. Her findings:\n\n%s\n\nRelay them to the user in your own words.", state.TaskID, findings)
}
