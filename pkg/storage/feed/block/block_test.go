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
package block

// import (
// 	// "context"
// 	"encoding/json"
// 	"os"
// 	"path/filepath"
// 	// "sync/atomic"
// 	"testing"
// 	"time"
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//
//

// 	// "github.com/benbjohnson/clock"
// 	. "github.com/onsi/gomega"
// 	"github.com/stretchr/testify/mock"

// 	"github.com/vandeefeng/zenfeed/pkg/model"
// 	"github.com/vandeefeng/zenfeed/pkg/storage/feed/block/chunk"
// 	// "github.com/vandeefeng/zenfeed/pkg/storage/feed/block/index/inverted"
// 	// "github.com/vandeefeng/zenfeed/pkg/storage/feed/block/index/primary"
// 	// "github.com/vandeefeng/zenfeed/pkg/storage/feed/block/index/vector"
// 	"github.com/vandeefeng/zenfeed/pkg/test"
// 	runtimeutil "github.com/vandeefeng/zenfeed/pkg/util/runtime"
// )

// func TestNew(t *testing.T) {
// 	RegisterTestingT(t)

// 	t0 := time.Now()

// 	type givenDetail struct {
// 		setupDir     func(string) error
// 		config       *Config
// 		chunkMock    func(obj *mock.Mock)
// 		primaryMock  func(obj *mock.Mock)
// 		invertedMock func(obj *mock.Mock)
// 		vectorMock   func(obj *mock.Mock)
// 	}
// 	type whenDetail struct{}
// 	type thenExpected struct {
// 		state                       State
// 		chunkFactoryNewCallCount    int
// 		primaryFactoryNewCallCount  int
// 		invertedFactoryNewCallCount int
// 		vectorFactoryNewCallCount   int
// 		err                         string
// 	}

