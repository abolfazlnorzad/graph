package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/abolfazlnorzad/graph/entity"
	"github.com/abolfazlnorzad/graph/param"
	"github.com/abolfazlnorzad/graph/pkg/msg"
	"github.com/abolfazlnorzad/graph/pkg/richerror"
	"github.com/abolfazlnorzad/graph/pkg/trace"
	"github.com/abolfazlnorzad/graph/validation"
	"go.opentelemetry.io/otel/attribute"
	"golang.org/x/sync/errgroup"
	"golang.org/x/sync/singleflight"
)

type ListTaskCriteria struct {
	PageSize   int
	PageNumber int
	Status     *entity.TaskStatus
	Assignee   *string
}

//go:generate mockery --name=Repository --output=./mocks --outpkg=mocks
type Repository interface {
	DeleteTask(ctx context.Context, id entity.ID) error
	GetTask(ctx context.Context, id entity.ID) (entity.Task, error)
	ListTask(ctx context.Context, criteria ListTaskCriteria) ([]entity.Task, int64, error)
	GetAuditLogsByTaskID(ctx context.Context, taskID entity.ID, page, size int) ([]entity.TaskAuditLog, int64, error)
	CreateTaskWithAuditLog(ctx context.Context, t entity.Task, auditLog entity.TaskAuditLog) (entity.Task, error)
	UpdateTaskWithAuditLog(ctx context.Context, t entity.Task, auditLog entity.TaskAuditLog) error
}

//go:generate mockery --name=CacheStore --output=./mocks --outpkg=mocks
type CacheStore interface {
	Set(ctx context.Context, key string, value any, expiration time.Duration) error
	Get(ctx context.Context, key string, dest any) error
	Delete(ctx context.Context, keys ...string) error
	GetTTL(ctx context.Context, key string) (time.Duration, bool, error)
	DeleteByPrefix(ctx context.Context, prefix string) error
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
	IncTaskListed(ctx context.Context, status string)
	RecordTaskListedDuration(ctx context.Context, duration float64)
}

