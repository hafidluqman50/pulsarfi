package agentcore

import (
    "github.com/cloudwego/eino/schema"
    "github.com/google/uuid"
)

const (
    RouteNone                 = "none"
    RouteAnalyzerOnly         = "analyzer_only"
    RouteAnalyzerThenExecutor = "analyzer_then_executor"
    RouteExecutorOnly         = "executor_only"
)

type TurnState struct {
    ChatID   uuid.UUID
    Wallet   string
    Scope    string
    Messages []*schema.Message

    TaskID       int64
    Route        string
    NovaRequest  string
    NovaFindings string
    NovaDone     bool
    Armed        bool
    CometDone    bool
}

func IsKnownRoute(route string) bool {
    switch route {
    case RouteNone, RouteAnalyzerOnly, RouteAnalyzerThenExecutor, RouteExecutorOnly:
        return true
    }

    return false
}

func RouteNeedsNova(route string) bool {
    return route == RouteAnalyzerOnly || route == RouteAnalyzerThenExecutor
}

func (state *TurnState) NeedsNova() bool {
    return state.TaskID != 0 && !state.NovaDone && RouteNeedsNova(state.Route)
}

func (state *TurnState) NeedsComet() bool {
    return state.Armed && !state.CometDone
}
