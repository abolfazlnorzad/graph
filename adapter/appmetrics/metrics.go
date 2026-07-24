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
	taskUpdatedCounter   metric.Int64Counter
	tasksCount           metric.Int64UpDownCounter
	taskCreatedHistogram metric.Float64Histogram
	taskUpdatedHistogram metric.Float64Histogram
	taskFetchedCounter   metric.Int64Counter
	taskFetchedHistogram metric.Float64Histogram
	taskDeletedCounter   metric.Int64Counter
	taskDeletedHistogram metric.Float64Histogram
	taskListedCounter    metric.Int64Counter
	taskListedHistogram  metric.Float64Histogram
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

	taskFetchedCounter, err := meter.Int64Counter("task_fetched_total", metric.WithDescription("Total task fetch requests"))
	if err != nil {
		return nil, err
	}

	taskFetchedHistogram, err := meter.Float64Histogram("task_fetched_latency", metric.WithDescription("Task fetch latency"))
	if err != nil {
		return nil, err
	}

	taskDeletedCounter, err := meter.Int64Counter("task_deleted_total", metric.WithDescription("Total task delete requests"))
	if err != nil {
		return nil, err
	}

	taskDeletedHistogram, err := meter.Float64Histogram("task_deleted_latency", metric.WithDescription("Task delete latency"))
	if err != nil {
		return nil, err
	}

	taskListedCounter, err := meter.Int64Counter("task_listed_total", metric.WithDescription("Total task list requests"))
	if err != nil {
		return nil, err
	}

	taskListedHistogram, err := meter.Float64Histogram("task_listed_latency", metric.WithDescription("Task list latency"))
	if err != nil {
		return nil, err
	}

	return &AppMetrics{
		taskCreatedCounter:   taskCreatedCounter,
		taskUpdatedCounter:   taskUpdatedCounter,
		tasksCount:           tasksCount,
		taskCreatedHistogram: taskCreatedHistogram,
		taskUpdatedHistogram: taskUpdatedHistogram,
		taskFetchedCounter:   taskFetchedCounter,
		taskFetchedHistogram: taskFetchedHistogram,
		taskDeletedCounter:   taskDeletedCounter,
		taskDeletedHistogram: taskDeletedHistogram,
		taskListedCounter:    taskListedCounter,
		taskListedHistogram:  taskListedHistogram,
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

func (m *AppMetrics) IncTaskFetched(ctx context.Context, status string, source string) {
	m.taskFetchedCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("status", status),
		attribute.String("source", source),
	))
}

func (m *AppMetrics) RecordTaskFetchedDuration(ctx context.Context, duration float64) {
	m.taskFetchedHistogram.Record(ctx, duration)
}

func (m *AppMetrics) IncTaskDeleted(ctx context.Context, status string, reason string) {
	m.taskDeletedCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("status", status),
		attribute.String("reason", reason),
	))
}

func (m *AppMetrics) RecordTaskDeletedDuration(ctx context.Context, duration float64) {
	m.taskDeletedHistogram.Record(ctx, duration)
}

func (m *AppMetrics) IncTaskListed(ctx context.Context, status string) {
	m.taskListedCounter.Add(ctx, 1, metric.WithAttributes(
		attribute.String("status", status),
	))
}

func (m *AppMetrics) RecordTaskListedDuration(ctx context.Context, duration float64) {
	m.taskListedHistogram.Record(ctx, duration)
}
