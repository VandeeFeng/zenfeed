package inverted

import (
	"bytes"
	"context"
	"testing"

	. "github.com/onsi/gomega"

	"github.com/vandeefeng/zenfeed/pkg/model"
	"github.com/vandeefeng/zenfeed/pkg/test"
)

func TestAdd(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct {
		existingLabels map[uint64]model.Labels
	}
	type whenDetail struct {
		id     uint64
		labels model.Labels
	}
	type thenExpected struct {
		indexState map[string]map[string]map[uint64]struct{}
	}

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Add Single Label",
			Given:    "An empty index",
			When:     "Adding an item with a single label",
			Then:     "Should index the item correctly",
			GivenDetail: givenDetail{
				existingLabels: map[uint64]model.Labels{},
			},
			WhenDetail: whenDetail{
				id: 1,
				labels: model.Labels{
					{Key: "category", Value: "tech"},
				},
			},
			ThenExpected: thenExpected{
				indexState: map[string]map[string]map[uint64]struct{}{
					"category": {
						"tech": {1: struct{}{}},
					},
				},
			},
		},
		{
			Scenario: "Add Multiple Labels",
			Given:    "An empty index",
			When:     "Adding an item with multiple labels",
			Then:     "Should index all labels correctly",
			GivenDetail: givenDetail{
				existingLabels: map[uint64]model.Labels{
					1: {model.Label{Key: "category", Value: "tech"}},
					3: {model.Label{Key: "category", Value: "news"}},
				},
			},
			WhenDetail: whenDetail{
				id: 2,
				labels: model.Labels{
					{Key: "category", Value: "tech"},
					{Key: "status", Value: "new"},
					{Key: "author", Value: "john"},
				},
			},
			ThenExpected: thenExpected{
				indexState: map[string]map[string]map[uint64]struct{}{
					"category": {
						"tech": {1: struct{}{}, 2: struct{}{}},
						"news": {3: struct{}{}},
					},
					"status": {
						"new": {2: struct{}{}},
					},
					"author": {
						"john": {2: struct{}{}},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// Given.
			idx0, err := NewFactory().New("test", &Config{}, Dependencies{})
			Expect(err).NotTo(HaveOccurred())
			for id, labels := range tt.GivenDetail.existingLabels {
				idx0.Add(context.Background(), id, labels)
			}

			// When.
			idx0.Add(context.Background(), tt.WhenDetail.id, tt.WhenDetail.labels)

			// Then.
			invIdx := idx0.(*idx)
			for label, values := range tt.ThenExpected.indexState {
				Expect(invIdx.m).To(HaveKey(label))
				for value, ids := range values {
					Expect(invIdx.m[label]).To(HaveKey(value))
					for id := range ids {
						Expect(invIdx.m[label][value]).To(HaveKey(id))
					}
				}
			}
		})
	}
}

