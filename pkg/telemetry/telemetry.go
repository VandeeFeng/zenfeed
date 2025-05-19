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

package telemetry

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/vandeefeng/zenfeed/pkg/telemetry/log"
	"github.com/vandeefeng/zenfeed/pkg/telemetry/metric"
)

type Labels []any

func (l Labels) Get(key any) any {
	for i := 0; i < len(l); i += 2 {
		if l[i] == key {
			return l[i+1]
		}
	}

	return nil
}

// StartWith starts a new operation with the given key-value pairs.
// MUST call End() to finalize the operation.
func StartWith(ctx context.Context, keyvals ...any) context.Context {
	ctx = log.With(ctx, keyvals...)
	ctx = metric.StartWith(ctx, keyvals...)

	return ctx
}

// End records and finalizes the operation.
func End(ctx context.Context, err error) {
	metric.RecordRED(ctx, err)
}

// CloseMetrics closes the metrics for the given id.
func CloseMetrics(id prometheus.Labels) {
	metric.Close(id)
}
