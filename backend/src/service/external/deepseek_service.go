package external

import (
	"context"
	"fmt"
	"os"
	"strings"

	einoopenai "github.com/cloudwego/eino-ext/components/model/openai"
	einomodel "github.com/cloudwego/eino/components/model"
)

// Model name constants — verified directly against the live API (both
// accepted, echoed back unchanged in resp.Model), not assumed from docs.
const (
	ModelFlash = "deepseek-v4-flash" // cheap, used for the frequent heartbeat judgment call
	ModelPro   = "deepseek-v4-pro"   // costlier, reserved for the rarer fund-manager analysis call
)

// NewDeepSeekChatModelFromEnv builds an Eino-native ToolCallingChatModel
// backed by DeepSeek's OpenAI-compatible API — the model each role's
// adk.NewChatModelAgent (service/agent/orchestrator, .../fundmanager/analyzer)
// is built on top of, the same way CATAT passes a BaseChatModel into
// createXAgent. JSON-object mode is forced here so every role gets
// syntactically valid JSON back regardless of prompt wording; the specific
// fields expected are still described in each role's own instructions/prompt.
func NewDeepSeekChatModelFromEnv(ctx context.Context, model string) (einomodel.ToolCallingChatModel, error) {
	key := strings.TrimSpace(os.Getenv("DEEPSEEK_API_KEY"))
	if key == "" {
		return nil, fmt.Errorf("deepseek: missing DEEPSEEK_API_KEY")
	}
	return einoopenai.NewChatModel(ctx, &einoopenai.ChatModelConfig{
		APIKey:  key,
		Model:   model,
		BaseURL: "https://api.deepseek.com/v1",
		ResponseFormat: &einoopenai.ChatCompletionResponseFormat{
			Type: einoopenai.ChatCompletionResponseFormatTypeJSONObject,
		},
	})
}
