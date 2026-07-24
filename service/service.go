package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/abolfazlnorzad/graph/entity"
	"github.com/abolfazlnorzad/graph/param"
	"github.com/abolfazlnorzad/graph/pkg/richerror"
	"github.com/abolfazlnorzad/graph/pkg/trace"
	"github.com/abolfazlnorzad/graph/validation"
	"go.opentelemetry.io/otel/attribute"
)

type ListTaskCriteria struct {
	PageSize   int
	PageNumber int
	Status     *entity.TaskStatus
	Assignee   *string
}

//go:generate mockery --name=Repository --output=./mocks --outpkg=mocks
type Repository interface {
	CreateTask(ctx context.Context, t entity.Task) (entity.Task, error)
	UpdateTask(ctx context.Context, t entity.Task) error
	DeleteTask(ctx context.Context, id entity.ID) error
	GetTask(ctx context.Context, id entity.ID) (entity.Task, error)
	ListTask(ctx context.Context, criteria ListTaskCriteria) ([]entity.Task, error)
}

//go:generate mockery --name=CacheStore --output=./mocks --outpkg=mocks
type CacheStore interface {
	Set(ctx context.Context, key string, value any, expiration time.Duration) error
	Get(ctx context.Context, key string) (string, error)
	Delete(ctx context.Context, keys ...string) error
	GetTTL(ctx context.Context, key string) (time.Duration, bool, error)
}

//go:generate mockery --name=Metrics --output=./mocks --outpkg=mocks
type Metrics interface {
	IncTaskCreated(ctx context.Context, status string, reason string)
	IncTasksCount(ctx context.Context, status string, reason string)
	DecTasksCount(ctx context.Context, status string, reason string)
	RecordTaskCreatedDuration(ctx context.Context, duration float64)
}

type Service struct {
	repo   Repository
	cache  CacheStore
	logger *slog.Logger
	mtr    Metrics
	vld    validation.Validator
}

func NewService(repo Repository, cache CacheStore, logger *slog.Logger, mtr Metrics, vld validation.Validator) Service {
	return Service{repo: repo, cache: cache, logger: logger, mtr: mtr, vld: vld}
}

func mapTaskEntityToTaskResponse(t entity.Task) param.TaskResponse {
	return param.TaskResponse{
		ID:          t.ID,
		Title:       t.Title,
		Description: t.Description,
		Status:      t.Status,
		Assignee:    t.Assignee,
		Version:     t.Version,
		CreatedAt:   t.CreatedAt,
		UpdatedAt:   t.UpdatedAt,
	}
}

func (s Service) CreateTask(ctx context.Context, req param.CreateTaskRequest) (param.CreateTaskResponse, error) {
	const op = "service.CreateTask"

	startTime := time.Now()

	ctx, span := trace.Tracer().Start(ctx, op)
	defer func() {
		s.mtr.RecordTaskCreatedDuration(ctx, time.Since(startTime).Seconds())
		span.End()
	}()

	span.SetAttributes(
		attribute.String("task.title", req.Title),
		attribute.String("task.status", string(req.Status)),
	)
	reqLogger := s.logger.With(
		slog.String("op", op),
		slog.String("title", req.Title),
		slog.String("status", string(req.Status)),
	)

	if err := s.vld.ValidateCreateTask(req); err != nil {
		s.mtr.IncTaskCreated(ctx, "fail", "validation_error")
		trace.RecordError(span, err)
		reqLogger.ErrorContext(ctx, "validation failed", slog.Any("error", err))
		return param.CreateTaskResponse{}, err
	}

	t, err := s.repo.CreateTask(ctx, entity.Task{
		Title:       req.Title,
		Description: req.Description,
		Status:      req.Status,
		Assignee:    req.Assignee,
		Version:     1,
	})

	if err != nil {
		s.mtr.IncTaskCreated(ctx, "fail", "db_error")
		trace.RecordError(span, err)
		reqLogger.ErrorContext(ctx, "failed to create task", slog.Any("error", err))
		return param.CreateTaskResponse{}, richerror.New(op).WithErr(err)
	}

	span.SetAttributes(attribute.Int64("task.id", int64(t.ID)))
	if req.Assignee != nil {
		span.SetAttributes(attribute.String("task.assignee", *req.Assignee))
	}

	if cErr := s.cache.Set(ctx, fmt.Sprintf("task:%d", t.ID), t, time.Minute*5); cErr != nil {
		trace.RecordError(span, cErr)
		reqLogger.ErrorContext(ctx, "failed to cache task", slog.Any("error", cErr))
	}

	s.mtr.IncTaskCreated(ctx, "success", "none")
	s.mtr.IncTasksCount(ctx, string(t.Status), "none")
	reqLogger.InfoContext(ctx, "task created", slog.Int64("task_id", int64(t.ID)))

	return param.CreateTaskResponse{
		Task: mapTaskEntityToTaskResponse(t),
	}, nil
}
