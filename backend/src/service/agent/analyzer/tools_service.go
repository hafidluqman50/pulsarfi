package analyzer

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	readability "codeberg.org/readeck/go-readability/v2"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/horizonlabs/pulsarfi-backend/src/config"
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
	"bisnis.com",
	"cnbcindonesia.com",
}

type searchRequest struct {
	Query string `json:"query" jsonschema_description:"The search query — the trigger condition's exact subject, not a paraphrase."`
}

type searchResultItem struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Content string `json:"content" jsonschema_description:"A snippet of the matching page's content — call read_article on the url for the full body before treating this as evidence."`
}

type searchResponse struct {
	Results []searchResultItem `json:"results"`
}

type tavilySearchRequestBody struct {
	Query          string   `json:"query"`
	MaxResults     int      `json:"max_results"`
	IncludeDomains []string `json:"include_domains,omitempty"`
	Topic          string   `json:"topic"`
}

// NewSearchTool replaces the previous DuckDuckGo-backed tool (no stable
// API, scraping-based, flagged by eino-ext itself as "not recommended for
// production" — and confirmed live, repeatedly, as an actual real-world
// failure point: a local network TLS-intercepting filter broke it
// entirely for this user, docs/plans/agent-task-manager-code-implementation.md
// §7.AA) with Tavily, a search API purpose-built for LLM agents (results
// come back pre-scored for relevance, not raw HTML to scrape). Restricted
// to trustedDomains via Tavily's own include_domains parameter — the same
// allowlist read_article already enforces (TrustedNewsDomains, this file),
// so untrusted results are filtered out at search time too, not only when
// an article is actually fetched.
func NewSearchTool(apiKey string, maxResults int, trustedDomains []string) (tool.InvokableTool, error) {
	return utils.InferTool(
		"web_search",
		"Searches the web for current news/information relevant to the trigger condition, restricted to a trusted domain allowlist. Returns a snippet per result — always call read_article on a promising result's url for the full body before treating it as evidence.",
		func(ctx context.Context, req searchRequest) (searchResponse, error) {
			return tavilySearch(ctx, apiKey, req.Query, maxResults, trustedDomains)
		},
	)
}

