package search

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://html.duckduckgo.com/html"
)

// Client represents a DuckDuckGo search client.
type Client struct {
	httpClient *http.Client
	baseURL    string
}

// SearchResult represents a single search result.
type SearchResult struct {
	Title   string
	URL     string
	Snippet string
}

// NewClient creates a new DuckDuckGo search client.
func NewClient(timeoutSeconds int) *Client {
	return NewClientWithBaseURL(timeoutSeconds, defaultBaseURL)
}

// NewClientWithBaseURL creates a new DuckDuckGo client with custom base URL.
func NewClientWithBaseURL(timeoutSeconds int, baseURL string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: time.Duration(timeoutSeconds) * time.Second,
		},
		baseURL: baseURL,
	}
}

// Search performs a web search and returns results.
func (c *Client) Search(ctx context.Context, query string, limit int) ([]SearchResult, error) {
	// Validate inputs
	if query == "" {
		return nil, fmt.Errorf("query cannot be empty")
	}
	if limit <= 0 || limit > 50 {
		return nil, fmt.Errorf("limit must be between 1 and 50")
	}

	// Build request URL
	params := url.Values{}
	params.Set("q", query)
	fullURL := fmt.Sprintf("%s?%s", c.baseURL, params.Encode())

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set user agent to mimic browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; NuimanBot/1.0)")

	// Execute request
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	// Check status code
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// Read response body
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// Parse HTML results
	results := c.parseResults(string(body), limit)

	return results, nil
}

// parseResults extracts search results from DuckDuckGo HTML.
func (c *Client) parseResults(html string, limit int) []SearchResult {
	results := []SearchResult{}

	// Simple regex-based parsing (for production, use an HTML parser like goquery)
	// Match result links: <a class="result__a" href="URL">Title</a>
	linkPattern := regexp.MustCompile(`<a[^>]+class="result__a"[^>]+href="([^"]+)"[^>]*>([^<]+)</a>`)
	snippetPattern := regexp.MustCompile(`<a[^>]+class="result__snippet"[^>]*>([^<]+)</a>`)

	links := linkPattern.FindAllStringSubmatch(html, -1)
	snippets := snippetPattern.FindAllStringSubmatch(html, -1)

	for i := 0; i < len(links) && i < limit; i++ {
		result := SearchResult{
			URL:   strings.TrimSpace(links[i][1]),
			Title: strings.TrimSpace(links[i][2]),
		}

		// Add snippet if available
		if i < len(snippets) {
			result.Snippet = strings.TrimSpace(snippets[i][1])
		}

		results = append(results, result)
	}

	return results
}
