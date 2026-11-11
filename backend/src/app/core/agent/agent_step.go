package agentcore

import (
    "context"
    "errors"
    "fmt"

    "github.com/cloudwego/eino/adk"
    "github.com/cloudwego/eino/compose"
    "github.com/cloudwego/eino/schema"
)

type AgentPause struct {
    InnerInterruptID string
    Payload          any
    State            *TurnState
}

type PausedAt struct {
    InterruptID string
    Payload     any
}

type NodeFunc func(ctx context.Context, state *TurnState) (*TurnState, error)

// runner: the runner of the agent the node runs.
// node: node name, part of the agent's own checkpoint id.
// state: graph state; nil when the node is resumed after a pause.
// messages: input for a fresh run; ignored when resuming.
// The string returned is the final text the agent wrote this turn.
type StepFunc func(ctx context.Context, runner *adk.Runner, node string, state *TurnState, messages []adk.Message) (*TurnState, string, error)

type EventReader func(events *adk.AsyncIterator[*adk.AgentEvent]) (*PausedAt, error)

type AgentStep struct {
    Runner      *adk.Runner
    Node        string
    Run         *RunContext
    Checkpoints ExtendedCheckPointStore
    Read        EventReader
}

func init() {
    schema.RegisterName[*AgentPause]("agent_graph_pause")
    schema.RegisterName[*TurnState]("agent_turn_state")
}

func AgentCheckpointID(state *TurnState, node string) string {
    if state.Scope == "" {
        return state.ChatID.String() + ":" + node
    }

    return state.ChatID.String() + ":" + state.Scope + ":" + node
}

// ctx: node context.
// state: graph state; nil when the node is resumed after a pause.
// messages: input for a fresh run; ignored when resuming.
func (step AgentStep) Execute(ctx context.Context, state *TurnState, messages []adk.Message) (*TurnState, error) {
    if step.Runner == nil {
        return state, fmt.Errorf("agent: agent for node %s is not configured", step.Node)
    }

    if step.Run == nil || step.Read == nil {
        return state, errors.New("agent: step needs a run context and an event reader")
    }

    wasInterrupted, hasSaved, pause := compose.GetInterruptState[*AgentPause](ctx)
    resuming := wasInterrupted && hasSaved
    if resuming {
        state = pause.State
    }

    if state == nil {
        return nil, fmt.Errorf("agent: node %s started without state", step.Node)
    }

    seedRunContext(step.Run, state)
    checkpointID := AgentCheckpointID(state, step.Node)

    var events *adk.AsyncIterator[*adk.AgentEvent]
    if resuming {
        isTarget, hasData, answer := compose.GetResumeContext[string](ctx)
        if !isTarget || !hasData {
            return state, compose.StatefulInterrupt(ctx, pause.Payload, pause)
        }

        resumed, err := step.Runner.ResumeWithParams(ctx, checkpointID, &adk.ResumeParams{Targets: map[string]any{pause.InnerInterruptID: answer}})
        if err != nil {
            return state, fmt.Errorf("agent: resume %s: %w", step.Node, err)
        }

        events = resumed
    } else {
        events = step.Runner.Run(ctx, messages, adk.WithCheckPointID(checkpointID))
    }

    pausedAt, err := step.Read(events)
    if err != nil {
        return state, err
    }

    syncStateFromRun(state, step.Run)
    if pausedAt != nil {
        return state, compose.StatefulInterrupt(ctx, pausedAt.Payload, &AgentPause{InnerInterruptID: pausedAt.InterruptID, Payload: pausedAt.Payload, State: state})
    }

    if resuming && step.Checkpoints != nil {
        _ = step.Checkpoints.Delete(ctx, checkpointID)
    }

    return state, nil
}

func seedRunContext(run *RunContext, state *TurnState) {
    run.Route = state.Route
    run.NovaRequest = state.NovaRequest
    run.NovaDone = state.NovaDone
    run.Armed = state.Armed
}

func syncStateFromRun(state *TurnState, run *RunContext) {
    state.TaskID = run.TaskID
    state.Route = run.Route
    state.NovaRequest = run.NovaRequest
    state.Armed = run.Armed
}