// 	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
// 		{
// 			Scenario: "Creating new block with valid config",
// 			Given:    "A config with valid parameters",
// 			When:     "Calling New function",
// 			Then:     "Should create a new block successfully",
// 			GivenDetail: givenDetail{
// 				setupDir: func(dir string) error {
// 					return nil // No need to pre-setup, new directory will be created.
// 				},
// 				config: &Config{
// 					Dir:                     "", // Will be set to temp dir in test.
// 					Start:                   t0,
// 					Duration:                24 * time.Hour,
// 					SelectableEmbeddingLLMs: []EmbeddingLLM{{Name: "test", Start: time.Time{}}},
// 				},
// 			},
// 			ThenExpected: thenExpected{
// 				state:                       StateHot,
// 				chunkFactoryNewCallCount:    1,
// 				primaryFactoryNewCallCount:  1,
// 				invertedFactoryNewCallCount: 1,
// 				vectorFactoryNewCallCount:   1,
// 			},
// 		},
// 		{
// 			Scenario: "Recover hot block from disk",
// 			Given:    "Block data directory exists on disk and no archive metadata file",
// 			When:     "Calling New function",
// 			Then:     "Should recover block successfully",
// 			GivenDetail: givenDetail{
// 				setupDir: func(dir string) error {
// 					// Create chunk directory but not create archive.json.
// 					chunkDir := filepath.Join(dir, chunkDirname)
// 					if err := os.MkdirAll(chunkDir, 0755); err != nil {
// 						return err
// 					}
// 					if _, err := os.Create(filepath.Join(chunkDir, chunkFilename(0))); err != nil {
// 						return err
// 					}
// 					if _, err := os.Create(filepath.Join(chunkDir, chunkFilename(1))); err != nil {
// 						return err
// 					}
// 					return nil
// 				},
// 				chunkMock: func(obj *mock.Mock) {
// 					obj.On("Range", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
// 						iter := args.Get(1).(func(feed *chunk.Feed, offset uint64) error)
// 						runtimeutil.Must(iter(&chunk.Feed{ID: 1, Vectors: [][]float32{{1, 2, 3}, {4, 5, 6}}, Feed: &model.Feed{Labels: model.Labels{model.Label{Key: "k1", Value: "v1"}}, Time: time.Date(2025, 3, 3, 0, 0, 0, 0, time.UTC)}}, 0))
// 						runtimeutil.Must(iter(&chunk.Feed{ID: 2, Vectors: [][]float32{{7, 8, 9}, {10, 11, 12}}, Feed: &model.Feed{Labels: model.Labels{model.Label{Key: "k2", Value: "v2"}}, Time: time.Date(2025, 5, 20, 0, 0, 0, 0, time.UTC)}}, 1))
// 					}).Return(nil) // Two chunk files.
// 				},
// 				primaryMock: func(obj *mock.Mock) {
// 					obj.On("Add", mock.Anything, uint64(1), primaryindex.FeedRef{Chunk: 0, Offset: 0, Time: time.Date(2025, 3, 3, 0, 0, 0, 0, time.UTC)}).Return(nil)
// 					obj.On("Add", mock.Anything, uint64(2), primaryindex.FeedRef{Chunk: 0, Offset: 1, Time: time.Date(2025, 5, 20, 0, 0, 0, 0, time.UTC)}).Return(nil)
// 					obj.On("Add", mock.Anything, uint64(1), primaryindex.FeedRef{Chunk: 1, Offset: 0, Time: time.Date(2025, 3, 3, 0, 0, 0, 0, time.UTC)}).Return(nil)
// 					obj.On("Add", mock.Anything, uint64(2), primaryindex.FeedRef{Chunk: 1, Offset: 1, Time: time.Date(2025, 5, 20, 0, 0, 0, 0, time.UTC)}).Return(nil)
// 				},
// 				invertedMock: func(obj *mock.Mock) {
// 					obj.On("Add", mock.Anything, uint64(1), model.Labels{model.Label{Key: "k1", Value: "v1"}}).Return(nil).Times(2) // Two chunk files.
// 					obj.On("Add", mock.Anything, uint64(2), model.Labels{model.Label{Key: "k2", Value: "v2"}}).Return(nil).Times(2)
// 				},
// 				vectorMock: func(obj *mock.Mock) {
// 					obj.On("Add", mock.Anything, uint64(1), [][]float32{{1, 2, 3}, {4, 5, 6}}).Return(nil).Times(2) // Two chunk files.
// 					obj.On("Add", mock.Anything, uint64(2), [][]float32{{7, 8, 9}, {10, 11, 12}}).Return(nil).Times(2)
// 				},
// 				config: &Config{
// 					Dir:                     "", // Will be set to temp dir in test.
// 					Start:                   t0,
// 					Retention:               7 * 24 * time.Hour,
// 					Duration:                24 * time.Hour,
// 					WriteableWindow:         48 * time.Hour, // Within this window.
// 					SelectableEmbeddingLLMs: []EmbeddingLLM{{Name: "test", Start: time.Time{}}},
// 				},
// 			},
// 			ThenExpected: thenExpected{
// 				state:                       StateHot,
// 				chunkFactoryNewCallCount:    2,
// 				primaryFactoryNewCallCount:  1,
// 				invertedFactoryNewCallCount: 1,
// 				vectorFactoryNewCallCount:   1,
// 			},
// 		},
// 		{
// 			Scenario: "Recover hot readonly block from disk",
// 			Given:    "Block data directory exists on disk and no archive metadata file, and WriteableWindow is out of range",
// 			When:     "Calling New function",
// 			Then:     "Should recover block successfully and state is hot readonly",
// 			GivenDetail: givenDetail{
// 				setupDir: func(dir string) error {
// 					// Create chunk directory but not create archive.json.
// 					chunkDir := filepath.Join(dir, chunkDirname)
// 					if err := os.MkdirAll(chunkDir, 0755); err != nil {
// 						return err
// 					}
// 					if _, err := os.Create(filepath.Join(chunkDir, chunkFilename(0))); err != nil {
// 						return err
// 					}
// 					if _, err := os.Create(filepath.Join(chunkDir, chunkFilename(1))); err != nil {
// 						return err
// 					}
// 					return nil
// 				},
// 				chunkMock: func(obj *mock.Mock) {
// 					obj.On("Range", mock.Anything, mock.Anything).Run(func(args mock.Arguments) {
// 						iter := args.Get(1).(func(feed *chunk.Feed, offset uint64) error)
// 						runtimeutil.Must(iter(&chunk.Feed{ID: 1, Feed: &model.Feed{Labels: model.Labels{model.Label{Key: "k1", Value: "v1"}}, Time: time.Date(2025, 3, 3, 0, 0, 0, 0, time.UTC)}}, 0))
// 						runtimeutil.Must(iter(&chunk.Feed{ID: 2, Feed: &model.Feed{Labels: model.Labels{model.Label{Key: "k2", Value: "v2"}}, Time: time.Date(2025, 5, 20, 0, 0, 0, 0, time.UTC)}}, 1))
// 					}).Return(nil) // Two chunk files.
// 				},
// 				primaryMock: func(obj *mock.Mock) {
// 					obj.On("Add", mock.Anything, uint64(1), primaryindex.FeedRef{Chunk: 0, Offset: 0, Time: time.Date(2025, 3, 3, 0, 0, 0, 0, time.UTC)}).Return(nil)
// 					obj.On("Add", mock.Anything, uint64(2), primaryindex.FeedRef{Chunk: 0, Offset: 1, Time: time.Date(2025, 5, 20, 0, 0, 0, 0, time.UTC)}).Return(nil)
// 					obj.On("Add", mock.Anything, uint64(1), primaryindex.FeedRef{Chunk: 1, Offset: 0, Time: time.Date(2025, 3, 3, 0, 0, 0, 0, time.UTC)}).Return(nil)
// 					obj.On("Add", mock.Anything, uint64(2), primaryindex.FeedRef{Chunk: 1, Offset: 1, Time: time.Date(2025, 5, 20, 0, 0, 0, 0, time.UTC)}).Return(nil)
// 				},
// 				invertedMock: func(obj *mock.Mock) {
// 					obj.On("Add", mock.Anything, uint64(1), model.Labels{model.Label{Key: "k1", Value: "v1"}}).Return(nil).Times(2) // Two chunk files.
// 					obj.On("Add", mock.Anything, uint64(2), model.Labels{model.Label{Key: "k2", Value: "v2"}}).Return(nil).Times(2)
// 				},
// 				vectorMock: func(obj *mock.Mock) {
// 					obj.On("Add", mock.Anything, uint64(1), [][]float32{{1, 2, 3}, {4, 5, 6}}).Return(nil).Times(2) // Two chunk files.
// 					obj.On("Add", mock.Anything, uint64(2), [][]float32{{7, 8, 9}, {10, 11, 12}}).Return(nil).Times(2)
// 				},
// 				config: &Config{
// 					Dir:                     "", // Will be set to temp dir in test.
// 					Start:                   t0.Add(-72 * time.Hour),
// 					Retention:               7 * 24 * time.Hour,
// 					Duration:                24 * time.Hour,
// 					WriteableWindow:         25 * time.Hour,
// 					SelectableEmbeddingLLMs: []EmbeddingLLM{{Name: "test", Start: time.Time{}}},
// 				},
// 			},
// 			ThenExpected: thenExpected{
// 				state:                       StateHotReadonly,
// 				chunkFactoryNewCallCount:    2,
// 				primaryFactoryNewCallCount:  1,
// 				invertedFactoryNewCallCount: 1,
// 				vectorFactoryNewCallCount:   1,
// 			},
// 		},
// 		{
// 			Scenario: "Recover cold block from disk",
// 			Given:    "Block data directory exists on disk and has archive metadata file",
// 			When:     "Calling New function",
// 			Then:     "Should recover block successfully as cold state",
// 			GivenDetail: givenDetail{
// 				setupDir: func(dir string) error {
// 					// Create block directory structure with archive.json
// 					chunkDir := filepath.Join(dir, chunkDirname)
// 					if err := os.MkdirAll(chunkDir, 0755); err != nil {
// 						return err
// 					}

// 					// Create chunk files
// 					if _, err := os.Create(filepath.Join(chunkDir, chunkFilename(0))); err != nil {
// 						return err
// 					}
// 					if _, err := os.Create(filepath.Join(chunkDir, chunkFilename(1))); err != nil {
// 						return err
// 					}

// 					// Create archive.json to indicate cold state
// 					meta := archiveMetadata{FeedCount: 4}
// 					bs := runtimeutil.Must1(json.Marshal(meta))
// 					if err := os.WriteFile(filepath.Join(dir, archiveMetaFilename), bs, 0644); err != nil {
// 						return err
// 					}

