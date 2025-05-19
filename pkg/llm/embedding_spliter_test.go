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

package llm

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/vandeefeng/zenfeed/pkg/model"
	"github.com/vandeefeng/zenfeed/pkg/test"
)

func TestEmbeddingSpliter_Split(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct {
		maxLabelValueTokens int
		overlapTokens       int
	}
	type whenDetail struct {
		labels model.Labels
	}
	type thenExpected struct {
		splits []model.Labels
		err    string
	}

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Split labels with all short values",
			Given:    "an embedding spliter with max token limit",
			When:     "splitting labels with all values under token limit",
			Then:     "should return original labels as single split",
			GivenDetail: givenDetail{
				maxLabelValueTokens: 1024,
			},
			WhenDetail: whenDetail{
				labels: model.Labels{
					{Key: "title", Value: "Short title"},
					{Key: "description", Value: "Short description"},
				},
			},
			ThenExpected: thenExpected{
				splits: []model.Labels{
					{
						{Key: "title", Value: "Short title"},
						{Key: "description", Value: "Short description"},
					},
				},
			},
		},
		{
			Scenario: "Split labels with one long value",
			Given:    "an embedding spliter with max token limit",
			When:     "splitting labels with one value exceeding token limit",
			Then:     "should split the long value and combine with common labels",
			GivenDetail: givenDetail{
				maxLabelValueTokens: 10, // Small limit to force splitting.
				overlapTokens:       1,
			},
			WhenDetail: whenDetail{
				labels: model.Labels{
					{Key: "title", Value: "Short title"},
					{Key: "content", Value: "This is a long content that exceeds the token limit and needs to be split into multiple parts"},
				},
			},
			ThenExpected: thenExpected{
				splits: []model.Labels{
					{
						{Key: "title", Value: "Short title"},
						{Key: "content", Value: "This is a long content that exceeds the "},
					},
					{
						{Key: "title", Value: "Short title"},
						{Key: "content", Value: "the token limit and needs to be split in"},
					},
					{
						{Key: "title", Value: "Short title"},
						{Key: "content", Value: "t into multiple parts"},
					},
				},
			},
		},
		{
			Scenario: "Handle non-Latin characters",
			Given:    "an embedding spliter with max token limit",
			When:     "splitting labels with non-Latin characters",
			Then:     "should correctly estimate tokens and split accordingly",
			GivenDetail: givenDetail{
				maxLabelValueTokens: 10, // Small limit to force splitting.
				overlapTokens:       2,
			},
			WhenDetail: whenDetail{
				labels: model.Labels{
					{Key: "title", Value: "Short title"},
					{Key: "content", Value: "中文内容需要被分割因为它超过了令牌限制"}, // Chinese content that needs to be split.
				},
			},
			ThenExpected: thenExpected{
				splits: []model.Labels{
					{
						{Key: "title", Value: "Short title"},
						{Key: "content", Value: "中文内容需要"},
					},
					{
						{Key: "title", Value: "Short title"},
						{Key: "content", Value: "要被分割因为"},
					},
					{
						{Key: "title", Value: "Short title"},
						{Key: "content", Value: "为它超过了令"},
					},
					{
						{Key: "title", Value: "Short title"},
						{Key: "content", Value: "令牌限制"},
					},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// Given.
			spliter := newEmbeddingSpliter(tt.GivenDetail.maxLabelValueTokens, tt.GivenDetail.overlapTokens)

			// When.
			splits, err := spliter.Split(tt.WhenDetail.labels)

			// Then.
			if tt.ThenExpected.err != "" {
				Expect(err).NotTo(BeNil())
				Expect(err.Error()).To(ContainSubstring(tt.ThenExpected.err))
			} else {
				Expect(err).To(BeNil())
				Expect(len(splits)).To(Equal(len(tt.ThenExpected.splits)))

				for i, expectedSplit := range tt.ThenExpected.splits {
					Expect(splits[i]).To(Equal(expectedSplit))
				}
			}
		})
	}
}