func TestSearch(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct {
		setupLabels map[uint64]model.Labels
	}
	type whenDetail struct {
		searchLabel string
		eq          bool
		searchValue string
	}
	type thenExpected struct {
		want []uint64
	}

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Search Existing Label-Value",
			Given:    "An index with feeds",
			When:     "Searching for existing label and value",
			Then:     "Should return matching item IDs",
			GivenDetail: givenDetail{
				setupLabels: map[uint64]model.Labels{
					1: {model.Label{Key: "category", Value: "tech"}},
					2: {model.Label{Key: "category", Value: "tech"}},
					3: {model.Label{Key: "category", Value: "news"}},
				},
			},
			WhenDetail: whenDetail{
				searchLabel: "category",
				searchValue: "tech",
				eq:          true,
			},
			ThenExpected: thenExpected{
				want: []uint64{1, 2},
			},
		},
		{
			Scenario: "Search Non-Existing Label",
			Given:    "An index with feeds",
			When:     "Searching for non-existing label",
			Then:     "Should return empty result",
			GivenDetail: givenDetail{
				setupLabels: map[uint64]model.Labels{
					1: {model.Label{Key: "category", Value: "tech"}},
				},
			},
			WhenDetail: whenDetail{
				searchLabel: "invalid",
				searchValue: "value",
				eq:          true,
			},
			ThenExpected: thenExpected{
				want: nil,
			},
		},
		{
			Scenario: "Search Non-Existing Value",
			Given:    "An index with feeds",
			When:     "Searching for existing label but non-existing value",
			Then:     "Should return empty result",
			GivenDetail: givenDetail{
				setupLabels: map[uint64]model.Labels{
					1: {model.Label{Key: "category", Value: "tech"}},
				},
			},
			WhenDetail: whenDetail{
				searchLabel: "category",
				searchValue: "invalid",
				eq:          true,
			},
			ThenExpected: thenExpected{
				want: nil,
			},
		},
		// Not equal tests.
		{
			Scenario: "Search Not Matching Label-Value",
			Given:    "An index with multiple feeds",
			When:     "Searching for feeds not matching a label-value pair",
			Then:     "Should return all feeds except those matching the pair",
			GivenDetail: givenDetail{
				setupLabels: map[uint64]model.Labels{
					1: {model.Label{Key: "category", Value: "tech"}, model.Label{Key: "status", Value: "new"}},
					2: {model.Label{Key: "category", Value: "news"}, model.Label{Key: "status", Value: "old"}},
					3: {model.Label{Key: "category", Value: "tech"}, model.Label{Key: "status", Value: "old"}},
				},
			},
			WhenDetail: whenDetail{
				searchLabel: "category",
				searchValue: "tech",
				eq:          false,
			},
			ThenExpected: thenExpected{
				want: []uint64{2},
			},
		},
		{
			Scenario: "Search Not Matching Non-Existing Label",
			Given:    "An index with feeds",
			When:     "Searching for feeds not matching a non-existing label",
			Then:     "Should return all feeds",
			GivenDetail: givenDetail{
				setupLabels: map[uint64]model.Labels{
					1: {model.Label{Key: "category", Value: "tech"}},
					2: {model.Label{Key: "category", Value: "news"}},
				},
			},
			WhenDetail: whenDetail{
				searchLabel: "invalid",
				searchValue: "value",
				eq:          false,
			},
			ThenExpected: thenExpected{
				want: []uint64{1, 2},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// Given.
			idx, err := NewFactory().New("test", &Config{}, Dependencies{})
			Expect(err).NotTo(HaveOccurred())
			for id, labels := range tt.GivenDetail.setupLabels {
				idx.Add(context.Background(), id, labels)
			}

			// When.
			result := idx.Search(context.Background(), tt.WhenDetail.searchLabel, tt.WhenDetail.eq, tt.WhenDetail.searchValue)

			// Then.
			if tt.ThenExpected.want == nil {
				Expect(result).To(BeEmpty())
			} else {
				resultIDs := make([]uint64, 0, len(result))
				for id := range result {
					resultIDs = append(resultIDs, id)
				}
				Expect(resultIDs).To(ConsistOf(tt.ThenExpected.want))
			}
		})
	}
}

func TestEncodeDecode(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct {
		setupLabels map[uint64]model.Labels
	}
	type whenDetail struct{}
	type thenExpected struct {
		success bool
	}

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Encode and Decode Empty Index",
			Given:    "An empty index",
			When:     "Encoding and decoding",
			Then:     "Should restore empty index correctly",
			GivenDetail: givenDetail{
				setupLabels: map[uint64]model.Labels{},
			},
			WhenDetail: whenDetail{},
			ThenExpected: thenExpected{
				success: true,
			},
		},
		{
			Scenario: "Encode and Decode Index with Data",
			Given:    "An index with feeds",
			When:     "Encoding and decoding",
			Then:     "Should restore all data correctly",
			GivenDetail: givenDetail{
				setupLabels: map[uint64]model.Labels{
					1: {model.Label{Key: "category", Value: "tech"}, model.Label{Key: "status", Value: "new"}},
					2: {model.Label{Key: "category", Value: "news"}, model.Label{Key: "author", Value: "john"}},
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
			for id, labels := range tt.GivenDetail.setupLabels {
				original.Add(context.Background(), id, labels)
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
