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

// Model name constants — verified directly against the live API (both
// accepted, echoed back unchanged in resp.Model), not assumed from docs.
const (
	ModelFlash = "deepseek-v4-flash" // cheap, used for the frequent heartbeat judgment call
	ModelPro   = "deepseek-v4-pro"   // costlier, reserved for the rarer fund-manager analysis call
)

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
func NewDeepSeekChatModelFromEnv(ctx context.Context, model string) (einomodel.ToolCallingChatModel, error) {
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
