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
	Title    string `json:"title"`
	URL      string `json:"url"`
	Content  string `json:"content" jsonschema_description:"A snippet of the matching page's content — call read_article on the url for the full body before treating this as evidence."`
	ImageURL string `json:"image_url,omitempty"`
}

type searchResponse struct {
	Results []searchResultItem `json:"results"`
}

type tavilySearchRequestBody struct {
	Query          string   `json:"query"`
	MaxResults     int      `json:"max_results"`
	IncludeDomains []string `json:"include_domains,omitempty"`
	Topic          string   `json:"topic"`
	IncludeImages  bool     `json:"include_images"`
}

// NewSearchTool replaces the previous DuckDuckGo-backed tool (no stable
// API, scraping-based, flagged by eino-ext itself as "not recommended for
// production" — and confirmed live, repeatedly, as an actual real-world
// failure point: a local network TLS-intercepting filter broke it
// entirely for this user, docs/plans/agent-task-manager-code-implementation.md
// §7.AA) with Tavily, a search API purpose-built for LLM agents (results
// come back pre-scored for relevance, not raw HTML to scrape).
func NewSearchTool(apiKey string, maxResults int, trustedDomains []string) (tool.InvokableTool, error) {
	return utils.InferTool(
		"web_search",
		"Searches the web for current news and articles relevant to the trigger condition. Returns search results with title, url, and snippet.",
		func(ctx context.Context, req searchRequest) (searchResponse, error) {
			return tavilySearch(ctx, apiKey, req.Query, maxResults, trustedDomains)
		},
	)
}

func tavilySearch(ctx context.Context, apiKey, query string, maxResults int, includeDomains []string) (searchResponse, error) {
	resp, err := executeTavilySearch(ctx, apiKey, query, maxResults, includeDomains)
	if err != nil {
		return searchResponse{}, err
	}
	if len(resp.Results) > 0 {
		return resp, nil
	}

	// Auto-fallback: if trusted search returned 0 results and includeDomains was specified,
	// run 1 fallback search without include_domains so external reporting is discoverable.
	if len(includeDomains) > 0 {
		fallbackResp, err := executeTavilySearch(ctx, apiKey, query, maxResults, nil)
		if err != nil {
			return resp, nil
		}
		for i := range fallbackResp.Results {
			fallbackResp.Results[i].Title = "[Sumber Eksternal] " + fallbackResp.Results[i].Title
		}
		return fallbackResp, nil
	}

	return resp, nil
}

