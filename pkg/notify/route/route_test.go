package route

import (
	"context"
	"fmt"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"
	"k8s.io/utils/ptr"

	"github.com/vandeefeng/zenfeed/pkg/component"
	"github.com/vandeefeng/zenfeed/pkg/llm"
	"github.com/vandeefeng/zenfeed/pkg/model"
	"github.com/vandeefeng/zenfeed/pkg/schedule/rule"
	"github.com/vandeefeng/zenfeed/pkg/storage/feed/block"
	"github.com/vandeefeng/zenfeed/pkg/test"
	timeutil "github.com/vandeefeng/zenfeed/pkg/util/time"
)

func TestRoute(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct {
		config       *Config
		relatedScore func(m *mock.Mock) // Mock setup for RelatedScore.
	}
	type whenDetail struct {
		ruleResult *rule.Result
	}
	type thenExpected struct {
		groups []*Group
		isErr  bool
		errMsg string
	}

	now := time.Now()
	testFeeds := []*block.FeedVO{
		{
			Feed: &model.Feed{
				ID: 1,
				Labels: model.Labels{
					{Key: model.LabelSource, Value: "TechCrunch"},
					{Key: "category", Value: "AI"},
					{Key: model.LabelTitle, Value: "Tech News 1"},
					{Key: model.LabelLink, Value: "http://example.com/tech1"},
				},
				Time: now,
			},
			Vectors: [][]float32{{0.1, 0.2}},
		},
		{
			Feed: &model.Feed{
				ID: 2,
				Labels: model.Labels{
					{Key: model.LabelSource, Value: "TechCrunch"},
					{Key: "category", Value: "AI"},
					{Key: model.LabelTitle, Value: "Tech News 2"},
					{Key: model.LabelLink, Value: "http://example.com/tech2"},
				},
				Time: now,
			},
			Vectors: [][]float32{{0.11, 0.21}},
		},
		{
			Feed: &model.Feed{
				ID: 3,
				Labels: model.Labels{
					{Key: model.LabelSource, Value: "Bloomberg"},
					{Key: "category", Value: "Markets"},
					{Key: model.LabelTitle, Value: "Finance News 1"},
					{Key: model.LabelLink, Value: "http://example.com/finance1"},
				},
				Time: now,
			},
			Vectors: [][]float32{{0.8, 0.9}},
		},
		{
			Feed: &model.Feed{
				ID: 4,
				Labels: model.Labels{
					{Key: model.LabelSource, Value: "TechCrunch"},
					{Key: "category", Value: "Hardware"},
					{Key: model.LabelTitle, Value: "Specific Tech News"},
					{Key: model.LabelLink, Value: "http://example.com/tech_specific"},
				},
				Time: now,
			},
			Vectors: [][]float32{{0.5, 0.5}},
		},
		{
			Feed: &model.Feed{
				ID: 5,
				Labels: model.Labels{
					{Key: model.LabelSource, Value: "OtherSource"},
					{Key: "category", Value: "Sports"},
					{Key: model.LabelTitle, Value: "Non-Matching Category News"},
					{Key: model.LabelLink, Value: "http://example.com/other"},
				},
				Time: now,
			},
			Vectors: [][]float32{{0.9, 0.1}},
		},
	}

	for _, tf := range testFeeds {
		tf.Labels.EnsureSorted()
	}

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Basic routing and grouping by source",
			Given:    "a default router config grouping by source and high related threshold",
			When:     "routing feeds from different sources",
			Then:     "should group feeds by source into separate groups without compression",
			GivenDetail: givenDetail{
				config: &Config{
					Route: Route{
						GroupBy:                    []string{model.LabelSource},
						CompressByRelatedThreshold: ptr.To(float32(0.99)),
						Receivers:                  []string{"default-receiver"},
					},
				},
			},
			WhenDetail: whenDetail{
				ruleResult: &rule.Result{
					Rule:  "TestRule",
					Time:  now,
					Feeds: []*block.FeedVO{testFeeds[0], testFeeds[2], testFeeds[4]},
				},
			},
			ThenExpected: thenExpected{
				groups: []*Group{
					{
						FeedGroup: FeedGroup{
							Name:   fmt.Sprintf("TestRule  %s", model.Labels{{Key: model.LabelSource, Value: "Bloomberg"}}.String()),
							Time:   now,
							Labels: model.Labels{{Key: model.LabelSource, Value: "Bloomberg"}},
							Feeds: []*Feed{
								{Feed: testFeeds[2].Feed, Vectors: testFeeds[2].Vectors},
							},
						},
						Receivers: []string{"default-receiver"},
					},
					{
						FeedGroup: FeedGroup{
							Name:   fmt.Sprintf("TestRule  %s", model.Labels{{Key: model.LabelSource, Value: "OtherSource"}}.String()),
							Time:   now,
							Labels: model.Labels{{Key: model.LabelSource, Value: "OtherSource"}},
							Feeds: []*Feed{
								{Feed: testFeeds[4].Feed, Vectors: testFeeds[4].Vectors},
							},
						},
						Receivers: []string{"default-receiver"},
					},
					{
						FeedGroup: FeedGroup{
							Name:   fmt.Sprintf("TestRule  %s", model.Labels{{Key: model.LabelSource, Value: "TechCrunch"}}.String()),
							Time:   now,
							Labels: model.Labels{{Key: model.LabelSource, Value: "TechCrunch"}},
							Feeds: []*Feed{
								{Feed: testFeeds[0].Feed, Vectors: testFeeds[0].Vectors},
							},
						},
						Receivers: []string{"default-receiver"},
					},
				},
				isErr: false,
			},
		},
		{
			Scenario: "Routing with sub-route matching",
			Given:    "a router config with a sub-route for AI category",
			When:     "routing feeds including AI category",
			Then:     "should apply the sub-route's receivers and settings to matching feeds",
			GivenDetail: givenDetail{
				config: &Config{
					Route: Route{
						GroupBy:                    []string{model.LabelSource},
						CompressByRelatedThreshold: ptr.To(float32(0.99)),
						Receivers:                  []string{"default-receiver"},
						SubRoutes: SubRoutes{
							{
								Route: Route{
									GroupBy:                    []string{model.LabelSource, "category"},
									CompressByRelatedThreshold: ptr.To(float32(0.99)),
									Receivers:                  []string{"ai-receiver"},
								},
								Matchers: []string{"category=AI"},
							},
						},
					},
				},
				relatedScore: func(m *mock.Mock) {
					m.On("RelatedScore", mock.Anything, mock.Anything).Return(float32(0.1), nil)
				},
			},
			WhenDetail: whenDetail{
				ruleResult: &rule.Result{
					Rule:  "SubRouteRule",
					Time:  now,
					Feeds: []*block.FeedVO{testFeeds[0], testFeeds[1], testFeeds[4]},
				},
			},
			ThenExpected: thenExpected{
				groups: []*Group{
					{
						FeedGroup: FeedGroup{
							Name: fmt.Sprintf("SubRouteRule  %s", model.Labels{
								{Key: model.LabelSource, Value: "TechCrunch"},
								{Key: "category", Value: "AI"},
							}.String()),
							Time: now,
							Labels: model.Labels{
								{Key: "category", Value: "AI"},
								{Key: model.LabelSource, Value: "TechCrunch"},
							},
							Feeds: []*Feed{
								{Feed: testFeeds[0].Feed, Vectors: testFeeds[0].Vectors},
								{Feed: testFeeds[1].Feed, Vectors: testFeeds[1].Vectors},
							},
						},
						Receivers: []string{"ai-receiver"},
					},
					{
						FeedGroup: FeedGroup{
							Name:   fmt.Sprintf("SubRouteRule  %s", model.Labels{{Key: model.LabelSource, Value: "OtherSource"}}.String()),
							Time:   now,
							Labels: model.Labels{{Key: model.LabelSource, Value: "OtherSource"}},
							Feeds: []*Feed{
								{Feed: testFeeds[4].Feed, Vectors: testFeeds[4].Vectors},
							},
						},
						Receivers: []string{"default-receiver"},
					},
				},
				isErr: false,
			},
		},
		{
			Scenario: "Compressing related feeds",
			Given:    "a router config with a low related threshold",
			When:     "routing feeds with similar vectors",
			Then:     "should compress related feeds into a single group entry",
			GivenDetail: givenDetail{
				config: &Config{
					Route: Route{
						GroupBy:                    []string{model.LabelSource, "category"},
						CompressByRelatedThreshold: ptr.To(float32(0.8)),
						Receivers:                  []string{"compress-receiver"},
					},
				},
				relatedScore: func(m *mock.Mock) {
					m.On("RelatedScore", testFeeds[0].Vectors, testFeeds[1].Vectors).Return(float32(0.9), nil)
					m.On("RelatedScore", mock.Anything, mock.Anything).Maybe().Return(float32(0.1), nil)
				},
			},
			WhenDetail: whenDetail{
				ruleResult: &rule.Result{
					Rule:  "CompressRule",
					Time:  now,
					Feeds: []*block.FeedVO{testFeeds[0], testFeeds[1], testFeeds[3]},
				},
			},
			ThenExpected: thenExpected{
				groups: []*Group{
					{
						FeedGroup: FeedGroup{
							Name: fmt.Sprintf("CompressRule  %s", model.Labels{
								{Key: model.LabelSource, Value: "TechCrunch"},
								{Key: "category", Value: "AI"},
							}.String()),
							Time: now,
							Labels: model.Labels{
								{Key: "category", Value: "AI"},
								{Key: model.LabelSource, Value: "TechCrunch"},
							},
							Feeds: []*Feed{
								{
									Feed:    testFeeds[0].Feed,
									Vectors: testFeeds[0].Vectors,
									Related: []*Feed{
										{Feed: testFeeds[1].Feed},
									},
								},
							},
						},
						Receivers: []string{"compress-receiver"},
					},
					{
						FeedGroup: FeedGroup{
							Name: fmt.Sprintf("CompressRule  %s", model.Labels{
								{Key: model.LabelSource, Value: "TechCrunch"},
								{Key: "category", Value: "Hardware"},
							}.String()),
							Time: now,
							Labels: model.Labels{
								{Key: "category", Value: "Hardware"},
								{Key: model.LabelSource, Value: "TechCrunch"},
							},
							Feeds: []*Feed{
								{Feed: testFeeds[3].Feed, Vectors: testFeeds[3].Vectors},
							},
						},
						Receivers: []string{"compress-receiver"},
					},
				},
				isErr: false,
			},
		},
		{
			Scenario: "Error during related score calculation",
			Given:    "a router config and RelatedScore dependency returns an error",
			When:     "routing feeds requiring related score check",
			Then:     "should return an error originating from RelatedScore",
			GivenDetail: givenDetail{
				config: &Config{
					Route: Route{
						GroupBy:                    []string{model.LabelSource},
						CompressByRelatedThreshold: ptr.To(float32(0.8)),
						Receivers:                  []string{"error-receiver"},
					},
				},
				relatedScore: func(m *mock.Mock) {
					m.On("RelatedScore", testFeeds[0].Vectors, testFeeds[1].Vectors).Return(float32(0), errors.New("related score calculation failed"))
					m.On("RelatedScore", mock.Anything, mock.Anything).Maybe().Return(float32(0.1), nil)
				},
			},
			WhenDetail: whenDetail{
				ruleResult: &rule.Result{
					Rule:  "ErrorRule",
					Time:  now,
					Feeds: []*block.FeedVO{testFeeds[0], testFeeds[1]},
				},
			},
			ThenExpected: thenExpected{
				groups: nil,
				isErr:  true,
				errMsg: "compress related feeds: compress related feeds: related score: related score calculation failed",
			},
		},
		{
			Scenario: "No feeds to route",
			Given:    "a standard router config",
			When:     "routing an empty list of feeds",
			Then:     "should return an empty list of groups without error",
			GivenDetail: givenDetail{
				config: &Config{
					Route: Route{
						GroupBy:                    []string{model.LabelSource},
						CompressByRelatedThreshold: ptr.To(float32(0.85)),
						Receivers:                  []string{"default-receiver"},
					},
				},
				relatedScore: func(m *mock.Mock) {
				},
			},
			WhenDetail: whenDetail{
				ruleResult: &rule.Result{
					Rule:  "EmptyRule",
					Time:  now,
					Feeds: []*block.FeedVO{},
				},
			},
			ThenExpected: thenExpected{
				groups: []*Group{},
				isErr:  false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			for _, group := range tt.ThenExpected.groups {
				group.Labels.EnsureSorted()
			}
			err := tt.GivenDetail.config.Validate()
			Expect(err).NotTo(HaveOccurred(), "Config validation failed during test setup")

			mockDep := mockDependencies{}
			if tt.GivenDetail.relatedScore != nil {
				tt.GivenDetail.relatedScore(&mockDep.Mock)
			}

			llmFactory, err := llm.NewFactory("", nil, llm.FactoryDependencies{}, component.MockOption(func(m *mock.Mock) {
				m.On("String", mock.Anything, mock.Anything).Return("test", nil)
			}))
			Expect(err).NotTo(HaveOccurred())

			routerInstance := &router{
				Base: component.New(&component.BaseConfig[Config, Dependencies]{
					Name:     "TestRouter",
					Instance: "test",
					Config:   tt.GivenDetail.config,
					Dependencies: Dependencies{
						RelatedScore: mockDep.RelatedScore,
						LLMFactory:   llmFactory,
					},
				}),
			}

			groups, err := routerInstance.Route(context.Background(), tt.WhenDetail.ruleResult)

			if tt.ThenExpected.isErr {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(tt.ThenExpected.errMsg))
				Expect(groups).To(BeNil())
			} else {
				Expect(err).NotTo(HaveOccurred())
				compareGroups(groups, tt.ThenExpected.groups)
			}

			mockDep.AssertExpectations(t)
		})
	}
}

