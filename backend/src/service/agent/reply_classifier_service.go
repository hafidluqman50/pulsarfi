package agent

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"

	"github.com/horizonlabs/pulsarfi-backend/src/contracts"
)

// toolCallResult mirrors one tool call Nova made internally.
type toolCallResult struct {
	ToolName string
	Result   string
}

type newsEvidenceItem struct {
	Title       string `json:"title,omitempty"`
	Source      string `json:"source"`
	URL         string `json:"url,omitempty"`
	PublishedAt string `json:"published_at,omitempty"`
	Excerpt     string `json:"excerpt,omitempty"`
	ImageURL    string `json:"image_url,omitempty"`
}

var markdownImagePattern = regexp.MustCompile(`!\[.*?\]\((https?://[^\s)]+)\)`)

func extractHostName(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "News"
	}
	host := strings.TrimPrefix(u.Hostname(), "www.")
	lower := strings.ToLower(host)
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
		if host != "" {
			return host
		}
		return "News"
	}
}

func extractNewsEvidenceFromToolCalls(toolCalls []toolCallResult) []newsEvidenceItem {
	var evidence []newsEvidenceItem
	seenURLs := make(map[string]bool)

	// 1. Prioritize read_article tool calls (carries full article metadata: source, date, image, excerpt)
	for _, tc := range toolCalls {
		if tc.ToolName != "read_article" {
			continue
		}
		var resp struct {
			Articles []struct {
				URL         string `json:"url"`
				Title       string `json:"title"`
				Excerpt     string `json:"excerpt"`
				SiteName    string `json:"site_name"`
				ImageURL    string `json:"image_url"`
				PublishedAt string `json:"published_at"`
			} `json:"articles"`
		}
		if err := json.Unmarshal([]byte(tc.Result), &resp); err == nil {
			for _, a := range resp.Articles {
				if a.URL == "" || seenURLs[a.URL] {
					continue
				}
				seenURLs[a.URL] = true
				source := a.SiteName
				if source == "" {
					source = extractHostName(a.URL)
				}
				evidence = append(evidence, newsEvidenceItem{
					Title:       a.Title,
					Source:      source,
					URL:         a.URL,
					PublishedAt: a.PublishedAt,
					Excerpt:     a.Excerpt,
					ImageURL:    a.ImageURL,
				})
			}
		}
	}

	// 2. Secondary fallback: if read_article wasn't called or yielded no articles, check web_search
	if len(evidence) == 0 {
		for _, tc := range toolCalls {
			if tc.ToolName != "web_search" {
				continue
			}
			var resp struct {
				Results []struct {
					Title    string `json:"title"`
					URL      string `json:"url"`
					Content  string `json:"content"`
					ImageURL string `json:"image_url"`
				} `json:"results"`
				Images []string `json:"images"`
			}
			if err := json.Unmarshal([]byte(tc.Result), &resp); err == nil {
				for i, r := range resp.Results {
					if r.URL == "" || seenURLs[r.URL] {
						continue
					}
					seenURLs[r.URL] = true
					imgURL := r.ImageURL
					if imgURL == "" && i < len(resp.Images) {
						imgURL = resp.Images[i]
					}
					if imgURL == "" {
						if m := markdownImagePattern.FindStringSubmatch(r.Content); len(m) > 1 {
							imgURL = m[1]
						}
					}
					evidence = append(evidence, newsEvidenceItem{
						Title:    r.Title,
						Source:   extractHostName(r.URL),
						URL:      r.URL,
						Excerpt:  r.Content,
						ImageURL: imgURL,
					})
				}
			}
		}
	}

	return evidence
}

func classifyReply(pendingQuestions []contracts.IntakeField, card *contracts.CardContract, analyzerToolCalls []toolCallResult) (contentType string, uiProps json.RawMessage) {
	if len(pendingQuestions) > 0 {
		if payload, err := json.Marshal(map[string]any{
			"questions": pendingQuestions,
			"card":      card,
		}); err == nil {
			return "workflow_card", payload
		}
	}

	var chartPayloads []json.RawMessage
	for _, tc := range analyzerToolCalls {
		if tc.ToolName == "get_portfolio_snapshot" || tc.ToolName == "get_stock_chart" {
			chartPayloads = append(chartPayloads, json.RawMessage(tc.Result))
		}
	}
	newsEvidence := extractNewsEvidenceFromToolCalls(analyzerToolCalls)

	// If both charts and news evidence are present, deliver both via a composite payload
	// using "news" as the database-safe content_type enum
	if len(chartPayloads) > 0 && len(newsEvidence) > 0 {
		var chartData any = chartPayloads
		if len(chartPayloads) == 1 {
			chartData = chartPayloads[0]
		}
		if payload, err := json.Marshal(map[string]any{
			"charts": chartData,
			"news":   newsEvidence,
		}); err == nil {
			return "news", payload
		}
	}

	if len(chartPayloads) == 1 {
		return "chart", chartPayloads[0]
	}
	if len(chartPayloads) > 1 {
		if payload, err := json.Marshal(chartPayloads); err == nil {
			return "chart", payload
		}
	}

	if len(newsEvidence) > 0 {
		if payload, err := json.Marshal(newsEvidence); err == nil {
			return "news", payload
		}
	}

	return "text", nil
}
