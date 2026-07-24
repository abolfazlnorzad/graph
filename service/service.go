package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/abolfazlnorzad/graph/entity"
	"github.com/abolfazlnorzad/graph/param"
	"github.com/abolfazlnorzad/graph/pkg/msg"
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
	Get(ctx context.Context, key string, dest any) error
	Delete(ctx context.Context, keys ...string) error
	GetTTL(ctx context.Context, key string) (time.Duration, bool, error)
}

//go:generate mockery --name=Metrics --output=./mocks --outpkg=mocks
type Metrics interface {
	IncTaskCreated(ctx context.Context, status string, reason string)
	IncTaskUpdated(ctx context.Context, status string, reason string)
	IncTasksCount(ctx context.Context, status string, reason string)
	DecTasksCount(ctx context.Context, status string, reason string)
	RecordTaskCreatedDuration(ctx context.Context, duration float64)
	RecordTaskUpdatedDuration(ctx context.Context, duration float64)
	IncTaskFetched(ctx context.Context, status string, source string) // source = "cache" or "db"
	RecordTaskFetchedDuration(ctx context.Context, duration float64)
	IncTaskDeleted(ctx context.Context, status string, reason string)
	RecordTaskDeletedDuration(ctx context.Context, duration float64)
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

func (s Service) UpdateTask(ctx context.Context, req param.UpdateTaskRequest) (param.UpdateTaskResponse, error) {
	const op = "service.UpdateTask"

	startTime := time.Now()

	ctx, span := trace.Tracer().Start(ctx, op)
	defer func() {
		s.mtr.RecordTaskUpdatedDuration(ctx, time.Since(startTime).Seconds())
		span.End()
	}()

	span.SetAttributes(attribute.Int64("task.id", int64(req.ID)))
	reqLogger := s.logger.With(
		slog.String("op", op),
		slog.Int64("task_id", int64(req.ID)),
	)

	if err := s.vld.ValidateUpdateTask(req); err != nil {
		s.mtr.IncTaskUpdated(ctx, "fail", "validation_error")
		trace.RecordError(span, err)
		reqLogger.ErrorContext(ctx, "validation failed", slog.Any("error", err))
		return param.UpdateTaskResponse{}, err
	}

	existing, err := s.repo.GetTask(ctx, req.ID)
	if err != nil {
		trace.RecordError(span, err)
		if richerror.IsKind(err, richerror.KindNotFound) {
			s.mtr.IncTaskUpdated(ctx, "fail", "not_found")
			reqLogger.ErrorContext(ctx, "task not found", slog.Int64("task_id", int64(req.ID)))
			return param.UpdateTaskResponse{}, richerror.New(op).
				WithErr(err).
				WithKind(richerror.KindNotFound).
				WithUserMsgKey(msg.ErrNotFound)
		}
		s.mtr.IncTaskUpdated(ctx, "fail", "db_error")
		reqLogger.ErrorContext(ctx, "failed to get task", slog.Any("error", err))
		return param.UpdateTaskResponse{}, richerror.New(op).WithErr(err)
	}

	if existing.Version != req.Version {
		err := richerror.New(op).
			WithKind(richerror.KindConflict).
			WithMessage("version conflict")
		s.mtr.IncTaskUpdated(ctx, "fail", "version_conflict")
		trace.RecordError(span, err)
		reqLogger.ErrorContext(ctx, "version conflict",
			slog.Int("expected", int(existing.Version)),
			slog.Int("got", int(req.Version)),
		)
		return param.UpdateTaskResponse{}, err
	}

	if req.Title != nil {
		existing.Title = *req.Title
	}
	if req.Description != nil {
		existing.Description = req.Description
	}
	if req.Status != nil {
		existing.Status = *req.Status
	}
	if req.Assignee != nil {
		existing.Assignee = req.Assignee
	}
	existing.Version++

	if err := s.repo.UpdateTask(ctx, existing); err != nil {
		s.mtr.IncTaskUpdated(ctx, "fail", "db_error")
		trace.RecordError(span, err)
		reqLogger.ErrorContext(ctx, "failed to update task", slog.Any("error", err))
		return param.UpdateTaskResponse{}, richerror.New(op).WithErr(err)
	}

	if cErr := s.cache.Delete(ctx, fmt.Sprintf("task:%d", existing.ID)); cErr != nil {
		trace.RecordError(span, cErr)
		reqLogger.ErrorContext(ctx, "failed to invalidate task cache", slog.Any("error", cErr))
	}

	s.mtr.IncTaskUpdated(ctx, "success", "none")
	reqLogger.InfoContext(ctx, "task updated", slog.Int64("task_id", int64(existing.ID)))

	return param.UpdateTaskResponse{
		Task: mapTaskEntityToTaskResponse(existing),
	}, nil
}

func (s Service) GetTask(ctx context.Context, req param.GetTaskByIDRequest) (param.GetTaskByIDResponse, error) {
	const op = "service.GetTask"
	startTime := time.Now()

	ctx, span := trace.Tracer().Start(ctx, op)
	defer func() {
		s.mtr.RecordTaskFetchedDuration(ctx, time.Since(startTime).Seconds())
		span.End()
	}()

	span.SetAttributes(attribute.Int64("task.id", int64(req.ID)))
	reqLogger := s.logger.With(
		slog.String("op", op),
		slog.Int64("task_id", int64(req.ID)),
	)

	cacheKey := fmt.Sprintf("task:%d", req.ID)
	var t entity.Task

	err := s.cache.Get(ctx, cacheKey, &t)

	if err == nil {
		//  (Cache Penetration)
		if t.ID == 0 {
			s.mtr.IncTaskFetched(ctx, "fail", "cache_hit_not_found")
			span.SetAttributes(attribute.Bool("cache.hit", true))
			reqLogger.DebugContext(ctx, "task marked as not found in cache")

			return param.GetTaskByIDResponse{}, richerror.New(op).
				WithKind(richerror.KindNotFound).
				WithMessage("task not found")
		}

		s.mtr.IncTaskFetched(ctx, "success", "cache")
		span.SetAttributes(attribute.Bool("cache.hit", true))
		reqLogger.DebugContext(ctx, "task fetched from cache")

		return param.GetTaskByIDResponse{Task: mapTaskEntityToTaskResponse(t)}, nil
	}

	span.SetAttributes(attribute.Bool("cache.hit", false))

	t, err = s.repo.GetTask(ctx, req.ID)
	if err != nil {
		s.mtr.IncTaskFetched(ctx, "fail", "db")
		trace.RecordError(span, err)

		if richerror.IsKind(err, richerror.KindNotFound) {
			_ = s.cache.Set(ctx, cacheKey, entity.Task{ID: 0}, time.Minute*1)
		}

		reqLogger.ErrorContext(ctx, "failed to get task from db", slog.Any("error", err))
		return param.GetTaskByIDResponse{}, richerror.New(op).WithErr(err)
	}

	if cErr := s.cache.Set(ctx, cacheKey, t, time.Minute*5); cErr != nil {
		trace.RecordError(span, cErr)
		reqLogger.ErrorContext(ctx, "failed to populate cache", slog.Any("error", cErr))
	}

	s.mtr.IncTaskFetched(ctx, "success", "db")
	reqLogger.InfoContext(ctx, "task fetched from db")

	return param.GetTaskByIDResponse{
		Task: mapTaskEntityToTaskResponse(t),
	}, nil
}

func (s Service) DeleteTask(ctx context.Context, req param.DeleteTaskRequest) (param.DeleteTaskResponse, error) {
	const op = "service.DeleteTask"
	startTime := time.Now()

	ctx, span := trace.Tracer().Start(ctx, op)
	defer func() {
		s.mtr.RecordTaskDeletedDuration(ctx, time.Since(startTime).Seconds())
		span.End()
	}()

	span.SetAttributes(attribute.Int64("task.id", int64(req.ID)))
	reqLogger := s.logger.With(
		slog.String("op", op),
		slog.Int64("task_id", int64(req.ID)),
	)


	err := s.repo.DeleteTask(ctx, req.ID)
	if err != nil {
		trace.RecordError(span, err)


		if richerror.IsKind(err, richerror.KindNotFound) {
			s.mtr.IncTaskDeleted(ctx, "fail", "not_found")
			reqLogger.WarnContext(ctx, "task not found for deletion", slog.Any("error", err))

			return param.DeleteTaskResponse{}, richerror.New(op).
				WithKind(richerror.KindNotFound).
				WithMessage("task not found").
				WithErr(err)
		}

		s.mtr.IncTaskDeleted(ctx, "fail", "db_error")
		reqLogger.ErrorContext(ctx, "failed to delete task from db", slog.Any("error", err))
		return param.DeleteTaskResponse{}, richerror.New(op).
			WithKind(richerror.KindUnexpected).
			WithMessage("failed to delete task").
			WithErr(err)
	}


	cacheKey := fmt.Sprintf("task:%d", req.ID)
	if cErr := s.cache.Delete(ctx, cacheKey); cErr != nil {
		trace.RecordError(span, cErr)
		reqLogger.WarnContext(ctx, "failed to invalidate cache after deletion, data might be stale", slog.Any("error", cErr))
	}


	s.mtr.IncTaskDeleted(ctx, "success", "none")
	s.mtr.DecTasksCount(ctx, "deleted", "user_action")

	reqLogger.InfoContext(ctx, "task deleted successfully")

	return param.DeleteTaskResponse{}, nil
}