package agent

import (
	"context"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

// TextDeltaMiddleware taps a ChatModelAgent's own model calls to forward
// each real content chunk out through the current run's RunContext.OnTextDelta
// the instant the model generates it — IF the underlying call is ever a
// real Stream(), which it currently is not (see agent-task-manager-code-
// implementation.md §7.Q): eino v0.9.15's ChatModelAgent hardcodes its
// model-invocation node as compose.InvokableLambda, so it always calls
// Generate() — confirmed via source inspection and live instrumented
// logging, not assumed. This middleware is real, correct, and currently
// inert as a result; kept as-is rather than ripped out, since it costs
// nothing at rest and is exactly what would be needed the moment either
// (a) eino is upgraded to a version whose ChatModelAgent streams, or
// (b) Supervisor's final-answer step is rewritten to call the model
// directly instead of through adk.Agent.Run(). Neither has been done —
// paused on explicit instruction pending a decision on which path to take.
//
// Attach this to Supervisor's own ChatModelAgentConfig.Handlers only —
// never Analyzer's/Executor's — so if this ever does start firing, only
// Supervisor's own final synthesis streams to the user, not their
// internal reasoning generations.
type TextDeltaMiddleware struct {
	adk.BaseChatModelAgentMiddleware
}

func (m *TextDeltaMiddleware) WrapModel(ctx context.Context, base model.BaseModel[*schema.Message], _ *adk.ModelContext) (model.BaseModel[*schema.Message], error) {
	rc, ok := RunContextFrom(ctx)
	if !ok || rc.OnTextDelta == nil {
		return base, nil
	}
	return &textDeltaTapModel{inner: base, onDelta: rc.OnTextDelta}, nil
}

// textDeltaTapModel passes every call straight through to the real model —
// Generate untouched, Stream wrapped so each chunk is observed on its way
// past, never altered. schema.StreamReaderWithConvert is eino's own utility
// for exactly this "tee": the chunk that downstream (eino's own internal
// concatenation into a final message) receives is byte-identical to what
// the real model produced.
type textDeltaTapModel struct {
	inner   model.BaseModel[*schema.Message]
	onDelta func(string)
}

func (m *textDeltaTapModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	return m.inner.Generate(ctx, input, opts...)
}

func (m *textDeltaTapModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	sr, err := m.inner.Stream(ctx, input, opts...)
	if err != nil {
		return nil, err
	}
	return schema.StreamReaderWithConvert(sr, func(chunk *schema.Message) (*schema.Message, error) {
		if chunk.Content != "" {
			m.onDelta(chunk.Content)
		}
		return chunk, nil
	}), nil
}
