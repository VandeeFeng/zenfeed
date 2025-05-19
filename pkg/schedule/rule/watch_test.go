package rule

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"github.com/stretchr/testify/mock"

	"github.com/vandeefeng/zenfeed/pkg/component"
	"github.com/vandeefeng/zenfeed/pkg/model"
	"github.com/vandeefeng/zenfeed/pkg/storage/feed"
	"github.com/vandeefeng/zenfeed/pkg/storage/feed/block"
	"github.com/vandeefeng/zenfeed/pkg/test"
)

func TestWatchExecute(t *testing.T) {
	RegisterTestingT(t)

	// --- Test types ---
	type givenDetail struct {
		config          *Config
		feedStorageMock func(m *mock.Mock) // Function to set expectations
	}
	type whenDetail struct {
		start time.Time
		end   time.Time
	}
	type thenExpected struct {
		queryCalled bool
		queryOpts   *block.QueryOptions   // Expected query options
		sentToOut   map[time.Time]*Result // Expected results sent to Out, keyed by interval start time
		err         error                 // Expected error (can be wrapped)
		isErr       bool
	}

	// --- Test cases ---
	watchInterval := 10 * time.Minute
	baseConfig := &Config{
		Name:          "test-watch",
		WatchInterval: watchInterval,
		Threshold:     0.7,
		Query:         "test query",
		LabelFilters:  []string{"source:test"},
	}
	now := time.Date(2024, 1, 15, 10, 35, 0, 0, time.Local) // Example time: 10:35
	// The execute function calculates start/end based on its input 'end' time and interval
	// Let's define the input range for the 'execute' call
	execEnd := now
	execStart := execEnd.Add(-3 * watchInterval) // Matches the logic in watch.go iter()

	// Define feed times relative to the interval
	interval1Start := time.Unix(now.Unix(), 0).Truncate(watchInterval) // 10:30
	interval2Start := interval1Start.Add(-watchInterval)               // 10:20
	// interval3Start := interval2Start.Add(-watchInterval)                // 10:10, covered by execStart

	feedTime1 := interval1Start.Add(1 * time.Minute) // 10:31 (belongs to 10:30 interval)
	feedTime2 := interval2Start.Add(2 * time.Minute) // 10:22 (belongs to 10:20 interval)
	feedTime3 := interval2Start.Add(5 * time.Minute) // 10:25 (belongs to 10:20 interval)

	mockFeeds := []*block.FeedVO{
		{Feed: &model.Feed{ID: 1, Time: feedTime1, Labels: model.Labels{{Key: "content_hash", Value: "a"}}}},
		{Feed: &model.Feed{ID: 2, Time: feedTime2, Labels: model.Labels{{Key: "content_hash", Value: "b"}}}},
		{Feed: &model.Feed{ID: 3, Time: feedTime3, Labels: model.Labels{{Key: "content_hash", Value: "c"}}}},
	}
	queryError := errors.New("database error")

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Feeds found, should query and notify grouped by interval",
			Given:    "a watch config and FeedStorage returns feeds across intervals",
			When:     "execute is called with a time range",
			Then:     "FeedStorage should be queried, and results grouped by WatchInterval sent to Out",
			GivenDetail: givenDetail{
				config: baseConfig,
				feedStorageMock: func(m *mock.Mock) {
					m.On("Query", mock.Anything, mock.AnythingOfType("block.QueryOptions")).
						Return(mockFeeds, nil)
				},
			},
			WhenDetail: whenDetail{
				start: execStart,
				end:   execEnd,
			},
			ThenExpected: thenExpected{
				queryCalled: true,
				queryOpts: &block.QueryOptions{
					Query:        baseConfig.Query,
					Threshold:    baseConfig.Threshold,
					LabelFilters: baseConfig.LabelFilters,
					Start:        execStart,
					End:          execEnd,
					Limit:        500,
				},
				sentToOut: map[time.Time]*Result{
					interval1Start: { // 10:30 interval
						Rule: baseConfig.Name,
						Time: interval1Start,
						Feeds: []*block.FeedVO{
							mockFeeds[0], // ID 1 at 10:31
						},
					},
					interval2Start: { // 10:20 interval
						Rule: baseConfig.Name,
						Time: interval2Start,
						Feeds: []*block.FeedVO{
							mockFeeds[1], // ID 2 at 10:22
							mockFeeds[2], // ID 3 at 10:25
						},
					},
				},
			},
		},
		{
			Scenario: "No feeds found, should query but not notify",
			Given:    "a watch config and FeedStorage returns no feeds",
			When:     "execute is called",
			Then:     "FeedStorage should be queried but nothing sent to Out",
			GivenDetail: givenDetail{
				config: baseConfig,
				feedStorageMock: func(m *mock.Mock) {
					m.On("Query", mock.Anything, mock.AnythingOfType("block.QueryOptions")).
						Return([]*block.FeedVO{}, nil) // Empty result
				},
			},
			WhenDetail: whenDetail{
				start: execStart,
				end:   execEnd,
			},
			ThenExpected: thenExpected{
				queryCalled: true,
				queryOpts: &block.QueryOptions{
					Query:        baseConfig.Query,
					Threshold:    baseConfig.Threshold,
					LabelFilters: baseConfig.LabelFilters,
					Start:        execStart,
					End:          execEnd,
					Limit:        500,
				},
				sentToOut: map[time.Time]*Result{}, // Expect empty map or nil
			},
		},
		{
			Scenario: "FeedStorage query error, should return error",
			Given:    "a watch config and FeedStorage returns an error",
			When:     "execute is called",
			Then:     "FeedStorage should be queried and an error returned",
			GivenDetail: givenDetail{
				config: baseConfig,
				feedStorageMock: func(m *mock.Mock) {
					m.On("Query", mock.Anything, mock.AnythingOfType("block.QueryOptions")).
						Return([]*block.FeedVO{}, queryError)
				},
			},
			WhenDetail: whenDetail{
				start: execStart,
				end:   execEnd,
			},
			ThenExpected: thenExpected{
				queryCalled: true,
				queryOpts: &block.QueryOptions{ // Still expect query options to be set
					Query:        baseConfig.Query,
					Threshold:    baseConfig.Threshold,
					LabelFilters: baseConfig.LabelFilters,
					Start:        execStart,
					End:          execEnd,
					Limit:        500,
				},
				sentToOut: nil, // Nothing sent on error
				err:       errors.Wrap(queryError, "query"),
				isErr:     true,
			},
		},
	}

	// --- Run tests ---
	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// --- Given ---
			configCopy := *tt.GivenDetail.config // Use a copy for safety

			outCh := make(chan *Result, 5) // Buffer size accommodates potential multiple sends
			var capturedOpts block.QueryOptions
			var mockStorageInstance *mock.Mock

			// Create mock factory using feed.NewFactory and capture the mock instance
			mockOption := component.MockOption(func(m *mock.Mock) {
				mockStorageInstance = m // Capture the mock instance
				// Setup mock expectation for FeedStorage.Query, including option capture
				if tt.GivenDetail.feedStorageMock != nil {
					tt.GivenDetail.feedStorageMock(m)
					// Enhance the mock setup to capture arguments
					for _, call := range m.ExpectedCalls {
						if call.Method == "Query" {
							for i, arg := range call.Arguments {
								if _, ok := arg.(mock.AnythingOfTypeArgument); ok && i == 1 { // Assuming options is the second argument (index 1)
									call.Arguments[i] = mock.MatchedBy(func(opts block.QueryOptions) bool {
										capturedOpts = opts // Capture the options
										return true
									})
									break
								}
							}
							break // Assume only one Query expectation per test case here
						}
					}
				}
			})
			// NOTE: feed.NewFactory needs *config.App, we pass nil as it's not used by the mock
			mockFeedFactory := feed.NewFactory(mockOption)
			mockFeedStorage, factoryErr := mockFeedFactory.New(component.Global, nil, feed.Dependencies{}) // Use factory to create mock
			Expect(factoryErr).NotTo(HaveOccurred())

			dependencies := Dependencies{
				FeedStorage: mockFeedStorage, // Use the created mock storage
				Out:         outCh,
			}

			// Use the specific type `watch` for testing its method
			r := &watch{
				Base: component.New(&component.BaseConfig[Config, Dependencies]{
					Name:         "WatchRuler",
					Instance:     "test-instance",
					Config:       &configCopy,
					Dependencies: dependencies,
				}),
			}

			// --- When ---
			err := r.execute(context.Background(), tt.WhenDetail.start, tt.WhenDetail.end)

			// --- Then ---
			close(outCh) // Close channel to range over received results

			if tt.ThenExpected.isErr {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(tt.ThenExpected.err.Error())) // Check if the error contains the expected wrapped message
				Expect(len(outCh)).To(Equal(0))                                       // No results sent on error
			} else {
				Expect(err).NotTo(HaveOccurred())

				receivedResults := make(map[time.Time]*Result)
				for res := range outCh {
					receivedResults[res.Time] = res
				}

				Expect(len(receivedResults)).To(Equal(len(tt.ThenExpected.sentToOut)), "Mismatch in number of results sent")
				for expectedTime, expectedResult := range tt.ThenExpected.sentToOut {
					receivedResult, ok := receivedResults[expectedTime]
					Expect(ok).To(BeTrue(), "Expected result for time %v not found", expectedTime)
					Expect(receivedResult.Rule).To(Equal(expectedResult.Rule))
					Expect(receivedResult.Time.Unix()).To(Equal(expectedResult.Time.Unix()))
					Expect(receivedResult.Feeds).To(ConsistOf(expectedResult.Feeds)) // Use ConsistOf for order-independent comparison
				}
			}

			// Verify FeedStorage.Query call and options using the captured mock instance
			if mockStorageInstance != nil {
				if tt.ThenExpected.queryCalled {
					mockStorageInstance.AssertCalled(t, "Query", mock.Anything, mock.AnythingOfType("block.QueryOptions"))
					// Assert specific fields of the captured options
					Expect(capturedOpts.Query).To(Equal(tt.ThenExpected.queryOpts.Query), "Query string mismatch")
					Expect(capturedOpts.Threshold).To(Equal(tt.ThenExpected.queryOpts.Threshold), "Threshold mismatch")
					Expect(capturedOpts.LabelFilters).To(Equal(tt.ThenExpected.queryOpts.LabelFilters), "LabelFilters mismatch")
					Expect(capturedOpts.Start.Unix()).To(Equal(tt.ThenExpected.queryOpts.Start.Unix()), "Start time mismatch")
					Expect(capturedOpts.End.Unix()).To(Equal(tt.ThenExpected.queryOpts.End.Unix()), "End time mismatch")
					Expect(capturedOpts.Limit).To(Equal(tt.ThenExpected.queryOpts.Limit), "Limit mismatch")
				} else {
					mockStorageInstance.AssertNotCalled(t, "Query", mock.Anything, mock.Anything)
				}
				// mockStorageInstance.AssertExpectations(t) // Uncomment for strict expectation matching if needed
			} else if tt.ThenExpected.queryCalled {
				t.Fatal("Expected query call but mock instance was not captured")
			}
		})
	}
}
