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

package route

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/pkg/errors"
	"k8s.io/utils/ptr"

	"github.com/vandeefeng/zenfeed/pkg/component"
	"github.com/vandeefeng/zenfeed/pkg/llm"
	"github.com/vandeefeng/zenfeed/pkg/model"
	"github.com/vandeefeng/zenfeed/pkg/schedule/rule"
	"github.com/vandeefeng/zenfeed/pkg/storage/feed/block"
	"github.com/vandeefeng/zenfeed/pkg/telemetry"
	telemetrymodel "github.com/vandeefeng/zenfeed/pkg/telemetry/model"
	runtimeutil "github.com/vandeefeng/zenfeed/pkg/util/runtime"
	timeutil "github.com/vandeefeng/zenfeed/pkg/util/time"
)

// --- Interface code block ---
type Router interface {
	component.Component
	Route(ctx context.Context, result *rule.Result) (groups []*Group, err error)
}

type Config struct {
	Route
}

type Route struct {
	GroupBy                    []string
	SourceLabel                string
	SummaryPrompt              string
	LLM                        string
	CompressByRelatedThreshold *float32
	Receivers                  []string
	SubRoutes                  SubRoutes
}

type SubRoutes []*SubRoute

func (s SubRoutes) Match(feed *block.FeedVO) *SubRoute {
	for _, sub := range s {
		if matched := sub.Match(feed); matched != nil {
			return matched
		}
	}

	return nil
}

type SubRoute struct {
	Route
	Matchers []string
	matchers []matcher
}

func (r *SubRoute) Match(feed *block.FeedVO) *SubRoute {
	for _, subRoute := range r.SubRoutes {
		if matched := subRoute.Match(feed); matched != nil {
			return matched
		}
	}
	for _, m := range r.matchers {
		fv := feed.Labels.Get(m.key)
		switch m.equal {
		case true:
			if fv != m.value {
				return nil
			}
		default:
			if fv == m.value {
				return nil
			}
		}
	}

	return r
}

type matcher struct {
	key   string
	value string
	equal bool
}

var (
	matcherEqual    = "="
	matcherNotEqual = "!="
	parseMatcher    = func(filter string) (matcher, error) {
		eq := false
		parts := strings.Split(filter, matcherNotEqual)
		if len(parts) != 2 {
			parts = strings.Split(filter, matcherEqual)
			eq = true
		}
		if len(parts) != 2 {
			return matcher{}, errors.New("invalid matcher")
		}

		return matcher{key: parts[0], value: parts[1], equal: eq}, nil
	}
)

func (r *SubRoute) Validate() error {
	if len(r.GroupBy) == 0 {
		r.GroupBy = []string{model.LabelSource}
	}
	if r.CompressByRelatedThreshold == nil {
		r.CompressByRelatedThreshold = ptr.To(float32(0.85))
	}
	if len(r.Matchers) == 0 {
		return errors.New("matchers is required")
	}
	r.matchers = make([]matcher, len(r.Matchers))
	for i, matcher := range r.Matchers {
		m, err := parseMatcher(matcher)
		if err != nil {
			return errors.Wrap(err, "invalid matcher")
		}
		r.matchers[i] = m
	}
	for _, subRoute := range r.SubRoutes {
		if err := subRoute.Validate(); err != nil {
			return errors.Wrap(err, "invalid sub_route")
		}
	}

	return nil
}

func (c *Config) Validate() error {
	if len(c.GroupBy) == 0 {
		c.GroupBy = []string{model.LabelSource}
	}
	if c.CompressByRelatedThreshold == nil {
		c.CompressByRelatedThreshold = ptr.To(float32(0.85))
	}
	for _, subRoute := range c.SubRoutes {
		if err := subRoute.Validate(); err != nil {
			return errors.Wrap(err, "invalid sub_route")
		}
	}

	return nil
}

type Dependencies struct {
	RelatedScore func(a, b [][]float32) (float32, error) // MUST same with vector index.
	LLMFactory   llm.Factory
}

type Group struct {
	FeedGroup
	Receivers []string
}

type FeedGroup struct {
	Name    string
	Time    time.Time
	Labels  model.Labels
	Feeds   []*Feed
	Summary string
}

func (g *FeedGroup) ID() string {
	return fmt.Sprintf("%s-%s", g.Name, timeutil.Format(g.Time))
}

type Feed struct {
	*model.Feed
	Related []*Feed     `json:"related"`
	Vectors [][]float32 `json:"-"`
}

// --- Factory code block ---
type Factory component.Factory[Router, Config, Dependencies]

func NewFactory(mockOn ...component.MockOption) Factory {
	if len(mockOn) > 0 {
		return component.FactoryFunc[Router, Config, Dependencies](
			func(instance string, config *Config, dependencies Dependencies) (Router, error) {
				m := &mockRouter{}
				component.MockOptions(mockOn).Apply(&m.Mock)

				return m, nil
			},
		)
	}

	return component.FactoryFunc[Router, Config, Dependencies](new)
}

func new(instance string, config *Config, dependencies Dependencies) (Router, error) {
	return &router{
		Base: component.New(&component.BaseConfig[Config, Dependencies]{
			Name:         "NotifyRouter",
			Instance:     instance,
			Config:       config,
			Dependencies: dependencies,
		}),
	}, nil
}

// --- Implementation code block ---
type router struct {
	*component.Base[Config, Dependencies]
}

