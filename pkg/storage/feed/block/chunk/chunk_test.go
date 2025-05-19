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

package chunk

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/pkg/errors"

	"github.com/vandeefeng/zenfeed/pkg/model"
	"github.com/vandeefeng/zenfeed/pkg/test"
)

func TestNew(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct {
		path            string
		readonlyAtFirst bool
		setupFeeds      []*Feed
	}
	type whenDetail struct{}
	type thenExpected struct {
		count uint32
		err   string
	}

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Create New Chunk File",
			Given:    "A valid non-existing file path",
			When:     "Creating a new chunk file",
			Then:     "Should return a valid File instance with count 0",
			GivenDetail: givenDetail{
				readonlyAtFirst: false,
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				count: 0,
			},
		},
		{
			Scenario: "Open Existing Chunk File",
			Given:    "A valid existing chunk file with data",
			When:     "Opening the file in readonly mode",
			Then:     "Should return a valid File instance with correct count",
			GivenDetail: givenDetail{
				readonlyAtFirst: true,
				setupFeeds: []*Feed{
					createTestFeed(1),
					createTestFeed(2),
					createTestFeed(3),
				},
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				count: 3,
			},
		},
		{
			Scenario: "Invalid Configuration",
			Given:    "An invalid configuration with empty path",
			When:     "Creating a new chunk file",
			Then:     "Should return an error",
			GivenDetail: givenDetail{
				path: "", // Empty path
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				err: "validate config: path is required",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// Given.
			if tt.GivenDetail.path == "" && tt.ThenExpected.err == "" {
				tt.GivenDetail.path = createTempFile(t)
				defer cleanupTempFile(tt.GivenDetail.path)
			}

			if len(tt.GivenDetail.setupFeeds) > 0 {
				initialFile, err := new("test", &Config{
					Path:            tt.GivenDetail.path,
					ReadonlyAtFirst: false,
				}, Dependencies{})
				Expect(err).NotTo(HaveOccurred())
				err = initialFile.Append(context.Background(), tt.GivenDetail.setupFeeds, nil)
				Expect(err).NotTo(HaveOccurred())
				initialFile.Close()
			}

			// When.
			file, err := new("test", &Config{
				Path:            tt.GivenDetail.path,
				ReadonlyAtFirst: tt.GivenDetail.readonlyAtFirst,
			}, Dependencies{})

			// Then.
			if tt.ThenExpected.err != "" {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(tt.ThenExpected.err))
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(file).NotTo(BeNil())
				Expect(file.Count(context.Background())).To(Equal(tt.ThenExpected.count))
				file.Close()
			}
		})
	}
}

func TestFileModeSwitching(t *testing.T) {
	RegisterTestingT(t)

	tests := []struct {
		scenario      string
		given         string
		when          string
		then          string
		initialMode   bool // true for readonly
		expectedError string
	}{
		{
			scenario:      "ReadWrite to ReadOnly Switch",
			given:         "a read-write mode chunk file",
			when:          "calling EnsureReadonly()",
			then:          "file should switch to read-only mode",
			initialMode:   false,
			expectedError: "",
		},
		{
			scenario:      "Already ReadOnly",
			given:         "a read-only mode chunk file",
			when:          "calling EnsureReadonly()",
			then:          "operation should return quickly",
			initialMode:   true,
			expectedError: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.scenario, func(t *testing.T) {
			// Setup
			path := createTempFile(t)
			defer cleanupTempFile(path)

			// Create initial file
			initialConfig := Config{
				Path:            path,
				ReadonlyAtFirst: false,
			}
			initialFile, err := new("test", &initialConfig, Dependencies{})
			Expect(err).NotTo(HaveOccurred())
			initialFile.Close()

			// Open file with specified mode
			config := Config{
				Path:            path,
				ReadonlyAtFirst: tt.initialMode,
			}
			f, err := new("test", &config, Dependencies{})
			Expect(err).NotTo(HaveOccurred())
			defer f.Close()

			// Execute
			err = f.EnsureReadonly(context.Background())

			// Verify
			if tt.expectedError != "" {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(tt.expectedError))
			} else {
				Expect(err).NotTo(HaveOccurred())
				// Verify it's now in readonly mode by attempting an append
				appendErr := f.Append(context.Background(), []*Feed{createTestFeed(1)}, nil)
				Expect(appendErr).To(HaveOccurred())
				Expect(appendErr.Error()).To(ContainSubstring("file is readonly"))
			}
		})
	}
}

