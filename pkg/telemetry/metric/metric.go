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

package metric

import (
	"context"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/vandeefeng/zenfeed/pkg/model"
	telemetrymodel "github.com/vandeefeng/zenfeed/pkg/telemetry/model"
)

func Handler() http.Handler {
	return promhttp.Handler()
}

var (
	operationInFlight = promauto.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: model.AppName,
			Name:      "operation_in_flight",
			Help:      "Number of operations in flight.",
		},
		[]string{
			telemetrymodel.KeyComponent,
			telemetrymodel.KeyComponentInstance,
			telemetrymodel.KeyOperation,
		},
	)

	operationTotal = promauto.NewCounterVec(
		prometheus.CounterOpts{
			Namespace: model.AppName,
			Name:      "operation_total",
			Help:      "Total number of operations.",
		},
		[]string{
			telemetrymodel.KeyComponent,
			telemetrymodel.KeyComponentInstance,
			telemetrymodel.KeyOperation,
			telemetrymodel.KeyResult,
		},
	)

	operationDuration = promauto.NewHistogramVec(
		prometheus.HistogramOpts{
			Namespace: model.AppName,
			Name:      "operation_duration_seconds",
			Help:      "Histogram of operation latencies in seconds.",
			Buckets:   []float64{.001, .005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5, 10, 20},
		},
		[]string{
			telemetrymodel.KeyComponent,
			telemetrymodel.KeyComponentInstance,
			telemetrymodel.KeyOperation,
			telemetrymodel.KeyResult,
		},
	)
)

type ctxKey uint8

const (
	ctxKeyComponent ctxKey = iota
	ctxKeyInstance
	ctxKeyOperation
	ctxKeyStartTime
)

func StartWith(ctx context.Context, keyvals ...any) context.Context {
	// Extend from parent context.
	component, instance, operation, _ := parseFrom(ctx)

	// Parse component and operation... from keyvals.
	for i := 0; i < len(keyvals); i += 2 {
		if i+1 < len(keyvals) {
			switch keyvals[i] {
			case telemetrymodel.KeyComponent:
				component = keyvals[i+1].(string)
			case telemetrymodel.KeyComponentInstance:
				instance = keyvals[i+1].(string)
			case telemetrymodel.KeyOperation:
				operation = keyvals[i+1].(string)
			}
		}
	}
	if component == "" || operation == "" {
		panic("missing required keyvals")
	}

	// Record operation in flight.
	operationInFlight.WithLabelValues(component, instance, operation).Inc()

	// Add to context.
	ctx = context.WithValue(ctx, ctxKeyComponent, component)
	ctx = context.WithValue(ctx, ctxKeyInstance, instance)
	ctx = context.WithValue(ctx, ctxKeyOperation, operation)
	ctx = context.WithValue(ctx, ctxKeyStartTime, time.Now())

	return ctx
}

func RecordRED(ctx context.Context, err error) {
	// Parse component, instance, operation, and start time from context.
	component, instance, operation, startTime := parseFrom(ctx)
	duration := time.Since(startTime)

	// Determine result.
	result := telemetrymodel.ValResultSuccess
	if err != nil {
		result = telemetrymodel.ValResultError
	}

	// Record metrics.
	operationTotal.WithLabelValues(component, instance, operation, result).Inc()
	operationDuration.WithLabelValues(component, instance, operation, result).Observe(duration.Seconds())
	operationInFlight.WithLabelValues(component, instance, operation).Dec()
}

func Close(id prometheus.Labels) {
	operationInFlight.DeletePartialMatch(id)
	operationTotal.DeletePartialMatch(id)
	operationDuration.DeletePartialMatch(id)
}

func parseFrom(ctx context.Context) (component, instance, operation string, startTime time.Time) {
	if v := ctx.Value(ctxKeyComponent); v != nil {
		component = v.(string)
	}
	if v := ctx.Value(ctxKeyInstance); v != nil {
		instance = v.(string)
	}
	if v := ctx.Value(ctxKeyOperation); v != nil {
		operation = v.(string)
	}
	if v := ctx.Value(ctxKeyStartTime); v != nil {
		startTime = v.(time.Time)
	}

	return
}