type mockDependencies struct {
	mock.Mock
}

func (m *mockDependencies) RelatedScore(a, b [][]float32) (float32, error) {
	args := m.Called(a, b)
	return args.Get(0).(float32), args.Error(1)
}

func compareGroups(actual, expected []*Group) {
	Expect(actual).To(HaveLen(len(expected)), "Number of groups mismatch")

	for i := range expected {
		actualGroup := actual[i]
		expectedGroup := expected[i]

		Expect(actualGroup.Name).To(Equal(expectedGroup.Name), fmt.Sprintf("Group %d Name mismatch", i))
		Expect(timeutil.Format(actualGroup.Time)).To(Equal(timeutil.Format(expectedGroup.Time)), fmt.Sprintf("Group %d Time mismatch", i))
		Expect(actualGroup.Labels).To(Equal(expectedGroup.Labels), fmt.Sprintf("Group %d Labels mismatch", i))
		Expect(actualGroup.Receivers).To(Equal(expectedGroup.Receivers), fmt.Sprintf("Group %d Receivers mismatch", i))

		compareFeedsWithRelated(actualGroup.Feeds, expectedGroup.Feeds, i)
	}
}

func compareFeedsWithRelated(actual, expected []*Feed, groupIndex int) {
	Expect(actual).To(HaveLen(len(expected)), fmt.Sprintf("Group %d: Number of primary feeds mismatch", groupIndex))

	for i := range expected {
		actualFeed := actual[i]
		expectedFeed := expected[i]

		Expect(actualFeed.Feed).To(Equal(expectedFeed.Feed), fmt.Sprintf("Group %d, Feed %d: Primary feed mismatch", groupIndex, i))

		Expect(actualFeed.Related).To(HaveLen(len(expectedFeed.Related)), fmt.Sprintf("Group %d, Feed %d: Number of related feeds mismatch", groupIndex, i))
		for j := range expectedFeed.Related {
			Expect(actualFeed.Related[j].Feed).To(Equal(expectedFeed.Related[j].Feed), fmt.Sprintf("Group %d, Feed %d, Related %d: Related feed mismatch", groupIndex, i, j))
		}
	}
}

