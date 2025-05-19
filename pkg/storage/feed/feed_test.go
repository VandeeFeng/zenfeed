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

// TODO: fix tests
package feed

// import (
// 	"context"
// 	"os"
// 	"testing"
// 	"time"
//

// 	"github.com/benbjohnson/clock"
// 	. "github.com/onsi/gomega"
// 	"github.com/stretchr/testify/mock"

// 	"github.com/vandeefeng/zenfeed/pkg/config"
// 	"github.com/vandeefeng/zenfeed/pkg/storage/feed/block"
// 	"github.com/vandeefeng/zenfeed/pkg/storage/feed/block/chunk"
// 	"github.com/vandeefeng/zenfeed/pkg/test"
// 	timeutil "github.com/vandeefeng/zenfeed/pkg/util/time"
// )

// func TestNew(t *testing.T) {
// 	RegisterTestingT(t)

// 	type givenDetail struct {
// 		now          time.Time
// 		blocksOnDisk []string // Block directory names in format "2006-01-02T15:04:05Z-2006-01-02T15:04:05Z"
// 	}
// 	type whenDetail struct {
// 		app *config.App
// 	}
// 	type thenExpected struct {
// 		storage        storage
// 		storageHotLen  int
// 		storageColdLen int
// 		blockCalls     []func(obj *mock.Mock)
// 	}
// 	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
// 		{
// 			Scenario: "Create a new storage from an empty directory",
// 			Given:    "just mock a time",
// 			When:     "call New with a config with a data directory",
// 			Then:     "should return a new storage and a hot block created",
// 			GivenDetail: givenDetail{
// 				now: timeutil.MustParse("2025-03-03T10:00:00Z"),
// 			},
// 			WhenDetail: whenDetail{
// 				app: &config.App{
// 					DB: config.DB{
// 						Dir: "/tmp/TestNew",
// 					},
// 				},
// 			},
// 			ThenExpected: thenExpected{
// 				storage: storage{
// 					config: &Config{
// 						Dir: "/tmp/TestNew",
// 					},
// 				},
// 				storageHotLen:  1,
// 				storageColdLen: 0,
// 			},
// 		},
// 		{
// 			Scenario: "Create a storage from existing directory with blocks",
// 			Given:    "existing blocks on disk",
// 			GivenDetail: givenDetail{
// 				now: timeutil.MustParse("2025-03-03T10:00:00Z"),
// 				blocksOnDisk: []string{
// 					"2025-03-02T10:00:00Z ~ 2025-03-03T10:00:00Z", // Hot block
// 					"2025-03-01T10:00:00Z ~ 2025-03-02T10:00:00Z", // Cold block
// 					"2025-02-28T10:00:00Z ~ 2025-03-01T10:00:00Z", // Cold block
// 				},
// 			},
// 			When: "call New with a config with existing data directory",
// 			WhenDetail: whenDetail{
// 				app: &config.App{
// 					DB: config.DB{
// 						Dir:             "/tmp/TestNew",
// 						WriteableWindow: 49 * time.Hour,
// 					},
// 				},
// 			},
// 			Then: "should return a storage with existing blocks loaded",
// 			ThenExpected: thenExpected{
// 				storage: storage{
// 					config: &Config{
// 						Dir: "/tmp/TestNew",
// 						Block: BlockConfig{
// 							WriteableWindow: 49 * time.Hour,
// 						},
// 					},
// 				},
// 				storageHotLen:  1,
// 				storageColdLen: 2,
// 				blockCalls: []func(obj *mock.Mock){
// 					func(m *mock.Mock) {
// 						m.On("State").Return(block.StateHot).Once()
// 					},
// 					func(m *mock.Mock) {
// 						m.On("State").Return(block.StateCold).Once()
// 					},
// 					func(m *mock.Mock) {
// 						m.On("State").Return(block.StateCold).Once()
// 					},
// 				},
// 			},
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.Scenario, func(t *testing.T) {
// 			// Given.
// 			c := clock.NewMock()
// 			c.Set(tt.GivenDetail.now)
// 			clk = c // Set global clock.
// 			defer func() { clk = clock.New() }()

// 			// Create test directories if needed
// 			if len(tt.GivenDetail.blocksOnDisk) > 0 {
// 				for _, blockDir := range tt.GivenDetail.blocksOnDisk {
// 					err := os.MkdirAll(tt.WhenDetail.app.DB.Dir+"/"+blockDir, 0755)
// 					Expect(err).To(BeNil())
// 				}
// 			}

// 			// When.
// 			var calls int
// 			var blockCalls []*mock.Mock
// 			blockFactory := block.NewFactory(func(obj *mock.Mock) {
// 				if calls < len(tt.ThenExpected.blockCalls) {
// 					tt.ThenExpected.blockCalls[calls](obj)
// 					calls++
// 					blockCalls = append(blockCalls, obj)
// 				}
// 			})
// 			s, err := new(tt.WhenDetail.app, blockFactory)
// 			defer os.RemoveAll(tt.WhenDetail.app.DB.Dir)

// 			// Then.
// 			Expect(err).To(BeNil())
// 			Expect(s).NotTo(BeNil())
// 			storage := s.(*storage)
// 			Expect(storage.config).To(Equal(tt.ThenExpected.storage.config))
// 			Expect(len(storage.hot.blocks)).To(Equal(tt.ThenExpected.storageHotLen))
// 			Expect(len(storage.cold.blocks)).To(Equal(tt.ThenExpected.storageColdLen))
// 			for _, call := range blockCalls {
// 				call.AssertExpectations(t)
// 			}
// 		})
// 	}
// }

// func TestAppend(t *testing.T) {
// 	RegisterTestingT(t)

// 	type givenDetail struct {
// 		hotBlocks  []func(m *mock.Mock)
// 		coldBlocks []func(m *mock.Mock)
// 	}
// 	type whenDetail struct {
// 		feeds []*chunk.Feed
// 	}
// 	type thenExpected struct {
// 		err string
// 	}

// 	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
// 		{
// 			Scenario: "Append feeds to hot block",
// 			Given:    "a storage with one hot block",
// 			When:     "append feeds within hot block time range",
// 			Then:     "should append feeds to hot block successfully",
// 			GivenDetail: givenDetail{
// 				hotBlocks: []func(m *mock.Mock){
// 					func(m *mock.Mock) {
// 						m.On("Start").Return(timeutil.MustParse("2025-03-02T10:00:00Z")).Twice()
// 						m.On("End").Return(timeutil.MustParse("2025-03-03T10:00:00Z")).Twice()
// 						m.On("State").Return(block.StateHot).Twice()
// 						m.On("Append", mock.Anything, []*chunk.Feed{
// 							{ID: 1, Time: timeutil.MustParse("2025-03-02T11:00:00Z")},
// 							{ID: 2, Time: timeutil.MustParse("2025-03-02T12:00:00Z")},
// 						}).Return(nil)
// 					},
// 				},
// 			},
// 			WhenDetail: whenDetail{
// 				feeds: []*chunk.Feed{
// 					{ID: 1, Time: timeutil.MustParse("2025-03-02T11:00:00Z")},
// 					{ID: 2, Time: timeutil.MustParse("2025-03-02T12:00:00Z")},
// 				},
// 			},
// 			ThenExpected: thenExpected{
// 				err: "",
// 			},
// 		},
// 		{
// 			Scenario: "Append feeds to non-hot block",
// 			Given:    "a storage with hot and cold blocks",
// 			When:     "append feeds with time in cold block range",
// 			Then:     "should return error",
// 			GivenDetail: givenDetail{
// 				coldBlocks: []func(m *mock.Mock){
// 					func(m *mock.Mock) {},
// 				},
// 			},
// 			WhenDetail: whenDetail{
// 				feeds: []*chunk.Feed{
// 					{ID: 1, Time: timeutil.MustParse("2025-03-01T11:00:00Z")},
// 				},
// 			},
// 			ThenExpected: thenExpected{
// 				err: "cannot find hot block",
// 			},
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.Scenario, func(t *testing.T) {
// 			// Given.
// 			calls := 0
// 			var blockMocks []*mock.Mock
// 			blockFactory := block.NewFactory(func(obj *mock.Mock) {
// 				if calls < len(tt.GivenDetail.hotBlocks) {
// 					tt.GivenDetail.hotBlocks[calls](obj)
// 					calls++
// 					blockMocks = append(blockMocks, obj)
// 				}
// 			})
// 			var hotBlocks blockChain
// 			for range tt.GivenDetail.hotBlocks {
// 				block, err := blockFactory.New(nil, nil, nil, nil, nil)
// 				Expect(err).To(BeNil())
// 				hotBlocks.add(block)
// 			}
// 			blockFactory = block.NewFactory(func(obj *mock.Mock) {
// 				if calls < len(tt.GivenDetail.coldBlocks) {
// 					tt.GivenDetail.coldBlocks[calls](obj)
// 					calls++
// 					blockMocks = append(blockMocks, obj)
// 				}
// 			})
// 			var coldBlocks blockChain
// 			for range tt.GivenDetail.coldBlocks {
// 				block, err := blockFactory.New(nil, nil, nil, nil, nil)
// 				Expect(err).To(BeNil())
// 				coldBlocks.add(block)
// 			}
// 			s := storage{
// 				hot:  &hotBlocks,
// 				cold: &coldBlocks,
// 			}

// 			// When.
// 			err := s.Append(context.Background(), tt.WhenDetail.feeds...)

// 			// Then.
// 			if tt.ThenExpected.err != "" {
// 				Expect(err.Error()).To(ContainSubstring(tt.ThenExpected.err))
// 			} else {
// 				Expect(err).To(BeNil())
// 			}
// 			for _, m := range blockMocks {
// 				m.AssertExpectations(t)
// 			}
// 		})
// 	}
// }

// func TestQuery(t *testing.T) {
// 	RegisterTestingT(t)

// 	type givenDetail struct {
// 		hotBlocks  []func(m *mock.Mock)
// 		coldBlocks []func(m *mock.Mock)
// 	}
// 	type whenDetail struct {
// 		query block.QueryOptions
// 	}
// 	type thenExpected struct {
// 		feeds []*block.FeedVO
// 		err   string
// 	}

// 	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
// 		{
// 			Scenario: "Query feeds from hot blocks",
// 			Given:    "a storage with one hot block containing feeds",
// 			When:     "querying with time range within hot block",
// 			Then:     "should return matching feeds from hot block",
// 			GivenDetail: givenDetail{
// 				hotBlocks: []func(m *mock.Mock){
// 					func(m *mock.Mock) {
// 						m.On("Start").Return(timeutil.MustParse("2025-03-02T10:00:00Z")).Once()
// 						m.On("End").Return(timeutil.MustParse("2025-03-03T10:00:00Z")).Once()
// 						m.On("Query", mock.Anything, mock.MatchedBy(func(q block.QueryOptions) bool {
// 							return q.Start.Equal(timeutil.MustParse("2025-03-02T12:00:00Z")) &&
// 								q.End.Equal(timeutil.MustParse("2025-03-02T14:00:00Z"))
// 						})).Return([]*block.FeedVO{
// 							{ID: 1, Time: timeutil.MustParse("2025-03-02T12:30:00Z")},
// 							{ID: 2, Time: timeutil.MustParse("2025-03-02T13:00:00Z")},
// 						}, nil)
// 					},
// 				},
// 			},
// 			WhenDetail: whenDetail{
// 				query: block.QueryOptions{
// 					Start: timeutil.MustParse("2025-03-02T12:00:00Z"),
// 					End:   timeutil.MustParse("2025-03-02T14:00:00Z"),
// 					Limit: 10,
// 				},
// 			},
// 			ThenExpected: thenExpected{
// 				feeds: []*block.FeedVO{
// 					{ID: 2, Time: timeutil.MustParse("2025-03-02T13:00:00Z")},
// 					{ID: 1, Time: timeutil.MustParse("2025-03-02T12:30:00Z")},
// 				},
// 				err: "",
// 			},
// 		},
// 		{
// 			Scenario: "Query feeds from multiple blocks",
// 			Given:    "a storage with hot and cold blocks containing feeds",
// 			When:     "querying with time range spanning multiple blocks",
// 			Then:     "should return combined and sorted feeds from all matching blocks",
// 			GivenDetail: givenDetail{
// 				hotBlocks: []func(m *mock.Mock){
// 					func(m *mock.Mock) {
// 						m.On("Start").Return(timeutil.MustParse("2025-03-02T10:00:00Z"))
// 						m.On("End").Return(timeutil.MustParse("2025-03-03T10:00:00Z"))
// 						m.On("Query", mock.Anything, mock.MatchedBy(func(q block.QueryOptions) bool {
// 							return !q.Start.IsZero() && q.End.IsZero()
// 						})).Return([]*block.FeedVO{
// 							{ID: 3, Time: timeutil.MustParse("2025-03-02T15:00:00Z")},
// 							{ID: 4, Time: timeutil.MustParse("2025-03-02T16:00:00Z")},
// 						}, nil)
// 					},
// 				},
// 				coldBlocks: []func(m *mock.Mock){
// 					func(m *mock.Mock) {
// 						m.On("Start").Return(timeutil.MustParse("2025-03-01T10:00:00Z"))
// 						m.On("End").Return(timeutil.MustParse("2025-03-02T10:00:00Z"))
// 						m.On("Query", mock.Anything, mock.MatchedBy(func(q block.QueryOptions) bool {
// 							return !q.Start.IsZero() && q.End.IsZero()
// 						})).Return([]*block.FeedVO{
// 							{ID: 1, Time: timeutil.MustParse("2025-03-01T15:00:00Z")},
// 							{ID: 2, Time: timeutil.MustParse("2025-03-01T16:00:00Z")},
// 						}, nil)
// 					},
// 				},
// 			},
// 			WhenDetail: whenDetail{
// 				query: block.QueryOptions{
// 					Start: timeutil.MustParse("2025-03-01T12:00:00Z"),
// 					Limit: 3,
// 				},
// 			},
// 			ThenExpected: thenExpected{
// 				feeds: []*block.FeedVO{
// 					{ID: 4, Time: timeutil.MustParse("2025-03-02T16:00:00Z")},
// 					{ID: 3, Time: timeutil.MustParse("2025-03-02T15:00:00Z")},
// 					{ID: 2, Time: timeutil.MustParse("2025-03-01T16:00:00Z")},
// 				},
// 				err: "",
// 			},
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.Scenario, func(t *testing.T) {
// 			// Given.
// 			calls := 0
// 			var blockMocks []*mock.Mock
// 			blockFactory := block.NewFactory(func(obj *mock.Mock) {
// 				if calls < len(tt.GivenDetail.hotBlocks) {
// 					tt.GivenDetail.hotBlocks[calls](obj)
// 					calls++
// 					blockMocks = append(blockMocks, obj)
// 				}
// 			})
// 			var hotBlocks blockChain
// 			for range tt.GivenDetail.hotBlocks {
// 				block, err := blockFactory.New(nil, nil, nil, nil, nil)
// 				Expect(err).To(BeNil())
// 				hotBlocks.add(block)
// 			}

// 			blockFactory = block.NewFactory(func(obj *mock.Mock) {
// 				if calls < len(tt.GivenDetail.hotBlocks)+len(tt.GivenDetail.coldBlocks) {
// 					tt.GivenDetail.coldBlocks[calls-len(tt.GivenDetail.hotBlocks)](obj)
// 					calls++
// 					blockMocks = append(blockMocks, obj)
// 				}
// 			})
// 			var coldBlocks blockChain
// 			for range tt.GivenDetail.coldBlocks {
// 				block, err := blockFactory.New(nil, nil, nil, nil, nil)
// 				Expect(err).To(BeNil())
// 				coldBlocks.add(block)
// 			}

// 			s := storage{
// 				hot:  &hotBlocks,
// 				cold: &coldBlocks,
// 			}

// 			// When.
// 			feeds, err := s.Query(context.Background(), tt.WhenDetail.query)

// 			// Then.
// 			if tt.ThenExpected.err != "" {
// 				Expect(err).NotTo(BeNil())
// 				Expect(err.Error()).To(ContainSubstring(tt.ThenExpected.err))
// 			} else {
// 				Expect(err).To(BeNil())
// 				Expect(feeds).To(HaveLen(len(tt.ThenExpected.feeds)))

// 				// Check feeds match expected
// 				for i, feed := range feeds {
// 					Expect(feed.ID).To(Equal(tt.ThenExpected.feeds[i].ID))
// 					Expect(feed.Time).To(Equal(tt.ThenExpected.feeds[i].Time))
// 					Expect(feed.Labels).To(Equal(tt.ThenExpected.feeds[i].Labels))
// 				}
// 			}

// 			for _, m := range blockMocks {
// 				m.AssertExpectations(t)
// 			}
// 		})
// 	}
// }