// 					// Create index directory and files
// 					indexDir := filepath.Join(dir, indexDirname)
// 					if err := os.MkdirAll(indexDir, 0755); err != nil {
// 						return err
// 					}
// 					for _, name := range []string{indexPrimaryFilename, indexInvertedFilename, indexVectorFilename} {
// 						if _, err := os.Create(filepath.Join(indexDir, name)); err != nil {
// 							return err
// 						}
// 					}
// 					return nil
// 				},
// 				chunkMock: func(obj *mock.Mock) {
// 					// No chunk mock needed since cold block doesn't load chunks initially
// 				},
// 				primaryMock: func(obj *mock.Mock) {
// 					// No primary mock needed since cold block doesn't load indices initially
// 				},
// 				invertedMock: func(obj *mock.Mock) {
// 					// No inverted mock needed since cold block doesn't load indices initially
// 				},
// 				vectorMock: func(obj *mock.Mock) {
// 					// No vector mock needed since cold block doesn't load indices initially
// 				},
// 				config: &Config{
// 					Dir:                     "", // Will be set to temp dir in test
// 					Start:                   t0,
// 					Retention:               7 * 24 * time.Hour,
// 					Duration:                24 * time.Hour,
// 					WriteableWindow:         48 * time.Hour,
// 					SelectableEmbeddingLLMs: []EmbeddingLLM{{Name: "test", Start: time.Time{}}},
// 				},
// 			},
// 			ThenExpected: thenExpected{
// 				state:                       StateCold,
// 				chunkFactoryNewCallCount:    0, // Cold block doesn't load chunks initially
// 				primaryFactoryNewCallCount:  1,
// 				invertedFactoryNewCallCount: 1,
// 				vectorFactoryNewCallCount:   1,
// 			},
// 		},
// 		{
// 			Scenario: "Creating new block - invalid config - missing SelectableEmbeddingLLMs",
// 			Given:    "A config with missing SelectableEmbeddingLLMs",
// 			When:     "Calling New function",
// 			Then:     "Should return an error",
// 			GivenDetail: givenDetail{
// 				config: &Config{
// 					Dir:      "", // Will be set to temp dir in test.
// 					Start:    t0,
// 					Duration: 24 * time.Hour,
// 				},
// 			},
// 			ThenExpected: thenExpected{
// 				err: "selectable embedding LLMs is required",
// 			},
// 		},
// 	}

// 	for _, tt := range tests {
// 		t.Run(tt.Scenario, func(t *testing.T) {
// 			// Given.
// 			tempDir, err := os.MkdirTemp("", "block_test_*")
// 			Expect(err).NotTo(HaveOccurred())
// 			defer os.RemoveAll(tempDir)

// 			// Set parent directory of config.
// 			if tt.GivenDetail.config != nil && tt.GivenDetail.config.Dir == "" {
// 				tt.GivenDetail.config.Dir = tempDir
// 			}

// 			// Execute setup of test scenario.
// 			if tt.GivenDetail.setupDir != nil {
// 				err := tt.GivenDetail.setupDir(tempDir)
// 				Expect(err).NotTo(HaveOccurred())
// 			}

// 			// Create mock factory.
// 			chunkFactoryNewCallCount := 0
// 			mockChunkFactory := chunk.NewFactory(func(obj *mock.Mock) {
// 				if tt.GivenDetail.chunkMock != nil {
// 					tt.GivenDetail.chunkMock(obj)
// 				}
// 				chunkFactoryNewCallCount++
// 			})
// 			primaryFactoryNewCallCount := 0
// 			mockPrimaryFactory := primaryindex.NewFactory(func(obj *mock.Mock) {
// 				if tt.GivenDetail.primaryMock != nil {
// 					tt.GivenDetail.primaryMock(obj)
// 				}
// 				primaryFactoryNewCallCount++
// 			})
// 			invertedFactoryNewCallCount := 0
// 			mockInvertedFactory := invertedindex.NewFactory(func(obj *mock.Mock) {
// 				if tt.GivenDetail.invertedMock != nil {
// 					tt.GivenDetail.invertedMock(obj)
// 				}
// 				invertedFactoryNewCallCount++
// 			})
// 			vectorFactoryNewCallCount := 0
// 			mockVectorFactory := vectorindex.NewFactory(func(obj *mock.Mock) {
// 				if tt.GivenDetail.vectorMock != nil {
// 					tt.GivenDetail.vectorMock(obj)
// 				}
// 				vectorFactoryNewCallCount++
// 			})

// 			// When.
// 			b, err := new("test", tt.GivenDetail.config, Dependencies{
// 				ChunkFactory:    mockChunkFactory,
// 				PrimaryFactory:  mockPrimaryFactory,
// 				InvertedFactory: mockInvertedFactory,
// 				VectorFactory:   mockVectorFactory,
// 			})

// 			// Then.
// 			if tt.ThenExpected.err != "" {
// 				Expect(err).To(HaveOccurred())
// 				Expect(err.Error()).To(ContainSubstring(tt.ThenExpected.err))
// 			} else {
// 				Expect(err).NotTo(HaveOccurred())
// 				Expect(b).NotTo(BeNil())

// 				// Validate self properties.
// 				rb := b.(*block)
// 				Expect(rb.state.Load().(State)).To(Equal(tt.ThenExpected.state))
// 				Expect(rb.chunks).To(HaveLen(tt.ThenExpected.chunkFactoryNewCallCount))

// 				// Validate dependencies.
// 				Expect(chunkFactoryNewCallCount).To(Equal(tt.ThenExpected.chunkFactoryNewCallCount))
// 				Expect(primaryFactoryNewCallCount).To(Equal(tt.ThenExpected.primaryFactoryNewCallCount))
// 				Expect(invertedFactoryNewCallCount).To(Equal(tt.ThenExpected.invertedFactoryNewCallCount))
// 				Expect(vectorFactoryNewCallCount).To(Equal(tt.ThenExpected.vectorFactoryNewCallCount))
// 				if tt.GivenDetail.config != nil {
// 					chunkDir := filepath.Join(tt.GivenDetail.config.Dir, chunkDirname)
// 					stat, err := os.Stat(chunkDir)
// 					Expect(err).NotTo(HaveOccurred())
// 					Expect(stat.IsDir()).To(BeTrue())
// 				}
// 			}
// 		})
// 	}
// }

// // func TestRun(t *testing.T) {
// // 	RegisterTestingT(t)

// // 	t0 := time.Date(2025, 3, 3, 0, 0, 0, 0, time.UTC)

// // 	type givenDetail struct {
// // 		config       *Config
// // 		initialState State
// // 		setup        func(b *block, mockClock *clock.Mock)
// // 	}
// // 	type whenDetail struct{}
// // 	type thenExpected struct {
// // 		state         State
// // 		chunksLen     int
// // 		chunksNil     bool
// // 		primaryNil    bool
// // 		invertedNil   bool
// // 		vectorNil     bool
// // 		archiveExists bool
// // 	}

