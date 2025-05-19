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

package retry

import (
	"context"
	"testing"
	"time"

	. "github.com/onsi/gomega"
	"github.com/pkg/errors"
	"k8s.io/utils/ptr"

	"github.com/vandeefeng/zenfeed/pkg/test"
)

func TestBackoff(t *testing.T) {
	RegisterTestingT(t)

	type givenDetail struct{}
	type whenDetail struct {
		operation   func() error
		opts        *Options
		cancelAfter time.Duration
	}
	type thenExpected struct {
		shouldError    bool
		errorContains  string
		attemptsNeeded int
	}

	tests := []test.Case[givenDetail, whenDetail, thenExpected]{
		{
			Scenario: "Operation succeeds on first attempt",
			When:     "calling Backoff with the operation that succeeds immediately",
			Then:     "should return nil error",
			WhenDetail: whenDetail{
				operation: func() error {
					return nil
				},
				opts: nil,
			},
			ThenExpected: thenExpected{
				shouldError:    false,
				attemptsNeeded: 1,
			},
		},
		{
			Scenario: "Operation succeeds after retries",
			When:     "calling Backoff with the operation that fails initially but succeeds after retries",
			Then:     "should return nil error after successful retry",
			WhenDetail: whenDetail{
				operation: createFailingThenSucceedingOperation(2),
				opts: &Options{
					MinInterval: 10 * time.Millisecond,
					MaxInterval: 50 * time.Millisecond,
					MaxAttempts: ptr.To(5),
				},
			},
			ThenExpected: thenExpected{
				shouldError:    false,
				attemptsNeeded: 3,
			},
		},
		{
			Scenario: "Operation fails all attempts",
			When:     "calling Backoff with the operation that always fails",
			Then:     "should return error after max attempts",
			WhenDetail: whenDetail{
				operation: func() error {
					return errors.New("persistent error")
				},
				opts: &Options{
					MinInterval: 10 * time.Millisecond,
					MaxInterval: 50 * time.Millisecond,
					MaxAttempts: ptr.To(3),
				},
			},
			ThenExpected: thenExpected{
				shouldError:    true,
				errorContains:  "max attempts reached",
				attemptsNeeded: 3,
			},
		},
		{
			Scenario: "Context cancellation",
			When:     "calling Backoff with an operation that takes time",
			Then:     "should return context error",
			WhenDetail: whenDetail{
				operation: func() error {
					return errors.New("operation error")
				},
				opts: &Options{
					MinInterval: 100 * time.Millisecond,
					MaxInterval: 200 * time.Millisecond,
					MaxAttempts: ptr.To(10),
				},
				cancelAfter: 50 * time.Millisecond,
			},
			ThenExpected: thenExpected{
				shouldError:   true,
				errorContains: "context canceled",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.Scenario, func(t *testing.T) {
			// When.
			ctx := context.Background()
			if tt.WhenDetail.cancelAfter > 0 {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)

				go func() {
					time.Sleep(tt.WhenDetail.cancelAfter)
					cancel()
				}()
			}
			err := Backoff(ctx, tt.WhenDetail.operation, tt.WhenDetail.opts)

			// Then.
			if tt.ThenExpected.shouldError {
				Expect(err).To(HaveOccurred())
				if tt.ThenExpected.errorContains != "" {
					Expect(err.Error()).To(ContainSubstring(tt.ThenExpected.errorContains))
				}
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		})
	}
}

// createFailingThenSucceedingOperation returns an operation that fails for the specified
// number of attempts and then succeeds.
func createFailingThenSucceedingOperation(failCount int) func() error {
	attempts := 0
	return func() error {
		if attempts < failCount {
			attempts++
			return errors.New("temporary error")
		}
		return nil
	}
}
