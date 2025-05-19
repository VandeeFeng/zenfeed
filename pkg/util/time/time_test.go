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

package time

import (
	"testing"
	"time"

	. "github.com/onsi/gomega"

	"github.com/vandeefeng/zenfeed/pkg/test"
)

func TestSetLocation(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct{}
	type whenDetail struct {
		locationName string
	}
	type thenExpected struct {
		shouldError bool
		errorMsg    string
	}

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Set valid location",
			When:     "calling SetLocation with a valid location name",
			Then:     "should set the location without error",
			WhenDetail: whenDetail{
				locationName: "UTC",
			},
			ThenExpected: thenExpected{
				shouldError: false,
			},
		},
		{
			Scenario: "Set invalid location",
			When:     "calling SetLocation with an invalid location name",
			Then:     "should return an error",
			WhenDetail: whenDetail{
				locationName: "InvalidLocationName",
			},
			ThenExpected: thenExpected{
				shouldError: true,
				errorMsg:    "load location",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// When.
			err := SetLocation(tt.WhenDetail.locationName)

			// Then.
			if tt.ThenExpected.shouldError {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(tt.ThenExpected.errorMsg))
			} else {
				Expect(err).NotTo(HaveOccurred())
				
				loc := time.Local
				Expect(loc.String()).To(Equal(tt.WhenDetail.locationName))
			}
		})
	}
}

func TestInRange(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct{}
	type whenDetail struct {
		t     time.Time
		start time.Time
		end   time.Time
	}
	type thenExpected struct {
		result bool
	}

	baseTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	before := baseTime.Add(-1 * time.Hour)
	after := baseTime.Add(1 * time.Hour)
	muchAfter := baseTime.Add(2 * time.Hour)

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Time is in range",
			When:     "a time that is between start and end, checking if time is in range",
			Then:     "should return true",
			WhenDetail: whenDetail{
				t:     baseTime,
				start: before,
				end:   after,
			},
			ThenExpected: thenExpected{
				result: true,
			},
		},
		{
			Scenario: "Time equals start",
			When:     "a time that equals the start time, checking if time is in range",
			Then:     "should return false",
			WhenDetail: whenDetail{
				t:     before,
				start: before,
				end:   after,
			},
			ThenExpected: thenExpected{
				result: false,
			},
		},
		{
			Scenario: "Time equals end",
			When:     "a time that equals the end time, checking if time is in range",
			Then:     "should return false",
			WhenDetail: whenDetail{
				t:     after,
				start: before,
				end:   after,
			},
			ThenExpected: thenExpected{
				result: false,
			},
		},
		{
			Scenario: "Time is before range",
			When:     "a time that is before the start time, checking if time is in range",
			Then:     "should return false",
			WhenDetail: whenDetail{
				t:     before,
				start: baseTime,
				end:   after,
			},
			ThenExpected: thenExpected{
				result: false,
			},
		},
		{
			Scenario: "Time is after range",
			When:     "a time that is after the end time, checking if time is in range",
			Then:     "should return false",
			WhenDetail: whenDetail{
				t:     muchAfter,
				start: before,
				end:   after,
			},
			ThenExpected: thenExpected{
				result: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// When.
			result := InRange(tt.WhenDetail.t, tt.WhenDetail.start, tt.WhenDetail.end)

			// Then.
			Expect(result).To(Equal(tt.ThenExpected.result))
		})
	}
}

func TestFormat(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct{}
	type whenDetail struct {
		t time.Time
	}
	type thenExpected struct {
		formatted string
	}

	testTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Format time",
			When:     "formatting the time",
			Then:     "should return RFC3339 formatted string",
			WhenDetail: whenDetail{
				t: testTime,
			},
			ThenExpected: thenExpected{
				formatted: "2023-01-01T12:00:00Z",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// When.
			result := Format(tt.WhenDetail.t)

			// Then.
			Expect(result).To(Equal(tt.ThenExpected.formatted))
		})
	}
}

func TestParse(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct{}
	type whenDetail struct {
		timeStr string
	}
	type thenExpected struct {
		time        time.Time
		shouldError bool
	}

	validTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Parse valid time string",
			When:     "parsing the valid RFC3339 string",
			Then:     "should return the correct time value",
			WhenDetail: whenDetail{
				timeStr: "2023-01-01T12:00:00Z",
			},
			ThenExpected: thenExpected{
				time:        validTime,
				shouldError: false,
			},
		},
		{
			Scenario: "Parse invalid time string",
			When:     "parsing the invalid time string",
			Then:     "should return an error",
			WhenDetail: whenDetail{
				timeStr: "invalid-time-string",
			},
			ThenExpected: thenExpected{
				shouldError: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// When.
			result, err := Parse(tt.WhenDetail.timeStr)

			// Then.
			if tt.ThenExpected.shouldError {
				Expect(err).To(HaveOccurred())
			} else {
				Expect(err).NotTo(HaveOccurred())
				Expect(result).To(Equal(tt.ThenExpected.time))
			}
		})
	}
}

func TestMustParse(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct{}
	type whenDetail struct {
		timeStr string
	}
	type thenExpected struct {
		time        time.Time
		shouldPanic bool
	}

	validTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "MustParse with valid time string",
			When:     "calling MustParse with a valid time string",
			Then:     "should return the correct time value without panic",
			WhenDetail: whenDetail{
				timeStr: "2023-01-01T12:00:00Z",
			},
			ThenExpected: thenExpected{
				time:        validTime,
				shouldPanic: false,
			},
		},
		{
			Scenario: "MustParse with invalid time string",
			When:     "calling MustParse with an invalid time string",
			Then:     "should panic",
			WhenDetail: whenDetail{
				timeStr: "invalid-time-string",
			},
			ThenExpected: thenExpected{
				shouldPanic: true,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// When & Then.
			if tt.ThenExpected.shouldPanic {
				Expect(func() { MustParse(tt.WhenDetail.timeStr) }).To(Panic())
			} else {
				result := MustParse(tt.WhenDetail.timeStr)
				Expect(result).To(Equal(tt.ThenExpected.time))
			}
		})
	}
}