// // 	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
// // 		{
// // 			Scenario: "Hot to HotReadonly transition",
// // 			Given:    "A hot block that is about to exceed its writeable window",
// // 			When:     "Running the block",
// // 			Then:     "Should transition to hot-readonly state",
// // 			GivenDetail: givenDetail{
// // 				config: &Config{
// // 					Dir:             "/tmp", // Use Dir instead of ParentDir
// // 					Start:           t0,
// // 					Duration:        24 * time.Hour,
// // 					WriteableWindow: 48 * time.Hour,
// // 				},
// // 				initialState: StateHot,
// // 				setup: func(b *block, mockClock *clock.Mock) {
// // 					mockClock.Set(t0.Add(49 * time.Hour)) // Past writeable window.
// // 				},
// // 			},
// // 			ThenExpected: thenExpected{
// // 				state: StateHotReadonly,
// // 			},
// // 		},
// // 		{
// // 			Scenario: "HotReadonly to Hot transition",
// // 			Given:    "A hot-readonly block with a new, wider writeable window",
// // 			When:     "Running the block",
// // 			Then:     "Should transition to hot state and add a new chunk",
// // 			GivenDetail: givenDetail{
// // 				config: &Config{
// // 					Dir:             "/tmp", // Use Dir instead of ParentDir
// // 					Start:           t0,
// // 					Duration:        24 * time.Hour,
// // 					WriteableWindow: 48 * time.Hour,
// // 				},
// // 				initialState: StateHotReadonly,
// // 				setup: func(b *block, mockClock *clock.Mock) {
// // 					mockClock.Set(t0.Add(47 * time.Hour))
// // 				},
// // 			},
// // 			ThenExpected: thenExpected{
// // 				state:     StateHot,
// // 				chunksLen: 2 + 1, // Add 1 chunk when rollback to Hot.
// // 			},
// // 		},
// // 		{
// // 			Scenario: "HotReadonly to Cold transition",
// // 			Given:    "A hot-readonly block with no recent data access",
// // 			When:     "Running the block",
// // 			Then:     "Should transition to cold state and release resources",
// // 			GivenDetail: givenDetail{
// // 				config: &Config{
// // 					Dir:             "/tmp", // Use Dir instead of ParentDir
// // 					Start:           t0.Add(-49 * time.Hour),
// // 					Duration:        24 * time.Hour,
// // 					WriteableWindow: 48 * time.Hour,
// // 				},
// // 				initialState: StateHotReadonly,
// // 				setup: func(b *block, mockClock *clock.Mock) {
// // 					b.lastDataAccess.Store(t0)
// // 					mockClock.Set(t0.Add(6 * time.Minute)) // Past colding window (5 minutes)
// // 				},
// // 			},
// // 			ThenExpected: thenExpected{
// // 				state:       StateCold,
// // 				chunksNil:   true,
// // 				primaryNil:  true,
// // 				invertedNil: true,
// // 				vectorNil:   true,
// // 			},
// // 		},
// // 		{
// // 			Scenario: "ColdLoaded to Cold transition",
// // 			Given:    "A cold-loaded block with no recent data access",
// // 			When:     "Running the block",
// // 			Then:     "Should transition to cold state and release resources",
// // 			GivenDetail: givenDetail{
// // 				config: &Config{
// // 					Dir:             "/tmp", // Use Dir instead of ParentDir
// // 					Start:           t0,
// // 					Duration:        24 * time.Hour,
// // 					WriteableWindow: 48 * time.Hour,
// // 				},
// // 				initialState: StateColdLoaded,
// // 				setup: func(b *block, mockClock *clock.Mock) {
// // 					b.lastDataAccess.Store(t0)
// // 					mockClock.Set(t0.Add(6 * time.Minute)) // Past colding window (5 minutes)
// // 				},
// // 			},
// // 			ThenExpected: thenExpected{
// // 				state:       StateCold,
// // 				chunksNil:   true,
// // 				primaryNil:  true,
// // 				invertedNil: true,
// // 				vectorNil:   true,
// // 			},
// // 		},
// // 		{
// // 			Scenario: "Hot to ColdExpired transition",
// // 			Given:    "A hot block exceeding retention period",
// // 			When:     "Running the block",
// // 			Then:     "Should transition to ColdExpired state and create archive file",
// // 			GivenDetail: givenDetail{
// // 				config: &Config{
// // 					Dir:             "/tmp", // Use Dir instead of ParentDir
// // 					Start:           t0.Add(-8 * 24 * time.Hour),
// // 					Duration:        24 * time.Hour,
// // 					WriteableWindow: 48 * time.Hour,
// // 					Retention:       7 * 24 * time.Hour,
// // 				},
// // 				initialState: StateHot,
// // 				setup: func(b *block, mockClock *clock.Mock) {
// // 					mockClock.Set(t0)
// // 				},
// // 			},
// // 			ThenExpected: thenExpected{
// // 				state:         StateColdExpired,
// // 				archiveExists: true,
// // 			},
// // 		},
// // 	}

// // 	for _, tt := range tests {
// // 		t.Run(tt.Scenario, func(t *testing.T) {
// // 			// Given.

// // 			// Setup mock clock
// // 			mockClock := clock.NewMock()
// // 			clk = mockClock

// // 			// Create test block
// // 			chunkFactory := chunk.NewFactory(func(obj *mock.Mock) {
// // 				obj.On("EnsureReadonly").Return(nil).Times(2)
// // 				obj.On("Close").Return(nil).Times(3) // Max 3 chunks (2 initial + 1 possible new)
// // 			})
// // 			chunk1, err := chunkFactory.New(&chunk.Config{})
// // 			Expect(err).NotTo(HaveOccurred())
// // 			chunk2, err := chunkFactory.New(&chunk.Config{})
// // 			Expect(err).NotTo(HaveOccurred())
// // 			primaryFactory := primaryindex.NewFactory(func(obj *mock.Mock) {
// // 				obj.On("EncodeTo", mock.Anything).Return(nil)
// // 				obj.On("Count").Return(uint32(4))
// // 				obj.On("Close").Return(nil)
// // 			})
// // 			invertedFactory := invertedindex.NewFactory(func(obj *mock.Mock) {
// // 				obj.On("EncodeTo", mock.Anything).Return(nil)
// // 				obj.On("Close").Return(nil)
// // 			})
// // 			vectorFactory := vectorindex.NewFactory(func(obj *mock.Mock) {
// // 				obj.On("EncodeTo", mock.Anything).Return(nil)
// // 				obj.On("Close").Return(nil)
// // 			})

// // 			// Create a temporary directory for testing
// // 			tempDir, err := os.MkdirTemp("", "block_run_test")
// // 			Expect(err).NotTo(HaveOccurred())
// // 			defer os.RemoveAll(tempDir)

// // 			// Update the config to use the temporary directory
// // 			config := tt.GivenDetail.config
// // 			config.Dir = tempDir

