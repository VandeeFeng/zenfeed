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
	"time"

	"github.com/pkg/errors"
	"k8s.io/utils/ptr"

	"github.com/vandeefeng/zenfeed/pkg/telemetry/log"
)

type Options struct {
	MinInterval time.Duration
	MaxInterval time.Duration
	MaxAttempts *int
}

func (opts *Options) adjust() {
	if opts.MinInterval == 0 {
		opts.MinInterval = 100 * time.Millisecond
	}
	if opts.MaxInterval == 0 {
		opts.MaxInterval = 10 * time.Second
	}
	if opts.MaxInterval < opts.MinInterval {
		opts.MaxInterval = opts.MinInterval
	}
	if opts.MaxAttempts == nil {
		opts.MaxAttempts = ptr.To(3)
	}
}

var InfAttempts = ptr.To(-1)

func Backoff(ctx context.Context, operation func() error, opts *Options) error {
	switch err := operation(); err {
	case nil:
		return nil // One time success.

	default:
		log.Error(ctx, err, "attempt", 1)
	}

	if opts == nil {
		opts = &Options{}
	}
	opts.adjust()

	interval := opts.MinInterval
	attempts := 2 // Start from 1.

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case <-time.After(interval):
			if err := operation(); err != nil {
				if reachedMaxAttempts(attempts, *opts.MaxAttempts) {
					return errors.Wrap(err, "max attempts reached")
				}
				log.Error(ctx, err, "attempt", attempts)

				interval = nextInterval(interval, opts.MaxInterval)
				attempts++

				continue
			}

			return nil
		}
	}
}

func nextInterval(cur, max time.Duration) (next time.Duration) {
	return min(2*cur, max)
}

func reachedMaxAttempts(cur, max int) bool {
	if max == *InfAttempts {
		return false
	}

	return cur >= max
}