func (r *router) Route(ctx context.Context, result *rule.Result) (groups []*Group, err error) {
	ctx = telemetry.StartWith(ctx, append(r.TelemetryLabels(), telemetrymodel.KeyOperation, "Route")...)
	defer func() { telemetry.End(ctx, err) }()

	// Find route for each feed.
	feedsByRoute := r.routeFeeds(result.Feeds)

	// Process each route and its feeds.
	for route, feeds := range feedsByRoute {
		// Group feeds by labels.
		groupedFeeds := r.groupFeedsByLabels(route, feeds)

		// Compress related feeds.
		relatedGroups, err := r.compressRelatedFeeds(route, groupedFeeds)
		if err != nil {
			return nil, errors.Wrap(err, "compress related feeds")
		}

		// Build final groups.
		for ls, feeds := range relatedGroups {
			var summary string
			if prompt := route.SummaryPrompt; prompt != "" && len(feeds) > 1 {
				// TODO: Avoid potential for duplicate generation.
				summary, err = r.generateSummary(ctx, prompt, feeds, route.SourceLabel)
				if err != nil {
					return nil, errors.Wrap(err, "generate summary")
				}
			}
			groups = append(groups, &Group{
				FeedGroup: FeedGroup{
					Name:    fmt.Sprintf("%s  %s", result.Rule, ls.String()),
					Time:    result.Time,
					Labels:  *ls,
					Feeds:   feeds,
					Summary: summary,
				},
				Receivers: route.Receivers,
			})
		}
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Name < groups[j].Name
	})

	return groups, nil
}

func (r *router) generateSummary(
	ctx context.Context,
	prompt string,
	feeds []*Feed,
	sourceLabel string,
) (string, error) {
	content := r.parseContentToSummary(feeds, sourceLabel)
	if content == "" {
		return "", nil
	}

	llm := r.Dependencies().LLMFactory.Get(r.Config().LLM)
	summary, err := llm.String(ctx, []string{
		content,
		prompt,
	})
	if err != nil {
		return "", errors.Wrap(err, "llm string")
	}

	return summary, nil
}

func (r *router) parseContentToSummary(feeds []*Feed, sourceLabel string) string {
	if sourceLabel == "" {
		b := runtimeutil.Must1(json.Marshal(feeds))

		return string(b)
	}

	var sb strings.Builder
	for _, feed := range feeds {
		sb.WriteString(feed.Labels.Get(sourceLabel))
	}

	return sb.String()
}

func (r *router) routeFeeds(feeds []*block.FeedVO) map[*Route][]*block.FeedVO {
	config := r.Config()
	feedsByRoute := make(map[*Route][]*block.FeedVO)
	for _, feed := range feeds {
		var targetRoute *Route
		if matched := config.SubRoutes.Match(feed); matched != nil {
			targetRoute = &matched.Route
		} else {
			// Fallback to default route.
			targetRoute = &config.Route
		}
		feedsByRoute[targetRoute] = append(feedsByRoute[targetRoute], feed)
	}

	return feedsByRoute
}

func (r *router) groupFeedsByLabels(route *Route, feeds []*block.FeedVO) map[*model.Labels][]*block.FeedVO {
	groupedFeeds := make(map[*model.Labels][]*block.FeedVO)

	labelGroups := make(map[string]*model.Labels)
	for _, feed := range feeds {
		var group model.Labels
		for _, key := range route.GroupBy {
			value := feed.Labels.Get(key)
			group.Put(key, value, true)
		}

		groupKey := group.String()
		labelGroup, exists := labelGroups[groupKey]
		if !exists {
			labelGroups[groupKey] = &group
			labelGroup = &group
		}

		groupedFeeds[labelGroup] = append(groupedFeeds[labelGroup], feed)
	}

	for _, feeds := range groupedFeeds {
		sort.Slice(feeds, func(i, j int) bool {
			return feeds[i].ID < feeds[j].ID
		})
	}

	return groupedFeeds
}

func (r *router) compressRelatedFeeds(
	route *Route, // config
	groupedFeeds map[*model.Labels][]*block.FeedVO, // group id -> feeds
) (map[*model.Labels][]*Feed, error) { // group id -> feeds with related feeds
	result := make(map[*model.Labels][]*Feed)

	for ls, feeds := range groupedFeeds { // per group
		fs, err := r.compressRelatedFeedsForGroup(route, feeds)
		if err != nil {
			return nil, errors.Wrap(err, "compress related feeds")
		}
		result[ls] = fs
	}

	return result, nil
}

func (r *router) compressRelatedFeedsForGroup(
	route *Route, // config
	feeds []*block.FeedVO, // feeds
) ([]*Feed, error) {
	feedsWithRelated := make([]*Feed, 0, len(feeds)/2)
	for _, feed := range feeds {

		foundRelated := false
		for i := range feedsWithRelated {
			// Try join with previous feeds.
			score, err := r.Dependencies().RelatedScore(feedsWithRelated[i].Vectors, feed.Vectors)
			if err != nil {
				return nil, errors.Wrap(err, "related score")
			}

			if score >= *route.CompressByRelatedThreshold {
				foundRelated = true
				feedsWithRelated[i].Related = append(feedsWithRelated[i].Related, &Feed{
					Feed: feed.Feed,
				})

				break
			}
		}

		// If not found related, create a group by itself.
		if !foundRelated {
			feedsWithRelated = append(feedsWithRelated, &Feed{
				Feed:    feed.Feed,
				Vectors: feed.Vectors,
			})
		}
	}

	// Sort.
	sort.Slice(feedsWithRelated, func(i, j int) bool {
		return feedsWithRelated[i].ID < feedsWithRelated[j].ID
	})
	for _, feed := range feedsWithRelated {
		sort.Slice(feed.Related, func(i, j int) bool {
			return feed.Related[i].ID < feed.Related[j].ID
		})
	}

	return feedsWithRelated, nil
}

type mockRouter struct {
	component.Mock
}

func (m *mockRouter) Route(ctx context.Context, result *rule.Result) (groups []*Group, err error) {
	m.Called(ctx, result)

	return groups, err
}