// // 			ctx, cancel := context.WithCancel(context.Background())
// // 			b := &block{
// // 				ctx:           ctx,
// // 				cancel:        cancel,
// // 				chunks:        chunkChain{chunk1, chunk2},
// // 				blockDirpath:  filepath.Join(tempDir, blockDirname(config.Start, config.Start.Add(config.Duration))),
// // 				indexDirpath:  filepath.Join(tempDir, blockDirname(config.Start, config.Start.Add(config.Duration)), indexDirname),
// // 				chunkDirpath:  filepath.Join(tempDir, blockDirname(config.Start, config.Start.Add(config.Duration)), chunkDirname),
// // 				chunkFactory:  chunkFactory,
// // 				primaryIndex:  primaryFactory.New(),
// // 				invertedIndex: invertedFactory.New(),
// // 				vectorIndex:   vectorFactory.New(),
// // 				config:        atomic.Pointer[Config]{},
// // 			}
// // 			b.config.Store(config)
// // 			b.state.Store(tt.GivenDetail.initialState)

// // 			// Setup test scenario
// // 			if tt.GivenDetail.setup != nil {
// // 				tt.GivenDetail.setup(b, mockClock)
// // 			}

// // 			// When.

// // 			// Run block in goroutine.
// // 			done := make(chan error)
// // 			go func() {
// // 				done <- b.Run()
// // 			}()

// // 			// Wait for state transition.
// // 			time.Sleep(100 * time.Millisecond)
// // 			clk.(*clock.Mock).Add(35 * time.Second) // Wait tick

// // 			// Then.

// // 			// Verify state.
// // 			Expect(b.state.Load()).To(Equal(tt.ThenExpected.state))

// // 			if tt.ThenExpected.chunksLen > 0 {
// // 				Expect(b.chunks).To(HaveLen(tt.ThenExpected.chunksLen))
// // 			}
// // 			if tt.ThenExpected.chunksNil {
// // 				Expect(b.chunks).To(BeNil())
// // 			}
// // 			if tt.ThenExpected.primaryNil {
// // 				Expect(b.primaryIndex).To(BeNil())
// // 			}
// // 			if tt.ThenExpected.invertedNil {
// // 				Expect(b.invertedIndex).To(BeNil())
// // 			}
// // 			if tt.ThenExpected.vectorNil {
// // 				Expect(b.vectorIndex).To(BeNil())
// // 			}
// // 			if tt.ThenExpected.archiveExists {
// // 				archivePath := filepath.Join(b.blockDirpath, archiveMetaFilename)
// // 				_, err := os.Stat(archivePath)
// // 				Expect(err).NotTo(HaveOccurred()) // Check archive file exists
// // 			}

// // 			// Cleanup.
// // 			b.cancel()
// // 			err = <-done
// // 			Expect(err).NotTo(HaveOccurred())
// // 		})
// // 	}
// // }

// // func TestReload(t *testing.T) {
// // 	RegisterTestingT(t)

// // 	t0 := time.Date(2025, 3, 3, 0, 0, 0, 0, time.UTC)
// // 	tests := []struct {
// // 		scenario    string
// // 		given       string
// // 		when        string
// // 		then        string
// // 		initialCfg  *Config
// // 		reloadCfg   *Config
// // 		expectedErr string
// // 	}{
// // 		{
// // 			scenario: "Successful reload with mutable fields",
// // 			given:    "a block with initial config",
// // 			when:     "reloading with valid changes to mutable fields",
// // 			then:     "should accept the new config",
// // 			initialCfg: &Config{
// // 				ParentDir:       "/tmp",
// // 				BlockDirname:    blockDirname(t0, t0.Add(24*time.Hour)),
// // 				Start:           t0,
// // 				Duration:        24 * time.Hour,
// // 				WriteableWindow: 48 * time.Hour,
// // 				Retention:       7 * 24 * time.Hour,
// // 			},
// // 			reloadCfg: &Config{
// // 				WriteableWindow: 72 * time.Hour,      // Changed
// // 				Retention:       10 * 24 * time.Hour, // Changed
// // 			},
// // 		},
// // 		{
// // 			scenario: "Attempt to change ParentDir",
// // 			given:    "a block with initial config",
// // 			when:     "trying to change ParentDir",
// // 			then:     "should reject the change",
// // 			initialCfg: &Config{
// // 				ParentDir:    "/tmp",
// // 				BlockDirname: blockDirname(t0, t0.Add(24*time.Hour)),
// // 				Start:        t0,
// // 				Duration:     24 * time.Hour,
// // 			},
// // 			reloadCfg: &Config{
// // 				ParentDir: "/new/path",
// // 			},
// // 			expectedErr: "cannot reload the parent dir, MUST pass the same dir, or set it to empty for unchange",
// // 		},
// // 		{
// // 			scenario: "Attempt to change BlockDirname",
// // 			given:    "a block with initial config",
// // 			when:     "trying to change BlockDirname",
// // 			then:     "should reject the change",
// // 			initialCfg: &Config{
// // 				ParentDir:    "/tmp",
// // 				BlockDirname: blockDirname(t0, t0.Add(24*time.Hour)),
// // 				Start:        t0,
// // 				Duration:     24 * time.Hour,
// // 			},
// // 			reloadCfg: &Config{
// // 				BlockDirname: blockDirname(t0.Add(24*time.Hour), t0.Add(48*time.Hour)),
// // 			},
// // 			expectedErr: "cannot reload the block dirname, MUST pass the same dirname, or set it to empty for unchange",
// // 		},
// // 		{
// // 			scenario: "Attempt to change Start time",
// // 			given:    "a block with initial config",
// // 			when:     "trying to change Start time",
// // 			then:     "should reject the change",
// // 			initialCfg: &Config{
// // 				ParentDir:    "/tmp",
// // 				BlockDirname: blockDirname(t0, t0.Add(24*time.Hour)),
// // 				Start:        t0,
// // 				Duration:     24 * time.Hour,
// // 			},
// // 			reloadCfg: &Config{
// // 				Start: t0.Add(24 * time.Hour),
// // 			},
// // 			expectedErr: "cannot reload the start time, MUST pass the same start time, or set it to empty for unchange",
// // 		},
// // 		{
// // 			scenario: "Attempt to change Duration",
// // 			given:    "a block with initial config",
// // 			when:     "trying to change Duration",
// // 			then:     "should reject the change",
// // 			initialCfg: &Config{
// // 				ParentDir:    "/tmp",
// // 				BlockDirname: blockDirname(t0, t0.Add(24*time.Hour)),
// // 				Start:        t0,
// // 				Duration:     24 * time.Hour,
// // 			},
// // 			reloadCfg: &Config{
// // 				Duration: 48 * time.Hour,
// // 			},
// // 			expectedErr: "cannot reload the duration, MUST pass the same duration, or set it to empty for unchange",
// // 		},
// // 		{
// // 			scenario: "Invalid config validation",
// // 			given:    "a block with initial config",
// // 			when:     "trying to reload with invalid config",
// // 			then:     "should reject the change",
// // 			initialCfg: &Config{
// // 				ParentDir:       "/tmp",
// // 				BlockDirname:    blockDirname(t0, t0.Add(24*time.Hour)),
// // 				Start:           t0,
// // 				Duration:        24 * time.Hour,
// // 				WriteableWindow: 48 * time.Hour,
// // 				Retention:       7 * 24 * time.Hour,
// // 			},
// // 			reloadCfg: &Config{
// // 				WriteableWindow: 1 * time.Hour, // Invalid: less than Duration
// // 			},
// // 			expectedErr: "validate config: writeable window must be greater than 24h0m0s",
// // 		},
// // 		{
// // 			scenario: "Concurrent reload attempt",
// // 			given:    "a block that is already reloading",
// // 			when:     "trying to reload again",
// // 			then:     "should reject the concurrent reload",
// // 			initialCfg: &Config{
// // 				ParentDir:    "/tmp",
// // 				BlockDirname: blockDirname(t0, t0.Add(24*time.Hour)),
// // 				Start:        t0,
// // 				Duration:     24 * time.Hour,
// // 			},
// // 			reloadCfg: &Config{
// // 				WriteableWindow: 72 * time.Hour,
// // 			},
// // 			expectedErr: "concurrent reloading is forbidden",
// // 		},
// // 	}

