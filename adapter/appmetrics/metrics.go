package appmetrics

import (
	"context"
	"fmt"

	"github.com/abolfazlnorzad/graph/service"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

var _ service.Metrics = (*AppMetrics)(nil)

type AppMetrics struct {
	taskCreatedCounter   metric.Int64Counter
	tasksCount           metric.Int64UpDownCounter
	taskCreatedHistogram metric.Float64Histogram
}

func NewAppMetrics(meter metric.Meter) (*AppMetrics, error) {
	taskCreatedCounter, err := meter.Int64Counter(
		"requests_total",
		metric.WithDescription("Total number of task creation requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create requests_total metric: %w", err)
	}

	tasksCount, err := meter.Int64UpDownCounter(
		"tasks_count",
		metric.WithDescription("Current total number of tasks in the system"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create tasks_count metric: %w", err)
	}

	taskCreatedHistogram, err := meter.Float64Histogram(
		"request_latency_histogram",
		metric.WithDescription("Task creation latency histogram in seconds"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create request_latency_histogram metric: %w", err)
	}

	return &AppMetrics{
		taskCreatedCounter:   taskCreatedCounter,
		tasksCount:           tasksCount,
		taskCreatedHistogram: taskCreatedHistogram,
	}, nil
}

func (m *AppMetrics) IncTaskCreated(ctx context.Context, status string, reason string) {
	m.taskCreatedCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("status", status),
		attribute.String("reason", reason),
	))
}

func (m *AppMetrics) DecTasksCount(ctx context.Context, status string, reason string) {
	m.tasksCount.Add(ctx, -1, metric.WithAttributes(
		attribute.String("status", status),
		attribute.String("reason", reason),
	))
}

func (m *AppMetrics) RecordTaskCreatedDuration(ctx context.Context, duration float64) {
	m.taskCreatedHistogram.Record(ctx, duration)
}

func (m *AppMetrics) IncTasksCount(ctx context.Context, status string, reason string) {
	m.tasksCount.Add(ctx, 1, metric.WithAttributes(
		attribute.String("status", status),
		attribute.String("reason", reason),
	))
}
