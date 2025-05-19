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

package vector

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/vandeefeng/zenfeed/pkg/test"
)

func TestQuantizeDequantize(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct{}
	type whenDetail struct {
		vector []float32
	}
	type thenExpected struct {
		maxError float32
	}

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Quantize and dequantize unit vector",
			When:     "quantizing and then dequantizing a vector with values between 0 and 1",
			Then:     "should return vector close to the original with small error",
			WhenDetail: whenDetail{
				vector: []float32{0.1, 0.5, 0.9, 0.3},
			},
			ThenExpected: thenExpected{
				maxError: 0.01,
			},
		},
		{
			Scenario: "Quantize and dequantize vector with negative values",
			When:     "quantizing and then dequantizing a vector with negative values",
			Then:     "should return vector close to the original with small error",
			WhenDetail: whenDetail{
				vector: []float32{-1.0, -0.5, 0.0, 0.5, 1.0},
			},
			ThenExpected: thenExpected{
				maxError: 0.01,
			},
		},
		{
			Scenario: "Quantize and dequantize large range vector",
			When:     "quantizing and then dequantizing a vector with large range of values",
			Then:     "should return vector close to the original with acceptable error",
			WhenDetail: whenDetail{
				vector: []float32{-100, -50, 0, 50, 100},
			},
			ThenExpected: thenExpected{
				maxError: 1.5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// When.
			quantized, min, scale := Quantize(tt.WhenDetail.vector)
			dequantized := Dequantize(quantized, min, scale)

			// Then.
			Expect(len(dequantized)).To(Equal(len(tt.WhenDetail.vector)))

			maxError := float32(0)

			for i := range tt.WhenDetail.vector {
				error := float32(0)
				if tt.WhenDetail.vector[i] > dequantized[i] {
					error = tt.WhenDetail.vector[i] - dequantized[i]
				} else {
					error = dequantized[i] - tt.WhenDetail.vector[i]
				}
				if error > maxError {
					maxError = error
				}
			}

			Expect(maxError).To(BeNumerically("<=", tt.ThenExpected.maxError))
		})
	}
}