// // 	for _, tt := range tests {
// // 		t.Run(tt.scenario, func(t *testing.T) {
// // 			// Create block with initial config
// // 			b := &block{
// // 				config:                atomic.Value{},
// // 				newConfigToReload:     make(chan *Config, 1),
// // 				newConfigReloadResult: make(chan error, 1),
// // 			}
// // 			b.config.Store(tt.initialCfg)

// // 			// For concurrent reload test
// // 			if tt.expectedErr == "concurrent reloading is forbidden" {
// // 				b.reloading.Store(true)
// // 			}

// // 			// Setup goroutine to handle reload request for successful cases
// // 			if tt.expectedErr == "" {
// // 				go func() {
// // 					cfg := <-b.newConfigToReload
// // 					// Verify the config was properly merged
// // 					Expect(cfg.ParentDir).To(Equal(tt.initialCfg.ParentDir))
// // 					Expect(cfg.BlockDirname).To(Equal(tt.initialCfg.BlockDirname))
// // 					Expect(cfg.Start).To(Equal(tt.initialCfg.Start))
// // 					Expect(cfg.Duration).To(Equal(tt.initialCfg.Duration))
// // 					if tt.reloadCfg.WriteableWindow != 0 {
// // 						Expect(cfg.WriteableWindow).To(Equal(tt.reloadCfg.WriteableWindow))
// // 					}
// // 					if tt.reloadCfg.Retention != 0 {
// // 						Expect(cfg.Retention).To(Equal(tt.reloadCfg.Retention))
// // 					}
// // 					b.config.Store(cfg)
// // 					b.newConfigReloadResult <- nil
// // 				}()
// // 			}

// // 			// Execute test
// // 			err := b.Reload(tt.reloadCfg)

// // 			// Verify results
// // 			if tt.expectedErr != "" {
// // 				Expect(err).To(HaveOccurred())
// // 				Expect(err.Error()).To(ContainSubstring(tt.expectedErr))
// // 			} else {
// // 				Expect(err).NotTo(HaveOccurred())

// // 				// Verify config was updated for successful cases
// // 				newConfig := b.config.Load().(*Config)
// // 				if tt.reloadCfg.WriteableWindow != 0 {
// // 					Expect(newConfig.WriteableWindow).To(Equal(tt.reloadCfg.WriteableWindow))
// // 				}
// // 				if tt.reloadCfg.Retention != 0 {
// // 					Expect(newConfig.Retention).To(Equal(tt.reloadCfg.Retention))
// // 				}
// // 			}
// // 		})
// // 	}
// // }

// // func TestAppend(t *testing.T) {
// // 	RegisterTestingT(t)

// // 	t0 := time.Date(2025, 3, 3, 0, 0, 0, 0, time.UTC)
// // 	clk = clock.NewMock()
// // 	clk.(*clock.Mock).Set(t0)

