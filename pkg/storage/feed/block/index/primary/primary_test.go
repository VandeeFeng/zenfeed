package primary

import (
	"bytes"
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/vandeefeng/zenfeed/pkg/test"
)

func TestAdd(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct {
		existingItems map[uint64]FeedRef
	}
	type whenDetail struct {
		id   uint64
		item FeedRef
	}
	type thenExpected struct {
		items map[uint64]FeedRef
	}

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Add Single Feed",
			Given:    "An index with existing item",
			When:     "Adding a single item",
			Then:     "Should store the item correctly",
			GivenDetail: givenDetail{
				existingItems: map[uint64]FeedRef{
					0: {Chunk: 0, Offset: 0},
				},
			},
			WhenDetail: whenDetail{
				id:   1,
				item: FeedRef{Chunk: 1, Offset: 100},
			},
			ThenExpected: thenExpected{
				items: map[uint64]FeedRef{
					0: {Chunk: 0, Offset: 0},
					1: {Chunk: 1, Offset: 100},
				},
			},
		},
		{
			Scenario: "Update Existing Feed",
			Given:    "An index with existing item",
			When:     "Adding item with same ID",
			Then:     "Should update the item reference",
			GivenDetail: givenDetail{
				existingItems: map[uint64]FeedRef{
					1: {Chunk: 1, Offset: 100},
				},
			},
			WhenDetail: whenDetail{
				id:   1,
				item: FeedRef{Chunk: 2, Offset: 200},
			},
			ThenExpected: thenExpected{
				items: map[uint64]FeedRef{
					1: {Chunk: 2, Offset: 200},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// Given.
			idx0, err := NewFactory().New("test", &Config{}, Dependencies{})
			Expect(err).NotTo(HaveOccurred())
			for id, item := range tt.GivenDetail.existingItems {
				idx0.Add(context.Background(), id, item)
			}

			// When.
			idx0.Add(context.Background(), tt.WhenDetail.id, tt.WhenDetail.item)

			// Then.
			primIdx := idx0.(*idx)
			for id, expected := range tt.ThenExpected.items {
				Expect(primIdx.m).To(HaveKey(id))
				Expect(primIdx.m[id]).To(Equal(expected))
			}
		})
	}
}

func TestSearch(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct {
		feeds map[uint64]FeedRef
	}
	type whenDetail struct {
		searchID uint64
	}
	type thenExpected struct {
		feedRef FeedRef
		found   bool
	}

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Search Existing Feed",
			Given:    "An index with feeds",
			When:     "Searching for existing ID",
			Then:     "Should return correct FeedRef",
			GivenDetail: givenDetail{
				feeds: map[uint64]FeedRef{
					1: {Chunk: 1, Offset: 100},
					2: {Chunk: 2, Offset: 200},
				},
			},
			WhenDetail: whenDetail{
				searchID: 1,
			},
			ThenExpected: thenExpected{
				feedRef: FeedRef{Chunk: 1, Offset: 100},
				found:   true,
			},
		},
		{
			Scenario: "Search Non-Existing Feed",
			Given:    "An index with feeds",
			When:     "Searching for non-existing ID",
			Then:     "Should return empty FeedRef",
			GivenDetail: givenDetail{
				feeds: map[uint64]FeedRef{
					1: {Chunk: 1, Offset: 100},
				},
			},
			WhenDetail: whenDetail{
				searchID: 2,
			},
			ThenExpected: thenExpected{
				feedRef: FeedRef{},
				found:   false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// Given.
			idx, err := NewFactory().New("test", &Config{}, Dependencies{})
			Expect(err).NotTo(HaveOccurred())
			for id, item := range tt.GivenDetail.feeds {
				idx.Add(context.Background(), id, item)
			}

			// When.
			result, ok := idx.Search(context.Background(), tt.WhenDetail.searchID)

			// Then.
			Expect(result).To(Equal(tt.ThenExpected.feedRef))
			Expect(ok).To(Equal(tt.ThenExpected.found))
		})
	}
}

func TestEncodeDecode(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct {
		feeds map[uint64]FeedRef
	}
	type whenDetail struct{}
	type thenExpected struct {
		success bool
	}

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Encode and Decode Index with Data",
			Given:    "An index with feeds",
			When:     "Encoding and decoding",
			Then:     "Should restore all data correctly",
			GivenDetail: givenDetail{
				feeds: map[uint64]FeedRef{
					1: {Chunk: 1, Offset: 100, Time: time.Now()},
					2: {Chunk: 2, Offset: 200, Time: time.Now()},
				},
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				success: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// Given.
			original, err := NewFactory().New("test", &Config{}, Dependencies{})
			Expect(err).NotTo(HaveOccurred())
			for id, item := range tt.GivenDetail.feeds {
				original.Add(context.Background(), id, item)
			}

			// When.
			var buf bytes.Buffer
			err = original.EncodeTo(context.Background(), &buf)
			Expect(err).NotTo(HaveOccurred())

			decoded, err := NewFactory().New("test", &Config{}, Dependencies{})
			Expect(err).NotTo(HaveOccurred())
			err = decoded.DecodeFrom(context.Background(), &buf)
			Expect(err).NotTo(HaveOccurred())

			// Then.
			origIdx := original.(*idx)
			decodedIdx := decoded.(*idx)
			Expect(decodedIdx.m).To(Equal(origIdx.m))
		})
	}
}
