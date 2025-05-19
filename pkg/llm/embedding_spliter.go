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
	"math"
	"slices"

	"github.com/vandeefeng/zenfeed/pkg/model"
)

type embeddingSpliter interface {
	Split(ls model.Labels) ([]model.Labels, error)
}

func newEmbeddingSpliter(maxLabelValueTokens, overlapTokens int) embeddingSpliter {
	if maxLabelValueTokens <= 0 {
		maxLabelValueTokens = 1024
	}
	if overlapTokens <= 0 {
		overlapTokens = 64
	}
	if overlapTokens > maxLabelValueTokens {
		overlapTokens = maxLabelValueTokens / 10
	}

	return &embeddingSpliterImpl{maxLabelValueTokens: maxLabelValueTokens, overlapTokens: overlapTokens}
}

type embeddingSpliterImpl struct {
	maxLabelValueTokens int
	overlapTokens       int
}

func (e *embeddingSpliterImpl) Split(ls model.Labels) ([]model.Labels, error) {
	var (
		short      = make(model.Labels, 0, len(ls))
		long       = make(model.Labels, 0, 1)
		longTokens = make([]int, 0, 1)
	)
	for _, l := range ls {
		tokens := e.estimateTokens(l.Value)
		if tokens <= e.maxLabelValueTokens {
			short = append(short, l)
		} else {
			long = append(long, l)
			longTokens = append(longTokens, tokens)
		}
	}
	if len(long) == 0 {
		return []model.Labels{ls}, nil
	}

	var (
		common = short
		splits = make([]model.Labels, 0, len(long)*2)
	)
	for i := range long {
		parts := e.split(long[i].Value, longTokens[i])
		for _, p := range parts {
			com := slices.Clone(common)
			s := append(com, model.Label{Key: long[i].Key, Value: p})
			splits = append(splits, s)
		}
	}

	return splits, nil
}

func (e *embeddingSpliterImpl) split(value string, tokens int) []string {
	var (
		results = make([]string, 0)
		chars   = []rune(value)
	)

	// Estimate the number of characters per token
	avgCharsPerToken := float64(len(chars)) / float64(tokens)
	// Calculate the approximate number of characters corresponding to maxLabelValueTokens tokens.
	charsPerSegment := int(float64(e.maxLabelValueTokens) * avgCharsPerToken)

	// The number of characters corresponding to a fixed overlap of 64 tokens.
	overlapChars := int(float64(e.overlapTokens) * avgCharsPerToken)

	// Actual step length = segment length - overlap.
	charStep := charsPerSegment - overlapChars

	for start := 0; start < len(chars); {
		end := min(start+charsPerSegment, len(chars))

		segment := string(chars[start:end])
		results = append(results, segment)

		if end == len(chars) {
			break
		}
		start += charStep
	}

	return results
}

func (e *embeddingSpliterImpl) estimateTokens(text string) int {
	latinChars := 0
	otherChars := 0

	for _, r := range text {
		if r <= 127 {
			latinChars++
		} else {
			otherChars++
		}
	}

	// Rough estimate:
	// - English and punctuation: about 0.25 tokens/char (4 characters ≈ 1 token).
	// - Chinese and other non-Latin characters: about 1.5 tokens/char.
	return int(math.Round(float64(latinChars)/4 + float64(otherChars)*3/2))
}
