// Copyright (C) 2025 wangyusong
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <https://www.gnu.org/licenses/>.

package scraper

import (
	"context"
	"strings"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"k8s.io/utils/ptr"

	"github.com/vandeefeng/zenfeed/pkg/model"
	"github.com/vandeefeng/zenfeed/pkg/telemetry/log"
	"github.com/vandeefeng/zenfeed/pkg/util/retry"
	textconvert "github.com/vandeefeng/zenfeed/pkg/util/text_convert"
)

// --- Interface code block ---
type ScrapeSourceRSS struct {
	URL             string
	RSSHubEndpoint  string
	RSSHubRoutePath string
	CrawlerEndpoint string // Crawl4AI endpoint for fetching full content
}

func (c *ScrapeSourceRSS) Validate() error {
	if c.URL == "" && c.RSSHubEndpoint == "" {
		return errors.New("URL or RSSHubEndpoint can not be empty at the same time")
	}
	if c.URL == "" {
		c.URL = strings.TrimSuffix(c.RSSHubEndpoint, "/") + "/" + strings.TrimPrefix(c.RSSHubRoutePath, "/")
	}
	if c.URL != "" && !strings.HasPrefix(c.URL, "http://") && !strings.HasPrefix(c.URL, "https://") {
		return errors.New("URL must be a valid HTTP/HTTPS URL")
	}

	return nil
}

// --- Factory code block ---
func newRSSReader(config *ScrapeSourceRSS) (reader, error) {
	if err := config.Validate(); err != nil {
		return nil, errors.Wrapf(err, "invalid RSS config")
	}

	var crawler *Crawl4AIClient
	if config.CrawlerEndpoint != "" {
		crawler = NewCrawl4AIClient(config.CrawlerEndpoint)
	}

	return &rssReader{
		config: config,
		client: &gofeedClient{
			url:  config.URL,
			base: gofeed.NewParser(),
		},
		crawler: crawler,
	}, nil
}

// --- Implementation code block ---

type rssReader struct {
	config  *ScrapeSourceRSS
	client  client
	crawler *Crawl4AIClient
}

func (r *rssReader) Read(ctx context.Context) ([]*model.Feed, error) {
	feed, err := r.client.Get(ctx)
	if err != nil {
		return nil, errors.Wrapf(err, "fetching RSS feed")
	}
	if len(feed.Items) == 0 {
		return []*model.Feed{}, nil
	}

	now := clk.Now()
	feeds := make([]*model.Feed, 0, len(feed.Items))
	for _, fi := range feed.Items {
		item, err := r.toResultFeed(ctx, now, fi)
		if err != nil {
			return nil, errors.Wrapf(err, "converting feed item")
		}

		feeds = append(feeds, item)
	}

	return feeds, nil
}

func (r *rssReader) toResultFeed(ctx context.Context, now time.Time, feedFeed *gofeed.Item) (*model.Feed, error) {
	var content string
	var err error

	// Try to get full content if:
	// 1. Link exists and crawler is configured
	// 2. And either:
	//    - Content is empty
	//    - Description is empty
	//    - Content length is suspiciously short (likely just a summary)
	if r.crawler != nil && feedFeed.Link != "" {
		shouldCrawl := feedFeed.Content == "" ||
			feedFeed.Description == "" ||
			(len(feedFeed.Content) < 500 && !strings.Contains(feedFeed.Content, "</table>")) // Skip crawling if content is a data table

		if shouldCrawl {
			// Use retry with backoff for crawler requests
			err = retry.Backoff(ctx, func() error {
				var fetchErr error
				fullContent, fetchErr := r.crawler.GetFullContent(ctx, feedFeed.Link)
				if fetchErr != nil {
					log.Info(ctx, "Failed to get full content, will retry", "error", fetchErr)
					return fetchErr
				}
				content = fullContent
				return nil
			}, &retry.Options{
				MinInterval: time.Second,
				MaxInterval: 5 * time.Second,
				MaxAttempts: ptr.To(3),
			})

			if err != nil {
				// After all retries failed, fallback to RSS content
				log.Info(ctx, "All attempts to get full content failed, falling back to RSS content",
					"error", err,
					"content_length", len(feedFeed.Content),
					"description_length", len(feedFeed.Description))
				content = r.combineContent(feedFeed.Content, feedFeed.Description)
			}
		} else {
			content = r.combineContent(feedFeed.Content, feedFeed.Description)
		}
	} else {
		content = r.combineContent(feedFeed.Content, feedFeed.Description)
	}

	// If content is from Crawl4AI, it's already in the desired format
	// For RSS content, convert from HTML to markdown
	if r.crawler != nil && !strings.Contains(content, feedFeed.Content) && !strings.Contains(content, feedFeed.Description) {
		// Content is from Crawl4AI, use as is
	} else {
		// Content is from RSS, convert from HTML to markdown
		mdContent, err := textconvert.HTMLToMarkdown([]byte(content))
		if err != nil {
			return nil, errors.Wrapf(err, "converting content to markdown")
		}
		content = string(mdContent)
	}

	// Create the feed item.
	feed := &model.Feed{
		Labels: model.Labels{
			{Key: model.LabelType, Value: "rss"},
			{Key: model.LabelTitle, Value: feedFeed.Title},
			{Key: model.LabelLink, Value: feedFeed.Link},
			{Key: model.LabelPubTime, Value: r.parseTime(feedFeed).Format(time.RFC3339)},
			{Key: model.LabelContent, Value: content},
		},
		Time: now,
	}

	return feed, nil
}

// parseTime parses the publication time from the feed item.
// If the feed item does not have a publication time, it returns the current time.
func (r *rssReader) parseTime(feedFeed *gofeed.Item) time.Time {
	if feedFeed.PublishedParsed == nil {
		return clk.Now().In(time.Local)
	}

	return feedFeed.PublishedParsed.In(time.Local)
}

// combineContent combines Content and Description fields with proper formatting.
func (r *rssReader) combineContent(content, description string) string {
	switch {
	case content == "":
		return description
	case description == "":
		return content
	default:
		return strings.Join([]string{description, content}, "\n\n")
	}
}

type client interface {
	Get(ctx context.Context) (*gofeed.Feed, error)
}

type gofeedClient struct {
	url  string
	base *gofeed.Parser
}

func (c *gofeedClient) Get(ctx context.Context) (*gofeed.Feed, error) {
	return c.base.ParseURLWithContext(c.url, ctx)
}

type mockClient struct {
	mock.Mock
}

func newMockClient() *mockClient {
	return &mockClient{}
}

func (c *mockClient) Get(ctx context.Context) (*gofeed.Feed, error) {
	args := c.Called(ctx)
	if args.Error(1) != nil {
		return nil, args.Error(1)
	}

	return args.Get(0).(*gofeed.Feed), nil
}
