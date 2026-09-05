package analyzer

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	readability "codeberg.org/readeck/go-readability/v2"
	ddgsearch "github.com/cloudwego/eino-ext/components/tool/duckduckgo/v2"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
)

// This file gives Analyzer its own news-gathering capabilities — search
// and read_article. Moved here from a shared top-level file: both are, in
// current practice, Analyzer-only (only analyzer.New ever receives them),
// so they belong inside Analyzer's own package, same as
// executor/tools_service.go and supervisor/tools_service.go already own
// their respective agent's tools — no splitting an agent's tools away from
// the agent itself.

// TrustedNewsDomains is the hard allowlist read_article enforces. Keep this
// in sync with the Trusted Sources list hardcoded into instructions.go —
// the instructions tell the model which domains to prefer searching, this
// is what actually gates what it can fetch.
var TrustedNewsDomains = []string{
	"liputan6.com",
	"kompas.com",
	"market.bisnis.com",
	"cnbcindonesia.com",
}

// NewSearchTool wraps DuckDuckGo's ready-made tool — no API key needed,
// unlike a production-grade provider (e.g. Tavily) would. eino-ext flags
// it "not recommended for production" (no stable API, scraping-based) —
// fine for now, upgrade later.
func NewSearchTool(ctx context.Context, maxResults int) (tool.InvokableTool, error) {
	return ddgsearch.NewTextSearchTool(ctx, &ddgsearch.Config{MaxResults: maxResults})
}

type readArticleRequest struct {
	URL string `json:"url" jsonschema_description:"The exact article URL from a search result to fetch and read in full — must be on the trusted domain allowlist, never a URL you invent or guess."`
}

type readArticleResponse struct {
	Title   string `json:"title" jsonschema_description:"The article's headline."`
	Content string `json:"content" jsonschema_description:"The article's full readable body text, stripped of ads/navigation/scripts."`
}

// NewReadArticleTool builds a tool that fetches a URL and extracts its
// readable article text (a Go port of Mozilla's Readability algorithm),
// refusing any URL whose host isn't in allowedDomains. The trust decision
// is enforced here in code — never left to the LLM's own judgment of
// "does this site look legit," which a convincingly-faked site could fool.
func NewReadArticleTool(allowedDomains []string) (tool.InvokableTool, error) {
	return utils.InferTool(
		"read_article",
		"Fetches a news article URL and returns its full readable text (title + body), stripped of ads/navigation/scripts. Only works for URLs on the trusted domain allowlist — refuses any other domain.",
		func(ctx context.Context, req readArticleRequest) (readArticleResponse, error) {
			return fetchArticle(req.URL, allowedDomains)
		},
	)
}

func fetchArticle(rawURL string, allowedDomains []string) (readArticleResponse, error) {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return readArticleResponse{}, fmt.Errorf("read_article: invalid URL: %w", err)
	}
	if !hostAllowed(parsed.Hostname(), allowedDomains) {
		return readArticleResponse{}, fmt.Errorf("read_article: %q is not on the trusted domain allowlist, refusing to fetch", parsed.Hostname())
	}

	// News sites commonly bot-block Go's default "Go-http-client" User-Agent
	// (e.g. Cloudflare) — a real browser UA is needed just to get past that,
	// not to disguise anything about what this tool does.
	article, err := readability.FromURL(rawURL, 15*time.Second, func(r *http.Request) {
		r.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	})
	if err != nil {
		return readArticleResponse{}, fmt.Errorf("read_article: fetch/parse failed: %w", err)
	}

	var body strings.Builder
	if err := article.RenderText(&body); err != nil {
		return readArticleResponse{}, fmt.Errorf("read_article: render text failed: %w", err)
	}

	return readArticleResponse{Title: article.Title(), Content: body.String()}, nil
}

func hostAllowed(host string, allowedDomains []string) bool {
	host = strings.ToLower(strings.TrimPrefix(strings.ToLower(host), "www."))
	for _, domain := range allowedDomains {
		domain = strings.ToLower(strings.TrimPrefix(strings.ToLower(domain), "www."))
		if host == domain || strings.HasSuffix(host, "."+domain) {
			return true
		}
	}
	return false
}