func TestConfig_Validate(t *testing.T) {
	RegisterTestingT(t)

	tests := []struct {
		name    string
		config  *Config
		wantErr bool
		errMsg  string
	}{
		{
			name: "Valid default config",
			config: &Config{
				Route: Route{
					Receivers: []string{"rec1"},
				},
			},
			wantErr: false,
		},
		{
			name: "Valid config with explicit defaults",
			config: &Config{
				Route: Route{
					GroupBy:                    []string{model.LabelSource},
					CompressByRelatedThreshold: ptr.To(float32(0.85)),
					Receivers:                  []string{"rec1"},
				},
			},
			wantErr: false,
		},
		{
			name: "Valid config with sub-route",
			config: &Config{
				Route: Route{
					Receivers: []string{"rec1"},
					SubRoutes: SubRoutes{
						{
							Route: Route{
								Receivers: []string{"rec2"},
							},
							Matchers: []string{"label=value"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid sub-route missing matchers",
			config: &Config{
				Route: Route{
					Receivers: []string{"rec1"},
					SubRoutes: SubRoutes{
						{
							Route: Route{
								Receivers: []string{"rec2"},
							},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid sub_route: matchers is required",
		},
		{
			name: "Invalid sub-route matcher format",
			config: &Config{
				Route: Route{
					Receivers: []string{"rec1"},
					SubRoutes: SubRoutes{
						{
							Route: Route{
								Receivers: []string{"rec2"},
							},
							Matchers: []string{"invalid-matcher"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid sub_route: invalid matcher: invalid matcher",
		},
		{
			name: "Valid nested sub-route",
			config: &Config{
				Route: Route{
					Receivers: []string{"rec1"},
					SubRoutes: SubRoutes{
						{
							Route: Route{
								Receivers: []string{"rec2"},
								SubRoutes: SubRoutes{
									{
										Route: Route{
											Receivers: []string{"rec3"},
										},
										Matchers: []string{"nested=true"},
									},
								},
							},
							Matchers: []string{"label=value"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "Invalid nested sub-route",
			config: &Config{
				Route: Route{
					Receivers: []string{"rec1"},
					SubRoutes: SubRoutes{
						{
							Route: Route{
								Receivers: []string{"rec2"},
								SubRoutes: SubRoutes{
									{
										Route: Route{
											Receivers: []string{"rec3"},
										},
									},
								},
							},
							Matchers: []string{"label=value"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid sub_route: invalid sub_route: matchers is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(tt.errMsg))
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(tt.config.GroupBy).NotTo(BeEmpty())
				Expect(tt.config.CompressByRelatedThreshold).NotTo(BeNil())
				for _, sr := range tt.config.SubRoutes {
					Expect(sr.GroupBy).NotTo(BeEmpty())
					Expect(sr.CompressByRelatedThreshold).NotTo(BeNil())
					for _, nestedSr := range sr.SubRoutes {
						Expect(nestedSr.GroupBy).NotTo(BeEmpty())
						Expect(nestedSr.CompressByRelatedThreshold).NotTo(BeNil())
					}
				}
			}
		})
	}
}

func TestSubRoutes_Match(t *testing.T) {
	RegisterTestingT(t)

	now := time.Now()
	feedAI := &block.FeedVO{
		Feed: &model.Feed{
			ID: 10,
			Labels: model.Labels{
				{Key: "category", Value: "AI"},
				{Key: model.LabelSource, Value: "TechCrunch"},
				{Key: model.LabelTitle, Value: "AI Feed"},
				{Key: model.LabelLink, Value: "http://example.com/ai"},
			},
			Time: now,
		},
	}
	feedHardware := &block.FeedVO{
		Feed: &model.Feed{
			ID: 11,
			Labels: model.Labels{
				{Key: "category", Value: "Hardware"},
				{Key: model.LabelSource, Value: "TechCrunch"},
				{Key: model.LabelTitle, Value: "Hardware Feed"},
				{Key: model.LabelLink, Value: "http://example.com/hw"},
			},
			Time: now,
		},
	}
	feedSports := &block.FeedVO{
		Feed: &model.Feed{
			ID: 12,
			Labels: model.Labels{
				{Key: "category", Value: "Sports"},
				{Key: model.LabelSource, Value: "OtherSource"},
				{Key: model.LabelTitle, Value: "Sports Feed"},
				{Key: model.LabelLink, Value: "http://example.com/sports"},
			},
			Time: now,
		},
	}
	feedNestedLow := &block.FeedVO{
		Feed: &model.Feed{
			ID: 13,
			Labels: model.Labels{
				{Key: "category", Value: "Nested"},
				{Key: "priority", Value: "low"},
				{Key: model.LabelTitle, Value: "Nested Low Prio"},
				{Key: model.LabelLink, Value: "http://example.com/nested_low"},
			},
			Time: now,
		},
	}
	feedNestedHigh := &block.FeedVO{
		Feed: &model.Feed{
			ID: 14,
			Labels: model.Labels{
				{Key: "category", Value: "Nested"},
				{Key: "priority", Value: "high"},
				{Key: model.LabelTitle, Value: "Nested High Prio"},
				{Key: model.LabelLink, Value: "http://example.com/nested_high"},
			},
			Time: now,
		},
	}

	feedAI.Labels.EnsureSorted()
	feedHardware.Labels.EnsureSorted()
	feedSports.Labels.EnsureSorted()
	feedNestedLow.Labels.EnsureSorted()
	feedNestedHigh.Labels.EnsureSorted()

	subRouteAI := &SubRoute{
		Route:    Route{Receivers: []string{"ai"}},
		Matchers: []string{"category=AI"},
	}
	err := subRouteAI.Validate()
	Expect(err).NotTo(HaveOccurred())

	subRouteHardware := &SubRoute{
		Route:    Route{Receivers: []string{"hardware"}},
		Matchers: []string{"category=Hardware"},
	}
	err = subRouteHardware.Validate()
	Expect(err).NotTo(HaveOccurred())

	subRouteTechCrunchNotAI := &SubRoute{
		Route:    Route{Receivers: []string{"tc-not-ai"}},
		Matchers: []string{model.LabelSource + "=TechCrunch", "category!=AI"},
	}
	err = subRouteTechCrunchNotAI.Validate()
	Expect(err).NotTo(HaveOccurred())

	subRouteNested := &SubRoute{
		Route: Route{
			Receivers: []string{"nested-route"},
			SubRoutes: SubRoutes{
				{
					Route:    Route{Receivers: []string{"deep-nested"}},
					Matchers: []string{"priority=high"},
				},
			},
		},
		Matchers: []string{"category=Nested"},
	}
	err = subRouteNested.Validate()
	Expect(err).NotTo(HaveOccurred())
	nestedDeepSubRoute := subRouteNested.SubRoutes[0]
	err = nestedDeepSubRoute.Validate()
	Expect(err).NotTo(HaveOccurred())

	routes := SubRoutes{subRouteAI, subRouteHardware, subRouteTechCrunchNotAI, subRouteNested}

	tests := []struct {
		name          string
		feed          *block.FeedVO
		expectedRoute *SubRoute
	}{
		{
			name:          "Match AI category",
			feed:          feedAI,
			expectedRoute: subRouteAI,
		},
		{
			name:          "Match Hardware category",
			feed:          feedHardware,
			expectedRoute: subRouteHardware,
		},
		{
			name:          "Match TechCrunch but not AI",
			feed:          feedHardware,
			expectedRoute: subRouteHardware,
		},
		{
			name:          "No matching category",
			feed:          feedSports,
			expectedRoute: nil,
		},
		{
			name:          "Match nested route (top level)",
			feed:          feedNestedLow,
			expectedRoute: subRouteNested,
		},
		{
			name:          "Match nested route (deep level)",
			feed:          feedNestedHigh,
			expectedRoute: nestedDeepSubRoute,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			matchedRoute := routes.Match(tt.feed)
			if tt.expectedRoute == nil {
				Expect(matchedRoute).To(BeNil())
			} else {
				Expect(matchedRoute).NotTo(BeNil())
				Expect(matchedRoute.Receivers).To(Equal(tt.expectedRoute.Receivers))
				Expect(matchedRoute.Matchers).To(Equal(tt.expectedRoute.Matchers))
			}
		})
	}
}
