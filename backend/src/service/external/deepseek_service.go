package external

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	einomodel "github.com/cloudwego/eino/components/model"
)

// requestTimeout bounds a single DeepSeek call. The underlying eino-ext
// client defaults to no timeout at all — observed live as a request that
// hung 5+ minutes with zero server errors and near-idle CPU (blocked on
// I/O, not looping) before being killed by hand. 90s comfortably covers
// the slowest real multi-tool-call turn seen so far (~20s, create_task +
// analyzer_agent + a chart tool) with headroom, while still turning a
// genuine hang into a real error the frontend's toastAgentError can show.
const requestTimeout = 90 * time.Second

// ModelFlash is the only model this app calls. "deepseek-v4-pro" is
// discontinued 2026-09-14 (per DeepSeek's own notice); confirmed live
// (matching system_fingerprint against the reported pre-release beta id
// deepseek-v4.1-flash-expires-on-0910) that "deepseek-flash" is the real
// V4.1 Flash, not the old V4-Flash under a shortened name, and DeepSeek's
// own claim that it beats V4-Pro on Pro's own benchmarks holds up under a
// direct side-by-side test on a real trading-judgment prompt: Pro needed
// ~3x the token budget just to avoid truncating mid-answer, took ~6x
// longer, and cost ~7x more for an equivalent conclusion. No second tier
// exists to route between right now, so there is nothing left for a
// ModelPro constant to name.
const ModelFlash = "deepseek-flash" // used for every role — no tiering, one model

// NewDeepSeekChatModelFromEnv builds an Eino-native ToolCallingChatModel
// backed by DeepSeek's OpenAI-compatible API — the model each role's
// adk.NewChatModelAgent (service/agent/supervisor, .../analyzer, .../executor)
// is built on top of, the same way CATAT passes a BaseChatModel into
// createXAgent. No ResponseFormat is set: every role's own
// instructions/prompt already say to reply in plain text, and forcing
// JSON-object mode here fought that — observed live as replies wrapped in
// {"reply": "..."} / {"path": ..., "message": "..."}, and once as a
// whitespace-only "reply" the model padded out to satisfy the JSON
// constraint with nothing valid left to say. InvokeAgentStructured (the
// only caller that would actually need structured JSON back) has no
// callers anywhere in this codebase.
func NewDeepSeekChatModel(ctx context.Context, model string) (einomodel.ToolCallingChatModel, error) {
	key := strings.TrimSpace(os.Getenv("DEEPSEEK_API_KEY"))
	if key == "" {
		return nil, fmt.Errorf("deepseek: missing DEEPSEEK_API_KEY")
	}
	return einoopenai.NewChatModel(ctx, &einoopenai.ChatModelConfig{
		APIKey:  key,
		Model:   model,
		BaseURL: "https://api.deepseek.com/v1",
		Timeout: requestTimeout,
	})
}
