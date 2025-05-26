package scraper

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/vandeefeng/zenfeed/pkg/storage/kv"
	"github.com/vandeefeng/zenfeed/pkg/telemetry/log"
	textconvert "github.com/vandeefeng/zenfeed/pkg/util/text_convert"
	timeutil "github.com/vandeefeng/zenfeed/pkg/util/time"
)

// --- Types ---

// Crawl4AIHandler handles the crawl4ai related operations
type Crawl4AIHandler struct {
	client *Crawl4AIClient
	kv     kv.Storage
	past   time.Duration
}

// Crawl4AIClient handles communication with Crawl4AI service
type Crawl4AIClient struct {
	endpoint string
	client   *http.Client
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

// --- Handler Methods ---

// NewCrawl4AIHandler creates a new Crawl4AIHandler
func NewCrawl4AIHandler(endpoint string, kvStorage kv.Storage, past time.Duration) *Crawl4AIHandler {
	var client *Crawl4AIClient
	if endpoint != "" {
		client = &Crawl4AIClient{
			endpoint: endpoint,
			client:   &http.Client{},
		}
	}

	return &Crawl4AIHandler{
		client: client,
		kv:     kvStorage,
		past:   past,
	}
}

// ShouldCrawl checks if the content needs to be crawled based on its characteristics
func (h *Crawl4AIHandler) ShouldCrawl(content string) bool {
	return content == "" || // No content from RSS
		(len(content) < 100 && !strings.Contains(content, "</table>")) // Content too short and not a data table
}

// isInTimeRange checks if the feed is within the configured time range
func (h *Crawl4AIHandler) isInTimeRange(pubTime, now time.Time) bool {
	return timeutil.InRange(pubTime, now.Add(-h.past), now)
}

// isAlreadyRead checks if the feed has already been read
func (h *Crawl4AIHandler) isAlreadyRead(ctx context.Context, url string) bool {
	if h.kv == nil {
		return false
	}

	key := fmt.Sprintf("feed:%s:read_status", url)
	status, err := h.kv.Get(ctx, key)
	return err == nil && status == "read"
}

// canCrawl checks if crawling is possible and necessary
func (h *Crawl4AIHandler) canCrawl(ctx context.Context, url string, pubTime, now time.Time) bool {
	return h.client != nil &&
		url != "" &&
		h.isInTimeRange(pubTime, now) &&
		!h.isAlreadyRead(ctx, url)
}

// GetFullContent tries to get the full content for a URL if it meets all criteria
func (h *Crawl4AIHandler) GetFullContent(ctx context.Context, url string, pubTime time.Time, now time.Time) (string, error) {
	if !h.canCrawl(ctx, url, pubTime, now) {
		return "", nil
	}

	fullContent, err := h.client.GetFullContent(ctx, url)
	if err != nil {
		return "", errors.Wrap(err, "getting full content")
	}

	return fullContent, nil
}

// FormatContent formats the content based on its source and format
func (h *Crawl4AIHandler) FormatContent(content string, originalContent string, originalDesc string) (string, error) {
	// If content is from Crawl4AI (doesn't contain original RSS content), use as is
	if !strings.Contains(content, originalContent) && !strings.Contains(content, originalDesc) {
		return content, nil
	}

	// Content is from RSS, convert from HTML to markdown
	mdContent, err := textconvert.HTMLToMarkdown([]byte(content))
	if err != nil {
		return "", errors.Wrap(err, "converting content to markdown")
	}
	return string(mdContent), nil
}

// --- Client Methods ---

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