func executeTavilySearch(ctx context.Context, apiKey, query string, maxResults int, includeDomains []string) (searchResponse, error) {
	body, err := json.Marshal(tavilySearchRequestBody{
		Query:          query,
		MaxResults:     maxResults,
		IncludeDomains: includeDomains,
		Topic:          "news",
		IncludeImages:  true,
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
		Images  []string           `json:"images"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return searchResponse{}, fmt.Errorf("web_search: parse response: %w", err)
	}
	for i := range parsed.Results {
		if parsed.Results[i].ImageURL == "" && i < len(parsed.Images) {
			parsed.Results[i].ImageURL = parsed.Images[i]
		}
	}
	return searchResponse{Results: parsed.Results}, nil
}

type readArticleRequest struct {
	URLs []string `json:"urls,omitempty" jsonschema_description:"A list of article URLs from search results to fetch and read in full in a single batch call. Only URLs matching the trusted domain allowlist will be fetched."`
	URL  string   `json:"url,omitempty" jsonschema_description:"A single article URL to fetch and read in full — must be on the trusted domain allowlist. (Prefer 'urls' to read multiple trusted sources in one call)."`
}

type readArticleItem struct {
	URL         string `json:"url"`
	Title       string `json:"title"`
	Content     string `json:"content"`
	Excerpt     string `json:"excerpt,omitempty"`
	SiteName    string `json:"site_name,omitempty"`
	ImageURL    string `json:"image_url,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
}

type readArticleResponse struct {
	Articles []readArticleItem `json:"articles" jsonschema_description:"List of extracted articles with full readable content, title, excerpt, and source metadata."`
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

// NewReadArticleTool builds a tool that fetches one or more URLs and extracts
// readable article text using Tavily Extract API (bypassing bot protections
// and JS rendering) with a safe fallback to Mozilla's Readability algorithm,
// refusing any URL whose host isn't in allowedDomains.
func NewReadArticleTool(apiKey string, allowedDomains []string) (tool.InvokableTool, error) {
	return utils.InferTool(
		"read_article",
		"Fetches news articles and returns their full readable text (title + body), stripped of ads/navigation/scripts. Supports batch extraction of multiple URLs via 'urls' parameter. Only works for URLs on the trusted domain allowlist — refuses any other domain.",
		func(ctx context.Context, req readArticleRequest) (readArticleResponse, error) {
			return fetchArticles(ctx, apiKey, req, allowedDomains)
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

func tavilyExtractBatch(ctx context.Context, apiKey string, targetURLs []string) (readArticleResponse, error) {
	body, err := json.Marshal(tavilyExtractRequestBody{
		URLs:         targetURLs,
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

	var articles []readArticleItem
	for _, result := range parsed.Results {
		content := strings.TrimSpace(result.RawContent)
		if content == "" {
			continue
		}
		var imageURL string
		if len(result.Images) > 0 {
			imageURL = result.Images[0]
		}
		parsedURL, _ := url.Parse(result.URL)
		hostname := ""
		if parsedURL != nil {
			hostname = parsedURL.Hostname()
		}
		excerpt := ""
		runes := []rune(content)
		if len(runes) > 300 {
			excerpt = string(runes[:300]) + "..."
		} else {
			excerpt = content
		}
		articles = append(articles, readArticleItem{
			URL:      result.URL,
			Title:    result.Title,
			Content:  content,
			Excerpt:  excerpt,
			SiteName: siteNameFromHost(hostname),
			ImageURL: imageURL,
		})
	}

	if len(articles) == 0 {
		if len(parsed.FailedResults) > 0 {
			return readArticleResponse{}, fmt.Errorf("read_article: extract failed: %s", parsed.FailedResults[0].Error)
		}
		return readArticleResponse{}, fmt.Errorf("read_article: no content extracted")
	}

	return readArticleResponse{
		Articles: articles,
	}, nil
}

func fetchSingleReadability(rawURL string, allowedDomains []string) (readArticleItem, error) {
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return readArticleItem{}, fmt.Errorf("read_article: invalid URL: %w", err)
	}
	if !hostAllowed(parsed.Hostname(), allowedDomains) {
		return readArticleItem{}, fmt.Errorf("read_article: %q is not on the trusted domain allowlist, refusing to fetch", parsed.Hostname())
	}

	article, err := readability.FromURL(rawURL, 15*time.Second, func(r *http.Request) {
		r.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	})
	if err != nil {
		return readArticleItem{}, fmt.Errorf("read_article: fetch/parse failed: %w", err)
	}

	if article.Node == nil {
		return readArticleItem{}, fmt.Errorf("read_article: no readable article found on %s", parsed.Hostname())
	}

	var body strings.Builder
	if err := article.RenderText(&body); err != nil {
		return readArticleItem{}, fmt.Errorf("read_article: render text failed: %w", err)
	}

	var publishedAt string
	if t, err := article.PublishedTime(); err == nil {
		publishedAt = t.Format(time.RFC3339)
	}

	siteName := article.SiteName()
	if siteName == "" {
		siteName = siteNameFromHost(parsed.Hostname())
	}

	return readArticleItem{
		URL:         rawURL,
		Title:       article.Title(),
		Content:     body.String(),
		Excerpt:     article.Excerpt(),
		SiteName:    siteName,
		ImageURL:    article.ImageURL(),
		PublishedAt: publishedAt,
	}, nil
}

func fetchArticles(ctx context.Context, apiKey string, req readArticleRequest, allowedDomains []string) (readArticleResponse, error) {
	var candidateURLs []string
	if len(req.URLs) > 0 {
		candidateURLs = append(candidateURLs, req.URLs...)
	}
	if req.URL != "" {
		candidateURLs = append(candidateURLs, req.URL)
	}

	var uniqueURLs []string
	seen := make(map[string]bool)
	for _, u := range candidateURLs {
		trimmed := strings.TrimSpace(u)
		if trimmed != "" && !seen[trimmed] {
			seen[trimmed] = true
			uniqueURLs = append(uniqueURLs, trimmed)
		}
	}

	if len(uniqueURLs) == 0 {
		return readArticleResponse{}, fmt.Errorf("read_article: no URL provided")
	}

	var validURLs []string
	for _, rawURL := range uniqueURLs {
		parsed, err := url.ParseRequestURI(rawURL)
		if err != nil {
			if len(uniqueURLs) == 1 {
				return readArticleResponse{}, fmt.Errorf("read_article: invalid URL: %w", err)
			}
			continue
		}
		if !hostAllowed(parsed.Hostname(), allowedDomains) {
			if len(uniqueURLs) == 1 {
				return readArticleResponse{}, fmt.Errorf("read_article: %q is not on the trusted domain allowlist, refusing to fetch", parsed.Hostname())
			}
			continue
		}
		validURLs = append(validURLs, rawURL)
	}

	if len(validURLs) == 0 {
		return readArticleResponse{}, fmt.Errorf("read_article: none of the provided URLs are on the trusted domain allowlist")
	}

	if apiKey != "" {
		res, err := tavilyExtractBatch(ctx, apiKey, validURLs)
		if err == nil && len(res.Articles) > 0 {
			return res, nil
		}
	}

	// Fallback to readability per URL
	var articles []readArticleItem
	for _, rawURL := range validURLs {
		single, err := fetchSingleReadability(rawURL, allowedDomains)
		if err == nil {
			articles = append(articles, single)
		}
	}

	if len(articles) == 0 {
		return readArticleResponse{}, fmt.Errorf("read_article: failed to extract readable content from provided URLs")
	}

	return readArticleResponse{
		Articles: articles,
	}, nil
}

func NewNewsTools() (readArticleTool, webSearchTool tool.BaseTool, err error) {
	apiKey := config.GetEnv("TAVILY_API_KEY")
	readArticleTool, err = NewReadArticleTool(apiKey, TrustedNewsDomains)
	if err != nil {
		return nil, nil, fmt.Errorf("analyzer: build read_article tool: %w", err)
	}
	webSearchTool, err = NewSearchTool(apiKey, 10, TrustedNewsDomains)
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