// // 	tests := []struct {
// // 		scenario           string
// // 		given              string
// // 		when               string
// // 		then               string
// // 		state              State
// // 		setupChunkMocks    func(chunkMock *mock.Mock)
// // 		setupPrimaryMocks  func(primaryMock *mock.Mock)
// // 		setupInvertedMocks func(invertedMock *mock.Mock)
// // 		setupVectorMocks   func(vectorMock *mock.Mock)
// // 		feeds              []*chunk.Feed
// // 		expectedErr        string
// // 	}{
// // 		{
// // 			scenario: "Successfully append feeds to hot block",
// // 			given:    "a hot block",
// // 			when:     "appending feeds",
// // 			then:     "should write to chunk and update indices",
// // 			state:    StateHot,
// // 			setupChunkMocks: func(chunkMock *mock.Mock) {
// // 				chunkMock.On("Count").Return(uint32(0))
// // 				chunkMock.On("Append", mock.Anything, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
// // 					feeds := args.Get(0).([]*chunk.Feed)
// // 					callback := args.Get(1).(func(*chunk.Feed, uint64))
// // 					for i, feed := range feeds {
// // 						callback(feed, uint64(i))
// // 					}
// // 				})
// // 			},
// // 			setupPrimaryMocks: func(primaryMock *mock.Mock) {
// // 				primaryMock.On("Add", uint64(1), primaryindex.FeedRef{
// // 					Chunk:  0,
// // 					Offset: 0,
// // 					Time:   t0,
// // 				}).Return(nil)
// // 			},
// // 			setupInvertedMocks: func(invertedMock *mock.Mock) {
// // 				invertedMock.On("Add", uint64(1), model.Labels{model.Label{Key: "k1", Value: "v1"}}).Return(nil)
// // 			},
// // 			setupVectorMocks: func(vectorMock *mock.Mock) {
// // 				vectorMock.On("Add", uint64(1), [][]float32{{1, 2, 3}}).Return(nil)
// // 			},
// // 			feeds: []*chunk.Feed{
// // 				{
// // 					ID:      1,
// // 					Labels:  model.Labels{model.Label{Key: "k1", Value: "v1"}},
// // 					Vectors: [][]float32{{1, 2, 3}},
// // 					Time:    t0,
// // 				},
// // 			},
// // 		},
// // 		{
// // 			scenario:    "Append to non-hot block",
// // 			given:       "a hot-readonly block",
// // 			when:        "appending feeds",
// // 			then:        "should return error",
// // 			state:       StateHotReadonly,
// // 			feeds:       []*chunk.Feed{{ID: 1}},
// // 			expectedErr: "block is not writable",
// // 		},
// // 		{
// // 			scenario: "Create new chunk when current is full",
// // 			given:    "a hot block with chunk near capacity",
// // 			when:     "appending feeds that would exceed capacity",
// // 			then:     "should create new chunk and write to it",
// // 			state:    StateHot,
// // 			setupChunkMocks: func(chunkMock *mock.Mock) {
// // 				chunkMock.On("Count").Return(uint32(estimatedChunkFeedsLimit)).Twice()
// // 				chunkMock.On("EnsureReadonly").Return(nil).Once()
// // 				chunkMock.On("Append", mock.Anything, mock.Anything).Return(nil).Once()
// // 			},
// // 			setupPrimaryMocks: func(primaryMock *mock.Mock) {
// // 				primaryMock.On("Add", uint64(1), primaryindex.FeedRef{
// // 					Chunk:  0,
// // 					Offset: 0,
// // 					Time:   t0,
// // 				}).Return(nil)
// // 			},
// // 			setupInvertedMocks: func(invertedMock *mock.Mock) {
// // 				invertedMock.On("Add", uint64(1), model.Labels{model.Label{Key: "k1", Value: "v1"}}).Return(nil)
// // 			},
// // 			setupVectorMocks: func(vectorMock *mock.Mock) {
// // 				vectorMock.On("Add", uint64(1), [][]float32{{1, 2, 3}}).Return(nil)
// // 			},
// // 			feeds: []*chunk.Feed{
// // 				{
// // 					ID:      1,
// // 					Labels:  model.Labels{model.Label{Key: "k1", Value: "v1"}},
// // 					Vectors: [][]float32{{1, 2, 3}},
// // 					Time:    t0,
// // 				},
// // 			},
// // 		},
// // 	}

// // 	for _, tt := range tests {
// // 		t.Run(tt.scenario, func(t *testing.T) {
// // 			// Create mock factories
// // 			chunkFactory := chunk.NewFactory(func(obj *mock.Mock) {
// // 				if tt.setupChunkMocks != nil {
// // 					tt.setupChunkMocks(obj)
// // 				}
// // 			})
// // 			primaryFactory := primaryindex.NewFactory(func(obj *mock.Mock) {
// // 				if tt.setupPrimaryMocks != nil {
// // 					tt.setupPrimaryMocks(obj)
// // 				}
// // 			})
// // 			invertedFactory := invertedindex.NewFactory(func(obj *mock.Mock) {
// // 				if tt.setupInvertedMocks != nil {
// // 					tt.setupInvertedMocks(obj)
// // 				}
// // 			})
// // 			vectorFactory := vectorindex.NewFactory(func(obj *mock.Mock) {
// // 				if tt.setupVectorMocks != nil {
// // 					tt.setupVectorMocks(obj)
// // 				}
// // 			})

// // 			// Create block.
// // 			chunk, err := chunkFactory.New(&chunk.Config{
// // 				Path: "/tmp/append",
// // 			})
// // 			Expect(err).NotTo(HaveOccurred())
// // 			b := &block{
// // 				config:        atomic.Value{},
// // 				state:         atomic.Value{},
// // 				chunks:        chunkChain{chunk},
// // 				primaryIndex:  primaryFactory.New(),
// // 				invertedIndex: invertedFactory.New(),
// // 				vectorIndex:   vectorFactory.New(),
// // 				chunkFactory:  chunkFactory,
// // 			}
// // 			b.config.Store(&Config{
// // 				ParentDir: "/tmp",
// // 				Start:     t0,
// // 				Duration:  24 * time.Hour,
// // 			})
// // 			b.state.Store(tt.state)

// // 			// Execute test.
// // 			err = b.Append(context.Background(), tt.feeds...)
// // 			if tt.expectedErr != "" {
// // 				Expect(err).To(HaveOccurred())
// // 				Expect(err.Error()).To(ContainSubstring(tt.expectedErr))
// // 			} else {
// // 				Expect(err).NotTo(HaveOccurred())
// // 			}
// // 		})
// // 	}
// // }

// // func TestQuery(t *testing.T) {
// // 	RegisterTestingT(t)

// // 	t0 := time.Date(2025, 3, 3, 0, 0, 0, 0, time.UTC)
// // 	clk = clock.NewMock()
// // 	clk.(*clock.Mock).Set(t0)