func TestAppend(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct {
		readonly bool
	}
	type whenDetail struct {
		appendFeeds []*Feed
	}
	type thenExpected struct {
		count uint32
		err   string
	}

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Append Single Feed",
			Given:    "A read-write mode chunk file",
			When:     "Adding a single feed",
			Then:     "Should successfully write the feed",
			GivenDetail: givenDetail{
				readonly: false,
			},
			WhenDetail: whenDetail{
				appendFeeds: []*Feed{createTestFeed(1)},
			},
			ThenExpected: thenExpected{
				count: 1,
			},
		},
		{
			Scenario: "Batch Append Multiple Feeds",
			Given:    "A read-write mode chunk file",
			When:     "Adding multiple feeds at once",
			Then:     "Should write all feeds as a single transaction",
			GivenDetail: givenDetail{
				readonly: false,
			},
			WhenDetail: whenDetail{
				appendFeeds: []*Feed{
					createTestFeed(1),
					createTestFeed(2),
					createTestFeed(3),
				},
			},
			ThenExpected: thenExpected{
				count: 3,
			},
		},
		{
			Scenario: "Append in ReadOnly Mode",
			Given:    "A read-only mode chunk file",
			When:     "Attempting to add a feed",
			Then:     "Should fail with readonly error",
			GivenDetail: givenDetail{
				readonly: true,
			},
			WhenDetail: whenDetail{
				appendFeeds: []*Feed{createTestFeed(1)},
			},
			ThenExpected: thenExpected{
				err: "file is readonly",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// Given.
			path := createTempFile(t)
			defer cleanupTempFile(path)

			if tt.GivenDetail.readonly {
				// Create and close initial file for readonly test.
				rwFile, err := new("test", &Config{Path: path}, Dependencies{})
				Expect(err).NotTo(HaveOccurred())
				rwFile.Close()
			}

			f, err := new("test", &Config{
				Path:            path,
				ReadonlyAtFirst: tt.GivenDetail.readonly,
			}, Dependencies{})
			Expect(err).NotTo(HaveOccurred())
			defer f.Close()

			// When.
			var offsets []uint64
			err = f.Append(context.Background(), tt.WhenDetail.appendFeeds, func(_ *Feed, offset uint64) error {
				offsets = append(offsets, offset)

				return nil
			})

			// Then.
			if tt.ThenExpected.err != "" {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(tt.ThenExpected.err))
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(f.Count(context.Background())).To(Equal(tt.ThenExpected.count))

				// Verify each feed can be read back.
				for i, offset := range offsets {
					feed, readErr := f.Read(context.Background(), offset)
					Expect(readErr).NotTo(HaveOccurred())
					Expect(feed.ID).To(Equal(tt.WhenDetail.appendFeeds[i].ID))
				}
			}
		})
	}
}

func TestRead(t *testing.T) {
	RegisterTestingT(t)

	tests := []struct {
		scenario    string
		given       string
		when        string
		then        string
		readonly    bool
		setupFeeds  []*Feed
		readOffset  uint64
		expectedErr string
	}{
		{
			scenario:    "Read from Valid Offset",
			given:       "a chunk file with feeds",
			when:        "reading with a valid offset",
			then:        "should return the correct feed",
			readonly:    false,
			setupFeeds:  []*Feed{createTestFeed(1)},
			readOffset:  uint64(dataStart), // Will be adjusted in the test
			expectedErr: "",
		},
		{
			scenario:    "Read from ReadOnly Mode",
			given:       "a read-only chunk file with feeds",
			when:        "reading with a valid offset",
			then:        "should return the correct feed using mmap",
			readonly:    true,
			setupFeeds:  []*Feed{createTestFeed(2)},
			readOffset:  uint64(dataStart), // Will be adjusted in the test
			expectedErr: "",
		},
		{
			scenario:    "Read with Small Offset",
			given:       "a chunk file with feeds",
			when:        "reading with an offset smaller than dataStart",
			then:        "should return 'offset too small' error",
			readonly:    false,
			setupFeeds:  []*Feed{createTestFeed(3)},
			readOffset:  uint64(dataStart - 1),
			expectedErr: "offset too small",
		},
		{
			scenario:    "Read with Large Offset",
			given:       "a chunk file with feeds",
			when:        "reading with an offset larger than appendOffset",
			then:        "should return 'offset too large' error",
			readonly:    false,
			setupFeeds:  []*Feed{createTestFeed(4)},
			readOffset:  999999, // Definitely beyond appendOffset
			expectedErr: "offset too large",
		},
	}

	for _, tt := range tests {
		t.Run(tt.scenario, func(t *testing.T) {
			// Setup
			path := createTempFile(t)
			defer cleanupTempFile(path)

			// Create and populate initial file
			initialConfig := Config{
				Path:            path,
				ReadonlyAtFirst: false,
			}
			initialFile, err := new("test", &initialConfig, Dependencies{})
			Expect(err).NotTo(HaveOccurred())

			var validOffset uint64
			if len(tt.setupFeeds) > 0 {
				// Track the first offset for later reading
				var firstOffset uint64
				err = initialFile.Append(context.Background(), tt.setupFeeds, func(_ *Feed, offset uint64) error {
					if firstOffset == 0 {
						firstOffset = offset
					}

					return nil
				})
				Expect(err).NotTo(HaveOccurred())
				validOffset = firstOffset
			}
			initialFile.Close()

			// Reopen with specified mode
			config := Config{
				Path:            path,
				ReadonlyAtFirst: tt.readonly,
			}
			f, err := new("test", &config, Dependencies{})
			Expect(err).NotTo(HaveOccurred())
			defer f.Close()

			// Use valid offset if needed
			readOffset := tt.readOffset
			if readOffset == uint64(dataStart) && validOffset > 0 {
				readOffset = validOffset
			}

			// Execute
			feed, err := f.Read(context.Background(), readOffset)

			// Verify
			if tt.expectedErr != "" {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(tt.expectedErr))
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(feed).NotTo(BeNil())
				Expect(feed.ID).To(Equal(tt.setupFeeds[0].ID))
			}
		})
	}
}

