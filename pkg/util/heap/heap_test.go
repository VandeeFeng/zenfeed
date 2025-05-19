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

package heap

import (
	"testing"

	. "github.com/onsi/gomega"

	"github.com/vandeefeng/zenfeed/pkg/test"
)

func TestNew(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct{}
	type whenDetail struct {
		data []int
		less func(a, b int) bool
	}
	type thenExpected struct {
		len int
		top int
	}

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Create min heap",
			When:     "creating a new min heap with initial data",
			Then:     "should create a valid heap with elements in min-heap order",
			WhenDetail: whenDetail{
				data: []int{3, 1, 4, 2},
				less: func(a, b int) bool { return a < b },
			},
			ThenExpected: thenExpected{
				len: 4,
				top: 1,
			},
		},
		{
			Scenario: "Create max heap",
			When:     "creating a new max heap with initial data",
			Then:     "should create a valid heap with elements in max-heap order",
			WhenDetail: whenDetail{
				data: []int{3, 1, 4, 2},
				less: func(a, b int) bool { return a > b },
			},
			ThenExpected: thenExpected{
				len: 4,
				top: 4,
			},
		},
		{
			Scenario: "Create empty heap",
			When:     "creating a new heap with no initial data",
			Then:     "should create an empty heap",
			WhenDetail: whenDetail{
				data: []int{},
				less: func(a, b int) bool { return a < b },
			},
			ThenExpected: thenExpected{
				len: 0,
				top: 0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// When.
			h := New(tt.WhenDetail.data, tt.WhenDetail.less)

			// Then.
			Expect(h.Len()).To(Equal(tt.ThenExpected.len))
			Expect(h.Peek()).To(Equal(tt.ThenExpected.top))
		})
	}
}

func TestPushPop(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct {
		heap *Heap[int]
	}
	type whenDetail struct {
		pushValue int
	}
	type thenExpected struct {
		popValue int
		newLen   int
		newTop   int
	}

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Push and pop from min heap",
			Given:    "a min heap with initial values",
			When:     "pushing a new value and then popping",
			Then:     "should maintain heap property and return the minimum value",
			GivenDetail: givenDetail{
				heap: New([]int{3, 5, 7}, func(a, b int) bool { return a < b }),
			},
			WhenDetail: whenDetail{
				pushValue: 2,
			},
			ThenExpected: thenExpected{
				popValue: 2,
				newLen:   3,
				newTop:   3,
			},
		},
		{
			Scenario: "Push and pop from max heap",
			Given:    "a max heap with initial values",
			When:     "pushing a new value and then popping",
			Then:     "should maintain heap property and return the maximum value",
			GivenDetail: givenDetail{
				heap: New([]int{5, 3, 1}, func(a, b int) bool { return a > b }),
			},
			WhenDetail: whenDetail{
				pushValue: 8,
			},
			ThenExpected: thenExpected{
				popValue: 8,
				newLen:   3,
				newTop:   5,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// Given.
			h := tt.GivenDetail.heap

			// When.
			h.Push(tt.WhenDetail.pushValue)
			popValue := h.Pop()

			// Then.
			Expect(popValue).To(Equal(tt.ThenExpected.popValue))
			Expect(h.Len()).To(Equal(tt.ThenExpected.newLen))
			Expect(h.Peek()).To(Equal(tt.ThenExpected.newTop))
		})
	}
}

func TestPeek(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct {
		heap *Heap[int]
	}
	type whenDetail struct{}
	type thenExpected struct {
		peekValue int
		unchanged bool
	}

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Peek from min heap",
			Given:    "a min heap with values",
			When:     "peeking at the top element",
			Then:     "should return the minimum value without modifying the heap",
			GivenDetail: givenDetail{
				heap: New([]int{2, 4, 6}, func(a, b int) bool { return a < b }),
			},
			ThenExpected: thenExpected{
				peekValue: 2,
				unchanged: true,
			},
		},
		{
			Scenario: "Peek from max heap",
			Given:    "a max heap with values",
			When:     "peeking at the top element",
			Then:     "should return the maximum value without modifying the heap",
			GivenDetail: givenDetail{
				heap: New([]int{6, 4, 2}, func(a, b int) bool { return a > b }),
			},
			ThenExpected: thenExpected{
				peekValue: 6,
				unchanged: true,
			},
		},
		{
			Scenario: "Peek from empty heap",
			Given:    "an empty heap",
			When:     "peeking at the top element",
			Then:     "should return zero value",
			GivenDetail: givenDetail{
				heap: New([]int{}, func(a, b int) bool { return a < b }),
			},
			ThenExpected: thenExpected{
				peekValue: 0,
				unchanged: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// Given.
			h := tt.GivenDetail.heap
			originalLen := h.Len()
			originalSlice := make([]int, len(h.Slice()))
			copy(originalSlice, h.Slice())

			// When.
			peekValue := h.Peek()

			// Then.
			Expect(peekValue).To(Equal(tt.ThenExpected.peekValue))
			if tt.ThenExpected.unchanged {
				Expect(h.Len()).To(Equal(originalLen))
				Expect(h.Slice()).To(Equal(originalSlice))
			}
		})
	}
}

func TestHeapOperations(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct {
		initialData []int
		less        func(a, b int) bool
	}
	type whenDetail struct {
		operations []string
		pushValues []int
	}
	type thenExpected struct {
		finalLen    int
		popResults  []int
		finalValues []int
	}

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Multiple operations on min heap",
			Given:    "a min heap with initial values",
			When:     "performing a series of push and pop operations",
			Then:     "should maintain heap property and return values in ascending order",
			GivenDetail: givenDetail{
				initialData: []int{5, 3, 7},
				less:        func(a, b int) bool { return a < b },
			},
			WhenDetail: whenDetail{
				operations: []string{"push", "push", "pop", "pop", "pop", "pop"},
				pushValues: []int{1, 9},
			},
			ThenExpected: thenExpected{
				finalLen:    1,
				popResults:  []int{1, 3, 5, 7},
				finalValues: []int{9},
			},
		},
		{
			Scenario: "Multiple operations on max heap",
			Given:    "a max heap with initial values",
			When:     "performing a series of push and pop operations",
			Then:     "should maintain heap property and return values in descending order",
			GivenDetail: givenDetail{
				initialData: []int{5, 3, 7},
				less:        func(a, b int) bool { return a > b },
			},
			WhenDetail: whenDetail{
				operations: []string{"push", "push", "pop", "pop", "pop", "pop"},
				pushValues: []int{1, 9},
			},
			ThenExpected: thenExpected{
				finalLen:    1,
				popResults:  []int{9, 7, 5, 3},
				finalValues: []int{1},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// Given.
			h := New(tt.GivenDetail.initialData, tt.GivenDetail.less)

			// When.
			pushIndex := 0
			popResults := []int{}

			for _, op := range tt.WhenDetail.operations {
				switch op {
				case "push":
					if pushIndex < len(tt.WhenDetail.pushValues) {
						h.Push(tt.WhenDetail.pushValues[pushIndex])
						pushIndex++
					}
				case "pop":
					if h.Len() > 0 {
						popResults = append(popResults, h.Pop())
					}
				}
			}

			// Then.
			Expect(h.Len()).To(Equal(tt.ThenExpected.finalLen))
			Expect(popResults).To(Equal(tt.ThenExpected.popResults))
			Expect(h.Slice()).To(Equal(tt.ThenExpected.finalValues))
		})
	}
}
