package rule

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/stretchr/testify/mock"

	"github.com/vandeefeng/zenfeed/pkg/component"
	"github.com/vandeefeng/zenfeed/pkg/model"
	"github.com/vandeefeng/zenfeed/pkg/storage/feed"
	"github.com/vandeefeng/zenfeed/pkg/storage/feed/block"
	"github.com/vandeefeng/zenfeed/pkg/test"
)

func TestPeriodicExecute(t *testing.T) {
	RegisterTestingT(t)

	// --- Test types ---
	type givenDetail struct {
		config          *Config
		feedStorageMock func(m *mock.Mock) // Function to set expectations
	}
	type whenDetail struct {
		now time.Time
	}
	type thenExpected struct {
		queryCalled bool
		queryOpts   *block.QueryOptions // Only check relevant fields like start/end
		sentToOut   *Result
		err         error // Expected error (can be wrapped)
		isErr       bool
	}

	// --- Test cases ---
	mockFeeds := []*block.FeedVO{
		{Feed: &model.Feed{ID: 1, Labels: model.Labels{{Key: "content_hash", Value: "a"}}}},
		{Feed: &model.Feed{ID: 2, Labels: model.Labels{{Key: "content_hash", Value: "b"}}}},
	}
	baseConfig := &Config{
		Name:      "test-periodic",
		EveryDay:  "09:00~18:00", // Will be parsed in Validate
		Threshold: 0.7,
		Query:     "test query",
	}
	// Manually parse time for expected values
	startTime, _ := time.ParseInLocation(timeFmt, "09:00", time.Local)
	endTime, _ := time.ParseInLocation(timeFmt, "18:00", time.Local)

	crossDayConfig := &Config{
		Name:      "test-crossday",
		EveryDay:  "-22:00~06:00", // Will be parsed in Validate
		Threshold: 0.7,
		Query:     "test query",
	}
	// Manually parse time for expected values
	crossStartTime, _ := time.ParseInLocation(timeFmt, "22:00", time.Local)
	crossEndTime, _ := time.ParseInLocation(timeFmt, "06:00", time.Local)

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Non-crossDay, feeds found, should query and notify",
			Given:    "a non-crossDay config and FeedStorage returns feeds",
			When:     "execute is called within the configured day",
			Then:     "FeedStorage should be queried with the correct daily time range and result sent to Out",
			GivenDetail: givenDetail{
				config: baseConfig,
				feedStorageMock: func(m *mock.Mock) {
					m.On("Query", mock.Anything, mock.AnythingOfType("block.QueryOptions")).
						Return(mockFeeds, nil)
				},
			},
			WhenDetail: whenDetail{
				now: time.Date(2024, 1, 15, 10, 0, 0, 0, time.Local), // 10:00 AM
			},
			ThenExpected: thenExpected{
				queryCalled: true,
				queryOpts: &block.QueryOptions{
					Start: time.Date(2024, 1, 15, startTime.Hour(), startTime.Minute(), 0, 0, time.Local),
					End:   time.Date(2024, 1, 15, endTime.Hour(), endTime.Minute(), 0, 0, time.Local),
					Query: baseConfig.Query,
					Limit: 500,
				},
				sentToOut: &Result{
					Rule:  baseConfig.Name,
					Time:  time.Date(2024, 1, 15, startTime.Hour(), startTime.Minute(), 0, 0, time.Local),
					Feeds: mockFeeds,
				},
			},
		},
		{
			Scenario: "CrossDay, feeds found, should query and notify",
			Given:    "a crossDay config and FeedStorage returns feeds",
			When:     "execute is called within the configured day",
			Then:     "FeedStorage should be queried with the correct cross-day time range and result sent to Out",
			GivenDetail: givenDetail{
				config: crossDayConfig,
				feedStorageMock: func(m *mock.Mock) {
					m.On("Query", mock.Anything, mock.AnythingOfType("block.QueryOptions")).
						Return(mockFeeds, nil)
				},
			},
			WhenDetail: whenDetail{
				now: time.Date(2024, 1, 15, 03, 0, 0, 0, time.Local), // 03:00 AM
			},
			ThenExpected: thenExpected{
				queryCalled: true,
				queryOpts: &block.QueryOptions{
					Start: time.Date(2024, 1, 14, crossStartTime.Hour(), crossStartTime.Minute(), 0, 0, time.Local),
					End:   time.Date(2024, 1, 15, crossEndTime.Hour(), crossEndTime.Minute(), 0, 0, time.Local),
					Query: crossDayConfig.Query,
					Limit: 500,
				},
				sentToOut: &Result{
					Rule:  crossDayConfig.Name,
					Time:  time.Date(2024, 1, 14, crossStartTime.Hour(), crossStartTime.Minute(), 0, 0, time.Local),
					Feeds: mockFeeds,
				},
			},
		},
		{
			Scenario: "Non-crossDay, no feeds found, should query but not notify",
			Given:    "a non-crossDay config and FeedStorage returns no feeds",
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
				now: time.Date(2024, 1, 15, 11, 0, 0, 0, time.Local), // 11:00 AM
			},
			ThenExpected: thenExpected{
				queryCalled: true,
				queryOpts: &block.QueryOptions{
					Start: time.Date(2024, 1, 15, startTime.Hour(), startTime.Minute(), 0, 0, time.Local),
					End:   time.Date(2024, 1, 15, endTime.Hour(), endTime.Minute(), 0, 0, time.Local),
					Query: baseConfig.Query,
					Limit: 500,
				},
				sentToOut: nil,
			},
		},
	}

	// --- Run tests ---
	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// --- Given ---
			configCopy := *tt.GivenDetail.config
			err := configCopy.Validate()
			Expect(err).NotTo(HaveOccurred(), "Config validation failed in test setup")

			outCh := make(chan *Result, 1)
			var capturedOpts block.QueryOptions
			var mockStorageInstance *mock.Mock

			// Create mock factory using feed.NewFactory and capture the mock instance
			mockOption := component.MockOption(func(m *mock.Mock) {
				mockStorageInstance = m // Capture the mock instance
				// Setup mock expectation for FeedStorage.Query, including option capture
				if tt.GivenDetail.feedStorageMock != nil {
					tt.GivenDetail.feedStorageMock(m)
					// Enhance the mock setup to capture arguments if the mock function exists
					// Find the Query expectation and add argument capture logic
					for _, call := range m.ExpectedCalls {
						if call.Method == "Query" {
							// Replace the generic matcher for options with one that captures
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
			mockFeedFactory := feed.NewFactory(mockOption)
			mockFeedStorage, factoryErr := mockFeedFactory.New(component.Global, nil, feed.Dependencies{}) // Use factory to create mock
			Expect(factoryErr).NotTo(HaveOccurred())

			dependencies := Dependencies{
				FeedStorage: mockFeedStorage, // Use the created mock storage
				Out:         outCh,
			}

			r := &periodic{
				Base: component.New(&component.BaseConfig[Config, Dependencies]{
					Name:         "PeriodicRuler",
					Instance:     "test-instance",
					Config:       &configCopy,
					Dependencies: dependencies,
				}),
			}

			// --- When ---
			err = r.execute(context.Background(), tt.WhenDetail.now)

			// --- Then ---
			if tt.ThenExpected.isErr {
				Expect(err).To(HaveOccurred())
				// Use MatchError for potentially wrapped errors, providing a more precise check
				Expect(err).To(MatchError(tt.ThenExpected.err))
				Expect(len(outCh)).To(Equal(0))
			} else {
				Expect(err).NotTo(HaveOccurred())
				if tt.ThenExpected.sentToOut != nil {
					Expect(len(outCh)).To(Equal(1))
					receivedResult := <-outCh
					Expect(receivedResult.Rule).To(Equal(tt.ThenExpected.sentToOut.Rule))
					Expect(receivedResult.Time.Unix()).To(Equal(tt.ThenExpected.sentToOut.Time.Unix()))
					Expect(receivedResult.Feeds).To(Equal(tt.ThenExpected.sentToOut.Feeds))
				} else {
					Expect(len(outCh)).To(Equal(0))
				}
			}

			// Verify FeedStorage.Query call and options using the captured mock instance
			if mockStorageInstance != nil { // Ensure mock instance was captured
				if tt.ThenExpected.queryCalled {
					// Assert the expectation set up in feedStorageMockFn was met
					mockStorageInstance.AssertCalled(t, "Query", mock.Anything, mock.AnythingOfType("block.QueryOptions"))
					// Assert specific fields of the captured options
					Expect(capturedOpts.Start.Unix()).To(Equal(tt.ThenExpected.queryOpts.Start.Unix()), "Start time mismatch")
					Expect(capturedOpts.End.Unix()).To(Equal(tt.ThenExpected.queryOpts.End.Unix()), "End time mismatch")
					Expect(capturedOpts.Query).To(Equal(tt.ThenExpected.queryOpts.Query), "Query string mismatch")
					Expect(capturedOpts.Threshold).To(Equal(configCopy.Threshold), "Threshold mismatch")
					Expect(capturedOpts.LabelFilters).To(Equal(configCopy.LabelFilters), "LabelFilters mismatch")
					Expect(capturedOpts.Limit).To(Equal(tt.ThenExpected.queryOpts.Limit), "Limit mismatch")
				} else {
					mockStorageInstance.AssertNotCalled(t, "Query", mock.Anything, mock.Anything)
				}
				// Optionally, assert all expectations are met
				// mockStorageInstance.AssertExpectations(t) // Uncomment if you want strict expectation matching
			} else if tt.ThenExpected.queryCalled {
				// Fail if query was expected but mock instance wasn't captured (indicates setup issue)
				t.Fatal("Expected query call but mock instance was not captured")
			}

			close(outCh)
		})
	}
}