type Service struct {
	repo   Repository
	cache  CacheStore
	logger *slog.Logger
	mtr    Metrics
	vld    validation.Validator
	sf     singleflight.Group
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

	// Default status to TODO if empty, normalize to uppercase
	if req.Status == "" {
		req.Status = entity.StatusTodo
	} else {
		req.Status = entity.TaskStatus(strings.ToUpper(string(req.Status)))
	}

	if err := s.vld.ValidateCreateTask(req); err != nil {
		s.mtr.IncTaskCreated(ctx, "fail", "validation_error")
		trace.RecordError(span, err)
		reqLogger.ErrorContext(ctx, "validation failed", slog.Any("error", err))
		return param.CreateTaskResponse{}, err
	}

	t, err := s.repo.CreateTaskWithAuditLog(ctx, entity.Task{
		Title:       req.Title,
		Description: req.Description,
		Status:      req.Status,
		Assignee:    req.Assignee,
		Version:     1,
	}, entity.TaskAuditLog{
		Action: entity.ActionCreate,
		NewState: map[string]any{
			"title":       req.Title,
			"description": req.Description,
			"status":      req.Status,
			"assignee":    req.Assignee,
			"version":     1,
		},
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
	if cErr := s.cache.DeleteByPrefix(ctx, "tasks:list:"); cErr != nil {
		trace.RecordError(span, cErr)
		reqLogger.WarnContext(ctx, "failed to invalidate tasks list cache", slog.Any("error", cErr))
	}
	if cErr := s.cache.Set(ctx, fmt.Sprintf("task:%d", t.ID), t, time.Minute*5); cErr != nil {
		trace.RecordError(span, cErr)
		reqLogger.ErrorContext(ctx, "failed to cache task", slog.Any("error", cErr))
	}

	s.mtr.IncTaskCreated(ctx, "success", "none")
	s.mtr.IncTasksCount(ctx, string(t.Status), "none")
	reqLogger.InfoContext(ctx, "task created", slog.Int64("task_id", int64(t.ID)))

	taskResp := mapTaskEntityToTaskResponse(t)

	auditLogs, _, auditErr := s.repo.GetAuditLogsByTaskID(ctx, t.ID, 1, 10)
	if auditErr == nil {
		auditResponses := make([]param.AuditLogResponse, 0, len(auditLogs))
		for _, l := range auditLogs {
			auditResponses = append(auditResponses, param.AuditLogResponse{
				ID:            l.ID,
				TaskID:        l.TaskID,
				Action:        l.Action,
				PreviousState: l.PreviousState,
				NewState:      l.NewState,
				CreatedAt:     l.CreatedAt,
			})
		}
		taskResp.AuditLogs = auditResponses
	}

	return param.CreateTaskResponse{
		Task: taskResp,
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

	// Normalize status to uppercase if provided
	if req.Status != nil {
		normalized := entity.TaskStatus(strings.ToUpper(string(*req.Status)))
		req.Status = &normalized
	}

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

	oldState := existing

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

	auditLog := entity.TaskAuditLog{
		Action: entity.ActionUpdate,
		PreviousState: map[string]any{
			"title":       oldState.Title,
			"description": oldState.Description,
			"status":      oldState.Status,
			"assignee":    oldState.Assignee,
			"version":     oldState.Version,
		},
		NewState: map[string]any{
			"title":       existing.Title,
			"description": existing.Description,
			"status":      existing.Status,
			"assignee":    existing.Assignee,
			"version":     existing.Version,
		},
	}

	if err := s.repo.UpdateTaskWithAuditLog(ctx, existing, auditLog); err != nil {
		s.mtr.IncTaskUpdated(ctx, "fail", "db_error")
		trace.RecordError(span, err)
		reqLogger.ErrorContext(ctx, "failed to update task and audit log", slog.Any("error", err))
		return param.UpdateTaskResponse{}, richerror.New(op).WithErr(err)
	}

	if cErr := s.cache.DeleteByPrefix(ctx, "tasks:list:"); cErr != nil {
		trace.RecordError(span, cErr)
		reqLogger.WarnContext(ctx, "failed to invalidate tasks list cache", slog.Any("error", cErr))
	}
	if cErr := s.cache.Delete(ctx, fmt.Sprintf("task:%d", existing.ID)); cErr != nil {
		trace.RecordError(span, cErr)
		reqLogger.ErrorContext(ctx, "failed to invalidate task cache", slog.Any("error", cErr))
	}

	s.mtr.IncTaskUpdated(ctx, "success", "none")
	reqLogger.InfoContext(ctx, "task updated with audit log", slog.Int64("task_id", int64(existing.ID)))

	taskResp := mapTaskEntityToTaskResponse(existing)

	auditLogs, _, auditErr := s.repo.GetAuditLogsByTaskID(ctx, existing.ID, 1, 10)
	if auditErr == nil {
		auditResponses := make([]param.AuditLogResponse, 0, len(auditLogs))
		for _, l := range auditLogs {
			auditResponses = append(auditResponses, param.AuditLogResponse{
				ID:            l.ID,
				TaskID:        l.TaskID,
				Action:        l.Action,
				PreviousState: l.PreviousState,
				NewState:      l.NewState,
				CreatedAt:     l.CreatedAt,
			})
		}
		taskResp.AuditLogs = auditResponses
	}

	return param.UpdateTaskResponse{
		Task: taskResp,
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

	auditPage := req.AuditPage
	if auditPage <= 0 {
		auditPage = 1
	}
	auditPageSize := req.AuditPageSize
	if auditPageSize <= 0 {
		auditPageSize = 10
	}

	var (
		task       entity.Task
		auditLogs  []entity.TaskAuditLog
		auditTotal int64
		auditErr   error
	)

	eg, egCtx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		cacheKey := fmt.Sprintf("task:%d", req.ID)

		if err := s.cache.Get(egCtx, cacheKey, &task); err == nil {
			if task.ID == 0 {
				return richerror.New(op).WithKind(richerror.KindNotFound).WithMessage("task not found")
			}
			s.mtr.IncTaskFetched(egCtx, "success", "cache")
			span.SetAttributes(attribute.Bool("cache.hit", true))
			reqLogger.DebugContext(egCtx, "task fetched from cache")
			return nil
		}

		span.SetAttributes(attribute.Bool("cache.hit", false))

		result, sfErr, _ := s.sf.Do(cacheKey, func() (any, error) {
			var cachedTask entity.Task
			if cacheErr := s.cache.Get(egCtx, cacheKey, &cachedTask); cacheErr == nil {
				if cachedTask.ID == 0 {
					return nil, richerror.New(op).WithKind(richerror.KindNotFound).WithMessage("task not found")
				}
				return cachedTask, nil
			}

			t, dbErr := s.repo.GetTask(egCtx, req.ID)
			if dbErr != nil {
				if richerror.IsKind(dbErr, richerror.KindNotFound) {
					_ = s.cache.Set(egCtx, cacheKey, entity.Task{ID: 0}, time.Minute*1)
				}
				return nil, dbErr
			}

			if cacheErr := s.cache.Set(egCtx, cacheKey, t, time.Minute*5); cacheErr != nil {
				trace.RecordError(span, cacheErr)
				reqLogger.WarnContext(egCtx, "failed to populate cache", slog.Any("error", cacheErr))
			}
			return t, nil
		})

		if sfErr != nil {
			return sfErr
		}

		task = result.(entity.Task)
		s.mtr.IncTaskFetched(egCtx, "success", "db")
		reqLogger.InfoContext(egCtx, "task fetched from db")
		return nil
	})

	eg.Go(func() error {
		var logsErr error
		auditLogs, auditTotal, logsErr = s.repo.GetAuditLogsByTaskID(egCtx, req.ID, auditPage, auditPageSize)
		if logsErr != nil {
			auditErr = logsErr
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		trace.RecordError(span, err)
		reqLogger.ErrorContext(ctx, "failed to get task", slog.Any("error", err))
		return param.GetTaskByIDResponse{}, err
	}

	taskResp := mapTaskEntityToTaskResponse(task)

	if auditErr != nil {
		trace.RecordError(span, auditErr)
		reqLogger.WarnContext(ctx, "failed to fetch audit logs", slog.Any("error", auditErr))
	} else {
		auditResponses := make([]param.AuditLogResponse, 0, len(auditLogs))
		for _, l := range auditLogs {
			auditResponses = append(auditResponses, param.AuditLogResponse{
				ID:            l.ID,
				TaskID:        l.TaskID,
				Action:        l.Action,
				PreviousState: l.PreviousState,
				NewState:      l.NewState,
				CreatedAt:     l.CreatedAt,
			})
		}
		taskResp.AuditLogs = auditResponses
	}

	return param.GetTaskByIDResponse{
		Task: taskResp,
		AuditPagination: &param.PaginationResponse{
			PageSize:   auditPageSize,
			PageNumber: auditPage,
			Total:      auditTotal,
		},
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

	if cErr := s.cache.DeleteByPrefix(ctx, "tasks:list:"); cErr != nil {
		trace.RecordError(span, cErr)
		reqLogger.WarnContext(ctx, "failed to invalidate tasks list cache", slog.Any("error", cErr))
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

func (s Service) ListTask(ctx context.Context, req param.ListTasksRequest) (param.ListTasksResponse, error) {
	const op = "service.ListTask"
	startTime := time.Now()

	ctx, span := trace.Tracer().Start(ctx, op)
	defer func() {
		s.mtr.RecordTaskListedDuration(ctx, time.Since(startTime).Seconds())
		span.End()
	}()

	reqLogger := s.logger.With(
		slog.String("op", op),
		slog.Int("page_number", req.Pagination.PageNumber),
		slog.Int("page_size", req.Pagination.PageSize),
	)

	var statusStr, assigneeStr string
	if req.Filter.Status != nil {
		statusStr = string(*req.Filter.Status)
	}
	if req.Filter.Assignee != nil {
		assigneeStr = *req.Filter.Assignee
	}

	cacheKey := fmt.Sprintf("tasks:list:page:%d:size:%d:status:%s:assignee:%s",
		req.Pagination.PageNumber, req.Pagination.PageSize, statusStr, assigneeStr)

	var cachedResp param.ListTasksResponse
	if err := s.cache.Get(ctx, cacheKey, &cachedResp); err == nil {
		s.mtr.IncTaskListed(ctx, "success_cache")
		span.SetAttributes(attribute.Bool("cache.hit", true))
		reqLogger.DebugContext(ctx, "tasks list fetched from cache")
		return cachedResp, nil
	}

	span.SetAttributes(attribute.Bool("cache.hit", false))

	// Singleflight: only one goroutine hits DB for the same cache key
	result, err, _ := s.sf.Do(cacheKey, func() (any, error) {
		// Re-check cache inside singleflight
		var cached param.ListTasksResponse
		if cacheErr := s.cache.Get(ctx, cacheKey, &cached); cacheErr == nil {
			return cached, nil
		}

		criteria := ListTaskCriteria{
			PageNumber: req.Pagination.PageNumber,
			PageSize:   req.Pagination.PageSize,
			Status:     req.Filter.Status,
			Assignee:   req.Filter.Assignee,
		}

		tasks, total, dbErr := s.repo.ListTask(ctx, criteria)
		if dbErr != nil {
			return nil, dbErr
		}

		var respTasks []param.TaskResponse
		for _, t := range tasks {
			respTasks = append(respTasks, mapTaskEntityToTaskResponse(t))
		}

		resp := param.ListTasksResponse{
			Tasks: respTasks,
			Pagination: param.PaginationResponse{
				PageSize:   req.Pagination.PageSize,
				PageNumber: req.Pagination.PageNumber,
				Total:      total,
			},
		}

		if cacheErr := s.cache.Set(ctx, cacheKey, resp, time.Minute*2); cacheErr != nil {
			trace.RecordError(span, cacheErr)
			reqLogger.WarnContext(ctx, "failed to cache task list", slog.Any("error", cacheErr))
		}

		return resp, nil
	})

	if err != nil {
		s.mtr.IncTaskListed(ctx, "fail_db")
		trace.RecordError(span, err)
		reqLogger.ErrorContext(ctx, "failed to list tasks", slog.Any("error", err))
		return param.ListTasksResponse{}, richerror.New(op).WithErr(err)
	}

	s.mtr.IncTaskListed(ctx, "success_db")
	return result.(param.ListTasksResponse), nil
}