func tavilySearch(ctx context.Context, apiKey, query string, maxResults int, includeDomains []string) (searchResponse, error) {
	body, err := json.Marshal(tavilySearchRequestBody{
		Query:          query,
		MaxResults:     maxResults,
		IncludeDomains: includeDomains,
		Topic:          "news",
	})
	if err != nil {
		return searchResponse{}, fmt.Errorf("web_search: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.tavily.com/search", bytes.NewReader(body))
	if err != nil {
		return searchResponse{}, fmt.Errorf("web_search: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return searchResponse{}, fmt.Errorf("web_search: request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return searchResponse{}, fmt.Errorf("web_search: read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return searchResponse{}, fmt.Errorf("web_search: tavily returned %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Results []searchResultItem `json:"results"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return searchResponse{}, fmt.Errorf("web_search: parse response: %w", err)
	}
	return searchResponse{Results: parsed.Results}, nil
}

type readArticleRequest struct {
	URL string `json:"url" jsonschema_description:"The exact article URL from a search result to fetch and read in full — must be on the trusted domain allowlist, never a URL you invent or guess."`
}

type readArticleResponse struct {
	Title       string `json:"title" jsonschema_description:"The article's headline."`
	Content     string `json:"content" jsonschema_description:"The article's full readable body text, stripped of ads/navigation/scripts."`
	Excerpt     string `json:"excerpt,omitempty" jsonschema_description:"A short summary/dek pulled from the article's own metadata, when the page provides one."`
	SiteName    string `json:"site_name,omitempty" jsonschema_description:"The publication's own name, from the article's metadata (e.g. 'Kompas.com')."`
	ImageURL    string `json:"image_url,omitempty" jsonschema_description:"The article's lead image, when the page provides one."`
	PublishedAt string `json:"published_at,omitempty" jsonschema_description:"When the article was actually published, RFC3339, only present when the page's own metadata states it — never guessed or left as the fetch time."`
}

type tavilyExtractRequestBody struct {
	URLs         []string `json:"urls"`
	ExtractDepth string   `json:"extract_depth,omitempty"`
}

type tavilyExtractResultItem struct {
	URL        string   `json:"url"`
	RawContent string   `json:"raw_content"`
	Title      string   `json:"title"`
	Images     []string `json:"images,omitempty"`
}

type tavilyExtractFailedItem struct {
	URL   string `json:"url"`
	Error string `json:"error"`
}

type tavilyExtractResponseBody struct {
	Results       []tavilyExtractResultItem `json:"results"`
	FailedResults []tavilyExtractFailedItem `json:"failed_results"`
}

// NewReadArticleTool builds a tool that fetches a URL and extracts its
// readable article text using Tavily Extract API (bypassing bot protections
// and JS rendering) with a safe fallback to Mozilla's Readability algorithm,
// refusing any URL whose host isn't in allowedDomains.
func NewReadArticleTool(apiKey string, allowedDomains []string) (tool.InvokableTool, error) {
	return utils.InferTool(
		"read_article",
		"Fetches a news article URL and returns its full readable text (title + body), stripped of ads/navigation/scripts. Only works for URLs on the trusted domain allowlist — refuses any other domain.",
		func(ctx context.Context, req readArticleRequest) (readArticleResponse, error) {
			return fetchArticle(ctx, apiKey, req.URL, allowedDomains)
		},
	)
}

func siteNameFromHost(host string) string {
	lower := strings.ToLower(strings.TrimPrefix(host, "www."))
	switch {
	case strings.Contains(lower, "kompas.com"):
		return "Kompas.com"
	case strings.Contains(lower, "liputan6.com"):
		return "Liputan6.com"
	case strings.Contains(lower, "bisnis.com"):
		return "Bisnis.com"
	case strings.Contains(lower, "cnbcindonesia.com"):
		return "CNBC Indonesia"
	default:
		return host
	}
}

func tavilyExtract(ctx context.Context, apiKey, targetURL string) (readArticleResponse, error) {
	body, err := json.Marshal(tavilyExtractRequestBody{
		URLs:         []string{targetURL},
		ExtractDepth: "basic",
	})
	if err != nil {
		return readArticleResponse{}, fmt.Errorf("read_article: marshal extract request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.tavily.com/extract", bytes.NewReader(body))
	if err != nil {
		return readArticleResponse{}, fmt.Errorf("read_article: build extract request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return readArticleResponse{}, fmt.Errorf("read_article: extract request failed: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return readArticleResponse{}, fmt.Errorf("read_article: read extract response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return readArticleResponse{}, fmt.Errorf("read_article: tavily extract returned %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed tavilyExtractResponseBody
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return readArticleResponse{}, fmt.Errorf("read_article: parse extract response: %w", err)
	}

	if len(parsed.Results) == 0 || strings.TrimSpace(parsed.Results[0].RawContent) == "" {
		if len(parsed.FailedResults) > 0 {
			return readArticleResponse{}, fmt.Errorf("read_article: extract failed for %s: %s", targetURL, parsed.FailedResults[0].Error)
		}
		return readArticleResponse{}, fmt.Errorf("read_article: no content extracted for %s", targetURL)
	}

	result := parsed.Results[0]
	var imageURL string
	if len(result.Images) > 0 {
		imageURL = result.Images[0]
	}

	parsedURL, _ := url.Parse(targetURL)
	hostname := ""
	if parsedURL != nil {
		hostname = parsedURL.Hostname()
	}

	content := strings.TrimSpace(result.RawContent)
	excerpt := ""
	runes := []rune(content)
	if len(runes) > 300 {
		excerpt = string(runes[:300]) + "..."
	} else {
		excerpt = content
	}

	return readArticleResponse{
		Title:    result.Title,
		Content:  content,
		Excerpt:  excerpt,
		SiteName: siteNameFromHost(hostname),
		ImageURL: imageURL,
	}, nil
}

func fetchArticle(ctx context.Context, apiKey, rawURL string, allowedDomains []string) (readArticleResponse, error) {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return readArticleResponse{}, fmt.Errorf("read_article: invalid URL: %w", err)
	}
	if !hostAllowed(parsed.Hostname(), allowedDomains) {
		return readArticleResponse{}, fmt.Errorf("read_article: %q is not on the trusted domain allowlist, refusing to fetch", parsed.Hostname())
	}

	if apiKey != "" {
		res, extractErr := tavilyExtract(ctx, apiKey, rawURL)
		if extractErr == nil {
			return res, nil
		}
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

	if article.Node == nil {
		return readArticleResponse{}, fmt.Errorf("read_article: no readable article found on %s", parsed.Hostname())
	}

	var body strings.Builder
	if err := article.RenderText(&body); err != nil {
		return readArticleResponse{}, fmt.Errorf("read_article: render text failed: %w", err)
	}

	var publishedAt string
	if t, err := article.PublishedTime(); err == nil {
		publishedAt = t.Format(time.RFC3339)
	}

	siteName := article.SiteName()
	if siteName == "" {
		siteName = siteNameFromHost(parsed.Hostname())
	}

	return readArticleResponse{
		Title:       article.Title(),
		Content:     body.String(),
		Excerpt:     article.Excerpt(),
		SiteName:    siteName,
		ImageURL:    article.ImageURL(),
		PublishedAt: publishedAt,
	}, nil
}

func NewNewsTools() (readArticleTool, webSearchTool tool.BaseTool, err error) {
	apiKey := config.GetEnv("TAVILY_API_KEY")
	readArticleTool, err = NewReadArticleTool(apiKey, TrustedNewsDomains)
	if err != nil {
		return nil, nil, fmt.Errorf("analyzer: build read_article tool: %w", err)
	}
	webSearchTool, err = NewSearchTool(apiKey, 5, TrustedNewsDomains)
	if err != nil {
		return nil, nil, fmt.Errorf("analyzer: build web_search tool: %w", err)
	}
	return readArticleTool, webSearchTool, nil
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
