package scraper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/vandeefeng/zenfeed/pkg/telemetry/log"
)

// Crawl4AIClient handles communication with Crawl4AI service
type Crawl4AIClient struct {
	endpoint string
	client   *http.Client
}

// NewCrawl4AIClient creates a new Crawl4AI client
func NewCrawl4AIClient(endpoint string) *Crawl4AIClient {
	return &Crawl4AIClient{
		endpoint: endpoint,
		client:   &http.Client{},
	}
}

type crawlRequest struct {
	URLs []string `json:"urls"`
}

type crawlResponse struct {
	Success bool `json:"success"`
	Results []struct {
		URL              string      `json:"url"`
		HTML             string      `json:"html"`
		Success          bool        `json:"success"`
		CleanedHTML      string      `json:"cleaned_html"`
		ExtractedContent interface{} `json:"extracted_content"`
		ErrorMessage     string      `json:"error_message"`
		Markdown         struct {
			RawMarkdown string `json:"raw_markdown"`
		} `json:"markdown"`
	} `json:"results"`
}

// GetFullContent fetches the full content of a webpage using Crawl4AI
func (c *Crawl4AIClient) GetFullContent(ctx context.Context, url string) (string, error) {
	log.Info(ctx, "Starting to crawl URL", "url", url)

	reqBody := crawlRequest{
		URLs: []string{url},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint+"/crawl", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	log.Debug(ctx, "Sending request to Crawl4AI", "endpoint", c.endpoint)
	resp, err := c.client.Do(req)
	if err != nil {
		log.Error(ctx, err, "Failed to send request to Crawl4AI", "url", url)
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		log.Error(ctx, fmt.Errorf("status code: %d", resp.StatusCode),
			"Crawl4AI returned non-200 status",
			"url", url,
			"status_code", resp.StatusCode,
			"response_body", string(body),
		)
		return "", fmt.Errorf("unexpected status code: %d, body: %s", resp.StatusCode, string(body))
	}

	var crawlResp crawlResponse
	if err := json.NewDecoder(resp.Body).Decode(&crawlResp); err != nil {
		log.Error(ctx, err, "Failed to decode Crawl4AI response", "url", url)
		return "", fmt.Errorf("decode response: %w", err)
	}

	if !crawlResp.Success || len(crawlResp.Results) == 0 {
		return "", fmt.Errorf("crawl failed: no results returned")
	}

	result := crawlResp.Results[0]
	if !result.Success {
		return "", fmt.Errorf("crawl failed for URL %s: %s", url, result.ErrorMessage)
	}

	// Prefer markdown content if available, otherwise use cleaned HTML
	content := result.Markdown.RawMarkdown
	if content == "" {
		content = result.CleanedHTML
	}
	if content == "" {
		content = result.HTML
	}

	content = strings.TrimSpace(content)
	contentLength := len(content)

	log.Info(ctx, "Successfully crawled URL",
		"url", url,
		"content_length", contentLength,
	)

	return content, nil
}