// // 	tests := []struct {
// // 		scenario           string
// // 		given              string
// // 		when               string
// // 		then               string
// // 		state              State
// // 		setupChunkMocks    func(chunkMock *mock.Mock)
// // 		setupPrimaryMocks  func(primaryMock *mock.Mock)
// // 		setupInvertedMocks func(invertedMock *mock.Mock)
// // 		setupVectorMocks   func(vectorMock *mock.Mock)
// // 		queryOpts          QueryOptions
// // 		expectedFeeds      []*FeedVO
// // 		expectedErr        string
// // 	}{
// // 		{
// // 			scenario: "Query hot block with label filters",
// // 			given:    "a hot block with indexed feeds",
// // 			when:     "querying with label filters",
// // 			then:     "should return matching feeds",
// // 			state:    StateHot,
// // 			setupInvertedMocks: func(m *mock.Mock) {
// // 				m.On("Search", "k1", true, "v1").Return(map[uint64]bool{1: true})
// // 			},
// // 			setupPrimaryMocks: func(m *mock.Mock) {
// // 				m.On("Search", uint64(1)).Return(primaryindex.FeedRef{
// // 					Chunk:  0,
// // 					Offset: 0,
// // 					Time:   t0,
// // 				}, true)
// // 			},
// // 			setupChunkMocks: func(m *mock.Mock) {
// // 				m.On("Read", uint64(0)).Return(&chunk.Feed{
// // 					ID:     1,
// // 					Labels: model.Labels{model.Label{Key: "k1", Value: "v1"}},
// // 					Time:   t0,
// // 				}, nil)
// // 			},
// // 			queryOpts: QueryOptions{
// // 				Filters: []LabelFilter{{
// // 					Label: "k1",
// // 					Equal: true,
// // 					Value: "v1",
// // 				}},
// // 				Start: t0.Add(-1 * time.Hour),
// // 				End:   t0.Add(1 * time.Hour),
// // 			},
// // 			expectedFeeds: []*FeedVO{{
// // 				ID:     1,
// // 				Labels: model.Labels{model.Label{Key: "k1", Value: "v1"}},
// // 				Time:   t0,
// // 				Score:  0,
// // 			}},
// // 		},
// // 		{
// // 			scenario: "Query with semantic filter",
// // 			given:    "a block with vector indexed feeds",
// // 			when:     "querying with semantic filter",
// // 			then:     "should return semantically matching feeds with scores",
// // 			state:    StateHot,
// // 			setupVectorMocks: func(m *mock.Mock) {
// // 				m.On("Search", []float32{0.1, 0.2}, float32(0.8)).Return(map[uint64]float32{
// // 					1: 0.9,
// // 				})
// // 			},
// // 			setupPrimaryMocks: func(m *mock.Mock) {
// // 				m.On("Search", uint64(1)).Return(primaryindex.FeedRef{
// // 					Chunk:  0,
// // 					Offset: 0,
// // 					Time:   t0,
// // 				}, true)
// // 			},
// // 			setupChunkMocks: func(m *mock.Mock) {
// // 				m.On("Read", uint64(0)).Return(&chunk.Feed{
// // 					ID:      1,
// // 					Labels:  model.Labels{},
// // 					Time:    t0,
// // 					Vectors: [][]float32{{0.1, 0.2}},
// // 				}, nil)
// // 			},
// // 			queryOpts: QueryOptions{
// // 				SemanticFilter: SemanticFilter{
// // 					QueryVector: []float32{0.1, 0.2},
// // 					Threshold:   0.8,
// // 				},
// // 				Start: t0.Add(-1 * time.Hour),
// // 				End:   t0.Add(1 * time.Hour),
// // 			},
// // 			expectedFeeds: []*FeedVO{{
// // 				ID:     1,
// // 				Labels: model.Labels{},
// // 				Time:   t0,
// // 				Score:  0.9,
// // 			}},
// // 		},
// // 		{
// // 			scenario: "Query with time range filter",
// // 			given:    "a block with feeds at different times",
// // 			when:     "querying with time range",
// // 			then:     "should only return feeds within range",
// // 			state:    StateHot,
// // 			setupPrimaryMocks: func(m *mock.Mock) {
// // 				m.On("IDs").Return(map[uint64]bool{1: true, 2: true})
// // 				m.On("Search", uint64(1)).Return(primaryindex.FeedRef{
// // 					Chunk:  0,
// // 					Offset: 0,
// // 					Time:   t0,
// // 				}, true)
// // 				m.On("Search", uint64(2)).Return(primaryindex.FeedRef{
// // 					Chunk:  0,
// // 					Offset: 1,
// // 					Time:   t0.Add(2 * time.Hour),
// // 				}, true)
// // 			},
// // 			setupChunkMocks: func(m *mock.Mock) {
// // 				m.On("Read", uint64(0)).Return(&chunk.Feed{
// // 					ID:     1,
// // 					Labels: model.Labels{},
// // 					Time:   t0,
// // 				}, nil)
// // 			},
// // 			queryOpts: QueryOptions{
// // 				Start: t0.Add(-1 * time.Hour),
// // 				End:   t0.Add(1 * time.Hour),
// // 			},
// // 			expectedFeeds: []*FeedVO{{
// // 				ID:     1,
// // 				Labels: model.Labels{},
// // 				Time:   t0,
// // 			}},
// // 		},
// // 	}

// // 	for _, tt := range tests {
// // 		t.Run(tt.scenario, func(t *testing.T) {
// // 			// Create mock factories
// // 			chunkFactory := chunk.NewFactory(func(obj *mock.Mock) {
// // 				if tt.setupChunkMocks != nil {
// // 					tt.setupChunkMocks(obj)
// // 				}
// // 			})
// // 			primaryFactory := primaryindex.NewFactory(func(obj *mock.Mock) {
// // 				if tt.setupPrimaryMocks != nil {
// // 					tt.setupPrimaryMocks(obj)
// // 				}
// // 			})
// // 			invertedFactory := invertedindex.NewFactory(func(obj *mock.Mock) {
// // 				if tt.setupInvertedMocks != nil {
// // 					tt.setupInvertedMocks(obj)
// // 				}
// // 			})
// // 			vectorFactory := vectorindex.NewFactory(func(obj *mock.Mock) {
// // 				if tt.setupVectorMocks != nil {
// // 					tt.setupVectorMocks(obj)
// // 				}
// // 			})

// // 			// Create block
// // 			chunk, err := chunkFactory.New(&chunk.Config{
// // 				Path: "/tmp",
// // 			})
// // 			Expect(err).NotTo(HaveOccurred())

// // 			b := &block{
// // 				config:        atomic.Value{},
// // 				state:         atomic.Value{},
// // 				chunks:        chunkChain{chunk},
// // 				primaryIndex:  primaryFactory.New(),
// // 				invertedIndex: invertedFactory.New(),
// // 				vectorIndex:   vectorFactory.New(),
// // 				chunkFactory:  chunkFactory,
// // 				ctx:           context.Background(),
// // 				blockDirpath:  "/tmp",
// // 				chunkDirpath:  "/tmp/chunk",
// // 				indexDirpath:  "/tmp/index",
// // 			}
// // 			b.config.Store(&Config{
// // 				ParentDir: "/tmp",
// // 				Start:     t0,
// // 				Duration:  24 * time.Hour,
// // 			})
// // 			b.state.Store(tt.state)

// // 			// Execute test
// // 			feeds, err := b.Query(context.Background(), tt.queryOpts)

// // 			// Verify results
// // 			if tt.expectedErr != "" {
// // 				Expect(err).To(HaveOccurred())
// // 				Expect(err.Error()).To(ContainSubstring(tt.expectedErr))
// // 			} else {
// // 				Expect(err).NotTo(HaveOccurred())
// // 				Expect(feeds).To(HaveLen(len(tt.expectedFeeds)))
// // 				for i, feed := range feeds {
// // 					Expect(feed.ID).To(Equal(tt.expectedFeeds[i].ID))
// // 					Expect(feed.Labels).To(Equal(tt.expectedFeeds[i].Labels))
// // 					Expect(feed.Time).To(Equal(tt.expectedFeeds[i].Time))
// // 					Expect(feed.Score).To(Equal(tt.expectedFeeds[i].Score))
// // 				}
// // 			}
// // 		})
// // 	}
// // }
