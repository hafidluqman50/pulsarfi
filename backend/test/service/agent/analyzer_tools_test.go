package agent_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/horizonlabs/pulsarfi-backend/src/config"
	"github.com/horizonlabs/pulsarfi-backend/src/service/agent/analyzer"
	"github.com/horizonlabs/pulsarfi-backend/src/service/external"
	publicsvc "github.com/horizonlabs/pulsarfi-backend/src/service/public"
	"github.com/joho/godotenv"
)

func TestReadArticleTool_ValidationAndDomainAllowlist(t *testing.T) {
	_ = godotenv.Load("../../../.env")

	apiKey := config.GetEnv("TAVILY_API_KEY")
	tool, err := analyzer.NewReadArticleTool(apiKey, analyzer.TrustedNewsDomains)
	if err != nil {
		t.Fatalf("failed to create read_article tool: %v", err)
	}

	ctx := context.Background()

	// 1. Test invalid URL
	_, err = tool.InvokableRun(ctx, `{"url":"not-a-url"}`)
	if err == nil {
		t.Errorf("expected error for invalid URL, got nil")
	}

	// 2. Test untrusted domain
	_, err = tool.InvokableRun(ctx, `{"url":"https://evil.com/article"}`)
	if err == nil {
		t.Errorf("expected error for untrusted domain, got nil")
	} else if !strings.Contains(err.Error(), "trusted domain allowlist") {
		t.Errorf("expected allowlist error, got %v", err)
	}

	// 3. Test trusted domain with live Tavily Extract if API key is present
	if apiKey != "" {
		testURL := "https://money.kompas.com/read/2026/09/21/095947226/ihsg-hari-ini-dibuka-naik-lalu-berbalik-melemah-ke-6417"
		reqPayload, _ := json.Marshal(map[string]string{"url": testURL})
		outStr, err := tool.InvokableRun(ctx, string(reqPayload))
		if err != nil {
			t.Fatalf("read_article failed for trusted URL: %v", err)
		}

		var parsed struct {
			Title    string `json:"title"`
			Content  string `json:"content"`
			SiteName string `json:"site_name"`
		}
		if err := json.Unmarshal([]byte(outStr), &parsed); err != nil {
			t.Fatalf("failed to parse read_article output: %v", err)
		}

		if parsed.Title == "" {
			t.Errorf("expected non-empty title, got empty")
		}
		if parsed.Content == "" {
			t.Errorf("expected non-empty content, got empty")
		}
		if parsed.SiteName != "Kompas.com" {
			t.Errorf("expected SiteName Kompas.com, got %q", parsed.SiteName)
		}
	}
}

func TestNewNewsTools_Construction(t *testing.T) {
	_ = godotenv.Load("../../../.env")

	readArticle, webSearch, err := analyzer.NewNewsTools()
	if err != nil {
		t.Fatalf("NewNewsTools returned error: %v", err)
	}
	if readArticle == nil {
		t.Errorf("readArticleTool is nil")
	}
	if webSearch == nil {
		t.Errorf("webSearchTool is nil")
	}
}

func TestReadArticleTool_FallbackNoKey(t *testing.T) {
	// Tool created with empty apiKey forces fallback path
	tool, err := analyzer.NewReadArticleTool("", analyzer.TrustedNewsDomains)
	if err != nil {
		t.Fatalf("failed to create read_article tool: %v", err)
	}

	ctx := context.Background()
	// Calling a trusted URL without Tavily key falls back to readability
	// Even if readability fails or bot-blocks, it MUST NOT panic with "the Node field is nil"
	testURL := "https://money.kompas.com/read/2026/09/21/095947226/ihsg-hari-ini-dibuka-naik-lalu-berbalik-melemah-ke-6417"
	reqPayload, _ := json.Marshal(map[string]string{"url": testURL})
	_, err = tool.InvokableRun(ctx, string(reqPayload))
	if err != nil {
		// Error is acceptable (e.g. anti-bot challenge on direct Go HTTP client),
		// but it must NOT be "the Node field is nil"
		if strings.Contains(err.Error(), "the Node field is nil") {
			t.Errorf("critical bug reproduced: encountered 'the Node field is nil' error: %v", err)
		}
	}
}

func TestStockChartTool_IndexAndTokenizedTickers(t *testing.T) {
	priceSvc := &publicsvc.PriceService{Price: external.NewPriceService()}
	tool, err := analyzer.NewStockChartTool(priceSvc)
	if err != nil {
		t.Fatalf("failed to create stock_chart tool: %v", err)
	}

	ctx := context.Background()

	// 1. Test IHSG: must return valid ChartPayload without error
	outStr, err := tool.InvokableRun(ctx, `{"ticker":"IHSG","range":"1M","chart_q":"chart IHSG"}`)
	if err != nil {
		t.Fatalf("get_stock_chart failed for IHSG: %v", err)
	}
	var res1 publicsvc.ChartPayload
	if err := json.Unmarshal([]byte(outStr), &res1); err != nil {
		t.Fatalf("unmarshal chart payload: %v", err)
	}
	points1, ok := res1.Data.([]any)
	if !ok || len(points1) == 0 {
		t.Fatalf("expected non-empty chart data points for IHSG, got %v", res1.Data)
	}

	// 2. Test clean SINI: must fetch SINI.JK and return valid ChartPayload without error
	outStr, err = tool.InvokableRun(ctx, `{"ticker":"SINI","range":"1M","chart_q":"chart SINI"}`)
	if err != nil {
		t.Fatalf("get_stock_chart failed for SINI: %v", err)
	}
	var res2 publicsvc.ChartPayload
	if err := json.Unmarshal([]byte(outStr), &res2); err != nil {
		t.Fatalf("unmarshal chart payload: %v", err)
	}
	points2, ok := res2.Data.([]any)
	if !ok || len(points2) == 0 {
		t.Fatalf("expected non-empty chart data points for SINI (SINI.JK), got %v", res2.Data)
	}

	// 3. Test non-existent ticker: must degrade gracefully with LensNote and not error out
	outStr, err = tool.InvokableRun(ctx, `{"ticker":"NONEXISTENTXYZ","range":"1M","chart_q":"chart unknown"}`)
	if err != nil {
		t.Fatalf("get_stock_chart must not fail on not-found ticker: %v", err)
	}
	var res3 publicsvc.ChartPayload
	if err := json.Unmarshal([]byte(outStr), &res3); err != nil {
		t.Fatalf("unmarshal chart payload: %v", err)
	}
	if !strings.Contains(res3.LensNote, "No chart data is available") {
		t.Fatalf("expected LensNote to explain not found, got %q", res3.LensNote)
	}
}



