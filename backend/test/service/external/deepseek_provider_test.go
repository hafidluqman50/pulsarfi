package external_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/cloudwego/eino/schema"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
	"github.com/joho/godotenv"
)

// The provider is chosen by environment (docs/plans/agent-as-tool-orchestration.md
// v1.11): DEEPSEEK_API_KEY, DEEPSEEK_BASE_URL, DEEPSEEK_MODEL.

type capturedRequest struct {
	path          string
	authorization string
	model         string
}

func fakeProvider(t *testing.T) (*httptest.Server, *capturedRequest) {
	t.Helper()
	captured := &capturedRequest{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		body, _ := io.ReadAll(request.Body)
		var parsed struct {
			Model string `json:"model"`
		}
		_ = json.Unmarshal(body, &parsed)
		captured.path = request.URL.Path
		captured.authorization = request.Header.Get("Authorization")
		captured.model = parsed.Model

		writer.Header().Set("Content-Type", "application/json")
		_, _ = writer.Write([]byte(`{"id":"x","object":"chat.completion","model":"` + parsed.Model + `","choices":[{"index":0,"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`))
	}))
	t.Cleanup(server.Close)
	return server, captured
}

func generate(t *testing.T, passedModel string) {
	t.Helper()
	chatModel, err := external.NewDeepSeekChatModel(context.Background(), passedModel)
	if err != nil {
		t.Fatalf("build model: %v", err)
	}
	reply, err := chatModel.Generate(context.Background(), []*schema.Message{schema.UserMessage("hi")})
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if reply.Content != "hello" {
		t.Fatalf("unexpected reply content %q", reply.Content)
	}
}

func TestProvider_BaseURLKeyAndModelOverrideReachTheRequest(t *testing.T) {
	server, captured := fakeProvider(t)
	t.Setenv("DEEPSEEK_API_KEY", "provider-key")
	t.Setenv("DEEPSEEK_BASE_URL", server.URL+"/provider/v1")
	t.Setenv("DEEPSEEK_MODEL", "deepseek/deepseek-v4-flash")

	generate(t, external.ModelFlash)

	if captured.path != "/provider/v1/chat/completions" {
		t.Errorf("request went to %q", captured.path)
	}
	if captured.authorization != "Bearer provider-key" {
		t.Errorf("authorization header was %q", captured.authorization)
	}
	if captured.model != "deepseek/deepseek-v4-flash" {
		t.Errorf("the model override did not replace the caller's model, got %q", captured.model)
	}
}

// Without DEEPSEEK_MODEL the caller's model name is used unchanged, so a
// production that sets only a key and a base URL behaves as before.
func TestProvider_WithoutModelOverrideTheCallersModelIsUsed(t *testing.T) {
	server, captured := fakeProvider(t)
	t.Setenv("DEEPSEEK_API_KEY", "provider-key")
	t.Setenv("DEEPSEEK_BASE_URL", server.URL)
	t.Setenv("DEEPSEEK_MODEL", "")

	generate(t, external.ModelFlash)

	if captured.model != external.ModelFlash {
		t.Errorf("expected %q, got %q", external.ModelFlash, captured.model)
	}
}

func TestProvider_MissingKeyIsAnExplicitError(t *testing.T) {
	t.Setenv("DEEPSEEK_API_KEY", "")
	if _, err := external.NewDeepSeekChatModel(context.Background(), external.ModelFlash); err == nil {
		t.Fatal("expected an error without a key")
	}
}

// Live: what the agents need from ANY provider. Opt-in, reads backend/.env.test
// first, then the process environment. Needs tool calling and streaming;
// reasoning content is reported, not required (the UI falls back to the text).
func TestLiveProvider_ToolCallingStreamingAndReasoning(t *testing.T) {
	if os.Getenv("LIVE_PROVIDER_CHECK") != "1" {
		t.Skip("LIVE_PROVIDER_CHECK=1 not set: this test calls the configured provider")
	}
	if err := godotenv.Load("../../../.env.test"); err != nil {
		t.Logf("no .env.test loaded (%v), relying on the process environment", err)
	}
	ctx := context.Background()
	chatModel, err := external.NewDeepSeekChatModel(ctx, external.ModelFlash)
	if err != nil {
		t.Fatalf("build model: %v", err)
	}
	t.Logf("provider base URL set: %v, model override: %q", os.Getenv("DEEPSEEK_BASE_URL") != "", os.Getenv("DEEPSEEK_MODEL"))

	t.Run("plain reply", func(t *testing.T) {
		reply, err := chatModel.Generate(ctx, []*schema.Message{schema.UserMessage("Reply with the single word: pong")})
		if err != nil {
			t.Fatalf("the provider rejected a plain request: %v", err)
		}
		if strings.TrimSpace(reply.Content) == "" {
			t.Fatalf("the provider returned an empty reply")
		}
		t.Logf("reply: %q", strings.TrimSpace(reply.Content))
	})

	t.Run("tool calling", func(t *testing.T) {
		withTool, err := chatModel.WithTools([]*schema.ToolInfo{{
			Name: "get_stock_price",
			Desc: "Returns the current price of a stock ticker.",
			ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
				"ticker": {Type: schema.String, Desc: "The ticker, e.g. BBCA", Required: true},
			}),
		}})
		if err != nil {
			t.Fatalf("bind tools: %v", err)
		}
		reply, err := withTool.Generate(ctx, []*schema.Message{schema.UserMessage("What is the current price of BBCA? Use the tool.")})
		if err != nil {
			t.Fatalf("generate with tools: %v", err)
		}
		if len(reply.ToolCalls) == 0 || reply.ToolCalls[0].Function.Name != "get_stock_price" {
			t.Fatalf("the provider did not return the expected tool call, got content %q and %d tool calls", reply.Content, len(reply.ToolCalls))
		}
		var arguments struct {
			Ticker string `json:"ticker"`
		}
		if err := json.Unmarshal([]byte(reply.ToolCalls[0].Function.Arguments), &arguments); err != nil || arguments.Ticker == "" {
			t.Fatalf("tool arguments are not usable JSON: %q (%v)", reply.ToolCalls[0].Function.Arguments, err)
		}
		t.Logf("tool call: %s %s", reply.ToolCalls[0].Function.Name, reply.ToolCalls[0].Function.Arguments)
	})

	t.Run("streaming and reasoning", func(t *testing.T) {
		stream, err := chatModel.Stream(ctx, []*schema.Message{schema.UserMessage("Count from 1 to 5 in words, comma separated.")})
		if err != nil {
			t.Fatalf("stream: %v", err)
		}
		defer stream.Close()
		chunks, reasoningChunks := 0, 0
		var text strings.Builder
		for {
			chunk, err := stream.Recv()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatalf("stream read: %v", err)
			}
			chunks++
			text.WriteString(chunk.Content)
			if chunk.ReasoningContent != "" {
				reasoningChunks++
			}
		}
		if chunks < 2 || strings.TrimSpace(text.String()) == "" {
			t.Fatalf("the provider did not stream incrementally: chunks=%d text=%q", chunks, text.String())
		}
		t.Logf("streamed in %d chunks; reasoning chunks: %d (0 is fine, the UI falls back to the text)", chunks, reasoningChunks)
	})
}
