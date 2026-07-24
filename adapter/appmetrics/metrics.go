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
	taskCreatedCounter      metric.Int64Counter
	taskUpdatedCounter      metric.Int64Counter
	tasksCount              metric.Int64UpDownCounter
	taskCreatedHistogram    metric.Float64Histogram
	taskUpdatedHistogram    metric.Float64Histogram
}

func NewAppMetrics(meter metric.Meter) (*AppMetrics, error) {
	taskCreatedCounter, err := meter.Int64Counter(
		"task_created_total",
		metric.WithDescription("Total number of task creation requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create task_created_total metric: %w", err)
	}

	taskUpdatedCounter, err := meter.Int64Counter(
		"task_updated_total",
		metric.WithDescription("Total number of task update requests"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create task_updated_total metric: %w", err)
	}

	tasksCount, err := meter.Int64UpDownCounter(
		"tasks_count",
		metric.WithDescription("Current total number of tasks in the system"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create tasks_count metric: %w", err)
	}

	taskCreatedHistogram, err := meter.Float64Histogram(
		"task_created_duration_seconds",
		metric.WithDescription("Task creation latency in seconds"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create task_created_duration_seconds metric: %w", err)
	}

	taskUpdatedHistogram, err := meter.Float64Histogram(
		"task_updated_duration_seconds",
		metric.WithDescription("Task update latency in seconds"),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create task_updated_duration_seconds metric: %w", err)
	}

	return &AppMetrics{
		taskCreatedCounter:   taskCreatedCounter,
		taskUpdatedCounter:   taskUpdatedCounter,
		tasksCount:           tasksCount,
		taskCreatedHistogram: taskCreatedHistogram,
		taskUpdatedHistogram: taskUpdatedHistogram,
	}, nil
}

func (m *AppMetrics) IncTaskCreated(ctx context.Context, status string, reason string) {
	m.taskCreatedCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("status", status),
		attribute.String("reason", reason),
	))
}

func (m *AppMetrics) IncTaskUpdated(ctx context.Context, status string, reason string) {
	m.taskUpdatedCounter.Add(ctx, 1, metric.WithAttributes(
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

func (m *AppMetrics) RecordTaskUpdatedDuration(ctx context.Context, duration float64) {
	m.taskUpdatedHistogram.Record(ctx, duration)
}

func (m *AppMetrics) IncTasksCount(ctx context.Context, status string, reason string) {
	m.tasksCount.Add(ctx, 1, metric.WithAttributes(
		attribute.String("status", status),
		attribute.String("reason", reason),
	))
}