func TestRange(t *testing.T) {
	RegisterTestingT(t)

	tests := []struct {
		scenario      string
		given         string
		when          string
		then          string
		readonly      bool
		setupFeeds    []*Feed
		earlyExit     bool
		expectedCount int
		expectedErr   string
	}{
		{
			scenario: "Range All Feeds",
			given:    "a chunk file with multiple feeds",
			when:     "calling Range()",
			then:     "iterator should visit each feed in sequence",
			readonly: false,
			setupFeeds: []*Feed{
				createTestFeed(1),
				createTestFeed(2),
				createTestFeed(3),
			},
			earlyExit:     false,
			expectedCount: 3,
			expectedErr:   "",
		},
		{
			scenario: "Range with Early Exit",
			given:    "a chunk file with multiple feeds",
			when:     "calling Range() and returning an error from iterator",
			then:     "range should stop and return that error",
			readonly: false,
			setupFeeds: []*Feed{
				createTestFeed(4),
				createTestFeed(5),
				createTestFeed(6),
			},
			earlyExit:     true,
			expectedCount: 1, // Should stop after first feed
			expectedErr:   "early exit",
		},
		{
			scenario: "Range in ReadOnly Mode",
			given:    "a read-only chunk file with feeds",
			when:     "calling Range()",
			then:     "should use mmap and correctly visit all feeds",
			readonly: true,
			setupFeeds: []*Feed{
				createTestFeed(7),
				createTestFeed(8),
			},
			earlyExit:     false,
			expectedCount: 2,
			expectedErr:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.scenario, func(t *testing.T) {
			// Setup
			path := createTempFile(t)
			defer cleanupTempFile(path)

			// Create and populate initial file
			initialConfig := Config{
				Path:            path,
				ReadonlyAtFirst: false,
			}
			initialFile, err := new("test", &initialConfig, Dependencies{})
			Expect(err).NotTo(HaveOccurred())

			if len(tt.setupFeeds) > 0 {
				err = initialFile.Append(context.Background(), tt.setupFeeds, nil)
				Expect(err).NotTo(HaveOccurred())
			}
			initialFile.Close()

			// Reopen with specified mode
			config := Config{
				Path:            path,
				ReadonlyAtFirst: tt.readonly,
			}
			f, err := new("test", &config, Dependencies{})
			Expect(err).NotTo(HaveOccurred())
			defer f.Close()

			// Execute
			visitCount := 0
			err = f.Range(context.Background(), func(feed *Feed, offset uint64) (err error) {
				visitCount++
				if tt.earlyExit && visitCount == 1 {
					return errors.New("early exit")
				}
				return nil
			})

			// Verify
			if tt.expectedErr != "" {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(tt.expectedErr))
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(visitCount).To(Equal(tt.expectedCount))
		})
	}
}

func createTempFile(t *testing.T) string {
	dir, err := os.MkdirTemp("", "chunk-test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	return filepath.Join(dir, "test.chunk")
}

func cleanupTempFile(path string) {
	os.RemoveAll(filepath.Dir(path))
}

func createTestFeed(id uint64) *Feed {
	return &Feed{
		Feed: &model.Feed{
			ID:     id,
			Labels: model.Labels{model.Label{Key: "test", Value: "value"}},
			Time:   time.Now(),
		},
		Vectors: [][]float32{
			{1.0, 2.0, 3.0},
			{4.0, 5.0, 6.0},
		},
	}
}
