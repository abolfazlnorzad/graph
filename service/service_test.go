package service_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/abolfazlnorzad/graph/entity"
	"github.com/abolfazlnorzad/graph/param"
	"github.com/abolfazlnorzad/graph/pkg/richerror"
	"github.com/abolfazlnorzad/graph/service"
	"github.com/abolfazlnorzad/graph/service/mocks"
	"github.com/abolfazlnorzad/graph/validation"
)

func TestService_CreateTask(t *testing.T) {
	nopLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	vld := validation.NewValidator()

	t.Run("Success - Task created and cached", func(t *testing.T) {
		mockRepo := new(mocks.Repository)
		mockCache := new(mocks.CacheStore)
		mockMetrics := new(mocks.Metrics)

		req := param.CreateTaskRequest{
			Title:  "Fix Login Bug",
			Status: entity.StatusTodo,
		}

		createdTask := entity.Task{
			ID:      1,
			Title:   "Fix Login Bug",
			Status:  entity.StatusTodo,
			Version: 1,
		}

		mockRepo.On("CreateTask", mock.Anything, mock.AnythingOfType("entity.Task")).
			Return(createdTask, nil).Once()

		mockCache.On("Set", mock.Anything, "task:1", createdTask, 5*time.Minute).
			Return(nil).Once()

		mockMetrics.On("RecordTaskCreatedDuration", mock.Anything, mock.AnythingOfType("float64")).
			Return().Once()
		mockMetrics.On("IncTaskCreated", mock.Anything, "success", "none").
			Return().Once()
		mockMetrics.On("IncTasksCount", mock.Anything, string(entity.StatusTodo), "none").
			Return().Once()

		svc := service.NewService(mockRepo, mockCache, nopLogger, mockMetrics, vld)

		resp, err := svc.CreateTask(context.Background(), req)

		assert.NoError(t, err)
		assert.Equal(t, entity.ID(1), resp.Task.ID)
		assert.Equal(t, "Fix Login Bug", resp.Task.Title)

		mockRepo.AssertExpectations(t)
		mockCache.AssertExpectations(t)
		mockMetrics.AssertExpectations(t)
	})

	t.Run("Success - Best effort cache (Cache fails but task is created)", func(t *testing.T) {
		mockRepo := new(mocks.Repository)
		mockCache := new(mocks.CacheStore)
		mockMetrics := new(mocks.Metrics)

		req := param.CreateTaskRequest{Title: "Design API", Status: entity.StatusTodo}
		createdTask := entity.Task{ID: 2, Title: "Design API", Status: entity.StatusTodo}

		mockRepo.On("CreateTask", mock.Anything, mock.AnythingOfType("entity.Task")).
			Return(createdTask, nil).Once()

		mockCache.On("Set", mock.Anything, "task:2", createdTask, 5*time.Minute).
			Return(errors.New("redis timeout")).Once()

		mockMetrics.On("RecordTaskCreatedDuration", mock.Anything, mock.AnythingOfType("float64")).Return().Once()
		mockMetrics.On("IncTaskCreated", mock.Anything, "success", "none").Return().Once()
		mockMetrics.On("IncTasksCount", mock.Anything, string(entity.StatusTodo), "none").Return().Once()

		svc := service.NewService(mockRepo, mockCache, nopLogger, mockMetrics, vld)

		resp, err := svc.CreateTask(context.Background(), req)

		assert.NoError(t, err)
		assert.Equal(t, entity.ID(2), resp.Task.ID)

		mockRepo.AssertExpectations(t)
		mockCache.AssertExpectations(t)
	})

	t.Run("Fail - Database error", func(t *testing.T) {
		mockRepo := new(mocks.Repository)
		mockCache := new(mocks.CacheStore)
		mockMetrics := new(mocks.Metrics)

		req := param.CreateTaskRequest{Title: "DB Test", Status: entity.StatusTodo}

		mockRepo.On("CreateTask", mock.Anything, mock.AnythingOfType("entity.Task")).
			Return(entity.Task{}, errors.New("db connection failed")).Once()

		mockMetrics.On("RecordTaskCreatedDuration", mock.Anything, mock.AnythingOfType("float64")).Return().Once()
		mockMetrics.On("IncTaskCreated", mock.Anything, "fail", "db_error").Return().Once()

		svc := service.NewService(mockRepo, mockCache, nopLogger, mockMetrics, vld)

		resp, err := svc.CreateTask(context.Background(), req)

		assert.Error(t, err)
		assert.Empty(t, resp)

		mockRepo.AssertExpectations(t)
		mockMetrics.AssertExpectations(t)
		mockCache.AssertNotCalled(t, "Set")
	})

	t.Run("Fail - Empty title validation error", func(t *testing.T) {
		mockRepo := new(mocks.Repository)
		mockCache := new(mocks.CacheStore)
		mockMetrics := new(mocks.Metrics)

		req := param.CreateTaskRequest{Title: "", Status: entity.StatusTodo}

		mockMetrics.On("RecordTaskCreatedDuration", mock.Anything, mock.AnythingOfType("float64")).Return().Once()
		mockMetrics.On("IncTaskCreated", mock.Anything, "fail", "validation_error").Return().Once()

		svc := service.NewService(mockRepo, mockCache, nopLogger, mockMetrics, vld)

		resp, err := svc.CreateTask(context.Background(), req)

		assert.Error(t, err)
		assert.Empty(t, resp)
		mockRepo.AssertNotCalled(t, "CreateTask")
		mockCache.AssertNotCalled(t, "Set")
	})

	t.Run("Fail - Invalid status validation error", func(t *testing.T) {
		mockRepo := new(mocks.Repository)
		mockCache := new(mocks.CacheStore)
		mockMetrics := new(mocks.Metrics)

		req := param.CreateTaskRequest{Title: "Valid Title", Status: "INVALID_STATUS"}

		mockMetrics.On("RecordTaskCreatedDuration", mock.Anything, mock.AnythingOfType("float64")).Return().Once()
		mockMetrics.On("IncTaskCreated", mock.Anything, "fail", "validation_error").Return().Once()

		svc := service.NewService(mockRepo, mockCache, nopLogger, mockMetrics, vld)

		resp, err := svc.CreateTask(context.Background(), req)

		assert.Error(t, err)
		assert.Empty(t, resp)
		mockRepo.AssertNotCalled(t, "CreateTask")
		mockCache.AssertNotCalled(t, "Set")
	})
}

func TestService_UpdateTask(t *testing.T) {
	nopLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	vld := validation.NewValidator()

	t.Run("Success - Update title only", func(t *testing.T) {
		mockRepo := new(mocks.Repository)
		mockCache := new(mocks.CacheStore)
		mockMetrics := new(mocks.Metrics)

		newTitle := "Updated Title"
		req := param.UpdateTaskRequest{
			ID:      1,
			Title:   &newTitle,
			Version: 1,
		}

		existingTask := entity.Task{
			ID:      1,
			Title:   "Old Title",
			Status:  entity.StatusTodo,
			Version: 1,
		}

		mockRepo.On("GetTask", mock.Anything, entity.ID(1)).
			Return(existingTask, nil).Once()

		mockRepo.On("UpdateTask", mock.Anything, mock.MatchedBy(func(t entity.Task) bool {
			return t.Title == "Updated Title" && t.Version == 2
		})).Return(nil).Once()

		mockCache.On("Delete", mock.Anything, "task:1").Return(nil).Once()

		mockMetrics.On("RecordTaskUpdatedDuration", mock.Anything, mock.AnythingOfType("float64")).Return().Once()
		mockMetrics.On("IncTaskUpdated", mock.Anything, "success", "none").Return().Once()

		svc := service.NewService(mockRepo, mockCache, nopLogger, mockMetrics, vld)

		resp, err := svc.UpdateTask(context.Background(), req)

		assert.NoError(t, err)
		assert.Equal(t, "Updated Title", resp.Task.Title)
		assert.Equal(t, int16(2), resp.Task.Version)

		mockRepo.AssertExpectations(t)
		mockCache.AssertExpectations(t)
		mockMetrics.AssertExpectations(t)
	})

	t.Run("Success - Update status only", func(t *testing.T) {
		mockRepo := new(mocks.Repository)
		mockCache := new(mocks.CacheStore)
		mockMetrics := new(mocks.Metrics)

		newStatus := entity.StatusInProgress
		req := param.UpdateTaskRequest{
			ID:      1,
			Status:  &newStatus,
			Version: 1,
		}

		existingTask := entity.Task{
			ID:      1,
			Title:   "My Task",
			Status:  entity.StatusTodo,
			Version: 1,
		}

		mockRepo.On("GetTask", mock.Anything, entity.ID(1)).
			Return(existingTask, nil).Once()

		mockRepo.On("UpdateTask", mock.Anything, mock.MatchedBy(func(t entity.Task) bool {
			return t.Status == entity.StatusInProgress && t.Version == 2
		})).Return(nil).Once()

		mockCache.On("Delete", mock.Anything, "task:1").Return(nil).Once()

		mockMetrics.On("RecordTaskUpdatedDuration", mock.Anything, mock.AnythingOfType("float64")).Return().Once()
		mockMetrics.On("IncTaskUpdated", mock.Anything, "success", "none").Return().Once()

		svc := service.NewService(mockRepo, mockCache, nopLogger, mockMetrics, vld)

		resp, err := svc.UpdateTask(context.Background(), req)

		assert.NoError(t, err)
		assert.Equal(t, entity.StatusInProgress, resp.Task.Status)
		assert.Equal(t, int16(2), resp.Task.Version)

		mockRepo.AssertExpectations(t)
	})

	t.Run("Fail - Version conflict (stale data)", func(t *testing.T) {
		mockRepo := new(mocks.Repository)
		mockCache := new(mocks.CacheStore)
		mockMetrics := new(mocks.Metrics)

		newTitle := "Updated"
		req := param.UpdateTaskRequest{
			ID:      1,
			Title:   &newTitle,
			Version: 1,
		}

		existingTask := entity.Task{
			ID:      1,
			Title:   "Old",
			Status:  entity.StatusTodo,
			Version: 2,
		}

		mockRepo.On("GetTask", mock.Anything, entity.ID(1)).
			Return(existingTask, nil).Once()

		mockMetrics.On("RecordTaskUpdatedDuration", mock.Anything, mock.AnythingOfType("float64")).Return().Once()
		mockMetrics.On("IncTaskUpdated", mock.Anything, "fail", "version_conflict").Return().Once()

		svc := service.NewService(mockRepo, mockCache, nopLogger, mockMetrics, vld)

		resp, err := svc.UpdateTask(context.Background(), req)

		assert.Error(t, err)
		assert.Empty(t, resp)
		assert.True(t, richerror.IsKind(err, richerror.KindConflict))
		mockRepo.AssertNotCalled(t, "UpdateTask")
		mockCache.AssertNotCalled(t, "Delete")
	})

	t.Run("Fail - Task not found", func(t *testing.T) {
		mockRepo := new(mocks.Repository)
		mockCache := new(mocks.CacheStore)
		mockMetrics := new(mocks.Metrics)

		newTitle := "Updated"
		req := param.UpdateTaskRequest{
			ID:      999,
			Title:   &newTitle,
			Version: 1,
		}

		notFoundErr := richerror.New("repo.GetTask").
			WithKind(richerror.KindNotFound).
			WithMessage("task not found")

		mockRepo.On("GetTask", mock.Anything, entity.ID(999)).
			Return(entity.Task{}, notFoundErr).Once()

		mockMetrics.On("RecordTaskUpdatedDuration", mock.Anything, mock.AnythingOfType("float64")).Return().Once()
		mockMetrics.On("IncTaskUpdated", mock.Anything, "fail", "not_found").Return().Once()

		svc := service.NewService(mockRepo, mockCache, nopLogger, mockMetrics, vld)

		resp, err := svc.UpdateTask(context.Background(), req)

		assert.Error(t, err)
		assert.Empty(t, resp)
		assert.True(t, richerror.IsKind(err, richerror.KindNotFound))
		mockRepo.AssertNotCalled(t, "UpdateTask")
		mockCache.AssertNotCalled(t, "Delete")
	})

	t.Run("Fail - DB error on GetTask", func(t *testing.T) {
		mockRepo := new(mocks.Repository)
		mockCache := new(mocks.CacheStore)
		mockMetrics := new(mocks.Metrics)

		newTitle := "Updated"
		req := param.UpdateTaskRequest{
			ID:      1,
			Title:   &newTitle,
			Version: 1,
		}

		mockRepo.On("GetTask", mock.Anything, entity.ID(1)).
			Return(entity.Task{}, errors.New("connection refused")).Once()

		mockMetrics.On("RecordTaskUpdatedDuration", mock.Anything, mock.AnythingOfType("float64")).Return().Once()
		mockMetrics.On("IncTaskUpdated", mock.Anything, "fail", "db_error").Return().Once()

		svc := service.NewService(mockRepo, mockCache, nopLogger, mockMetrics, vld)

		resp, err := svc.UpdateTask(context.Background(), req)

		assert.Error(t, err)
		assert.Empty(t, resp)
		assert.False(t, richerror.IsKind(err, richerror.KindNotFound))
		mockRepo.AssertNotCalled(t, "UpdateTask")
		mockCache.AssertNotCalled(t, "Delete")
	})

	t.Run("Fail - Zero ID validation error", func(t *testing.T) {
		mockRepo := new(mocks.Repository)
		mockCache := new(mocks.CacheStore)
		mockMetrics := new(mocks.Metrics)

		req := param.UpdateTaskRequest{
			ID:      0,
			Version: 1,
		}

		mockMetrics.On("RecordTaskUpdatedDuration", mock.Anything, mock.AnythingOfType("float64")).Return().Once()
		mockMetrics.On("IncTaskUpdated", mock.Anything, "fail", "validation_error").Return().Once()

		svc := service.NewService(mockRepo, mockCache, nopLogger, mockMetrics, vld)

		resp, err := svc.UpdateTask(context.Background(), req)

		assert.Error(t, err)
		assert.Empty(t, resp)
		mockRepo.AssertNotCalled(t, "GetTask")
	})

	t.Run("Fail - Zero version validation error", func(t *testing.T) {
		mockRepo := new(mocks.Repository)
		mockCache := new(mocks.CacheStore)
		mockMetrics := new(mocks.Metrics)

		req := param.UpdateTaskRequest{
			ID:      1,
			Version: 0,
		}

		mockMetrics.On("RecordTaskUpdatedDuration", mock.Anything, mock.AnythingOfType("float64")).Return().Once()
		mockMetrics.On("IncTaskUpdated", mock.Anything, "fail", "validation_error").Return().Once()

		svc := service.NewService(mockRepo, mockCache, nopLogger, mockMetrics, vld)

		resp, err := svc.UpdateTask(context.Background(), req)

		assert.Error(t, err)
		assert.Empty(t, resp)
		mockRepo.AssertNotCalled(t, "GetTask")
	})

	t.Run("Fail - Invalid status validation error", func(t *testing.T) {
		mockRepo := new(mocks.Repository)
		mockCache := new(mocks.CacheStore)
		mockMetrics := new(mocks.Metrics)

		invalidStatus := entity.TaskStatus("INVALID")
		req := param.UpdateTaskRequest{
			ID:      1,
			Status:  &invalidStatus,
			Version: 1,
		}

		mockMetrics.On("RecordTaskUpdatedDuration", mock.Anything, mock.AnythingOfType("float64")).Return().Once()
		mockMetrics.On("IncTaskUpdated", mock.Anything, "fail", "validation_error").Return().Once()

		svc := service.NewService(mockRepo, mockCache, nopLogger, mockMetrics, vld)

		resp, err := svc.UpdateTask(context.Background(), req)

		assert.Error(t, err)
		assert.Empty(t, resp)
		mockRepo.AssertNotCalled(t, "GetTask")
	})

	t.Run("Fail - DB error on update", func(t *testing.T) {
		mockRepo := new(mocks.Repository)
		mockCache := new(mocks.CacheStore)
		mockMetrics := new(mocks.Metrics)

		newTitle := "Updated"
		req := param.UpdateTaskRequest{
			ID:      1,
			Title:   &newTitle,
			Version: 1,
		}

		existingTask := entity.Task{
			ID:      1,
			Title:   "Old",
			Status:  entity.StatusTodo,
			Version: 1,
		}

		mockRepo.On("GetTask", mock.Anything, entity.ID(1)).
			Return(existingTask, nil).Once()

		mockRepo.On("UpdateTask", mock.Anything, mock.AnythingOfType("entity.Task")).
			Return(errors.New("db error")).Once()

		mockMetrics.On("RecordTaskUpdatedDuration", mock.Anything, mock.AnythingOfType("float64")).Return().Once()
		mockMetrics.On("IncTaskUpdated", mock.Anything, "fail", "db_error").Return().Once()

		svc := service.NewService(mockRepo, mockCache, nopLogger, mockMetrics, vld)

		resp, err := svc.UpdateTask(context.Background(), req)

		assert.Error(t, err)
		assert.Empty(t, resp)
		mockCache.AssertNotCalled(t, "Delete")
	})
}

func TestService_GetTask(t *testing.T) {
	nopLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	vld := validation.NewValidator()

	t.Run("Success - Cache hit with valid data", func(t *testing.T) {
		mockRepo := new(mocks.Repository)
		mockCache := new(mocks.CacheStore)
		mockMetrics := new(mocks.Metrics)

		cachedTask := entity.Task{
			ID:      1,
			Title:   "Cached Task",
			Status:  entity.StatusTodo,
			Version: 1,
		}

		mockCache.On("Get", mock.Anything, "task:1", mock.AnythingOfType("*entity.Task")).
			Run(func(args mock.Arguments) {
				dest := args.Get(2).(*entity.Task)
				*dest = cachedTask
			}).Return(nil).Once()

		mockMetrics.On("RecordTaskFetchedDuration", mock.Anything, mock.AnythingOfType("float64")).Return().Once()
		mockMetrics.On("IncTaskFetched", mock.Anything, "success", "cache").Return().Once()

		svc := service.NewService(mockRepo, mockCache, nopLogger, mockMetrics, vld)

		resp, err := svc.GetTask(context.Background(), param.GetTaskByIDRequest{ID: 1})

		assert.NoError(t, err)
		assert.Equal(t, entity.ID(1), resp.Task.ID)
		assert.Equal(t, "Cached Task", resp.Task.Title)
		assert.Equal(t, cachedTask.Status, resp.Task.Status)

		mockRepo.AssertNotCalled(t, "GetTask")
		mockMetrics.AssertExpectations(t)
	})

	t.Run("Fail - Cache penetration (cached not-found marker)", func(t *testing.T) {
		mockRepo := new(mocks.Repository)
		mockCache := new(mocks.CacheStore)
		mockMetrics := new(mocks.Metrics)

		mockCache.On("Get", mock.Anything, "task:999", mock.AnythingOfType("*entity.Task")).
			Run(func(args mock.Arguments) {
				dest := args.Get(2).(*entity.Task)
				*dest = entity.Task{ID: 0}
			}).Return(nil).Once()

		mockMetrics.On("RecordTaskFetchedDuration", mock.Anything, mock.AnythingOfType("float64")).Return().Once()
		mockMetrics.On("IncTaskFetched", mock.Anything, "fail", "cache_hit_not_found").Return().Once()

		svc := service.NewService(mockRepo, mockCache, nopLogger, mockMetrics, vld)

		resp, err := svc.GetTask(context.Background(), param.GetTaskByIDRequest{ID: 999})

		assert.Error(t, err)
		assert.Empty(t, resp)
		assert.True(t, richerror.IsKind(err, richerror.KindNotFound))

		mockRepo.AssertNotCalled(t, "GetTask")
	})

	t.Run("Success - Cache miss, DB hit, cache set", func(t *testing.T) {
		mockRepo := new(mocks.Repository)
		mockCache := new(mocks.CacheStore)
		mockMetrics := new(mocks.Metrics)

		dbTask := entity.Task{
			ID:      1,
			Title:   "DB Task",
			Status:  entity.StatusInProgress,
			Version: 3,
		}

		mockCache.On("Get", mock.Anything, "task:1", mock.AnythingOfType("*entity.Task")).
			Return(errors.New("cache miss")).Once()

		mockRepo.On("GetTask", mock.Anything, entity.ID(1)).
			Return(dbTask, nil).Once()

		mockCache.On("Set", mock.Anything, "task:1", dbTask, 5*time.Minute).
			Return(nil).Once()

		mockMetrics.On("RecordTaskFetchedDuration", mock.Anything, mock.AnythingOfType("float64")).Return().Once()
		mockMetrics.On("IncTaskFetched", mock.Anything, "success", "db").Return().Once()

		svc := service.NewService(mockRepo, mockCache, nopLogger, mockMetrics, vld)

		resp, err := svc.GetTask(context.Background(), param.GetTaskByIDRequest{ID: 1})

		assert.NoError(t, err)
		assert.Equal(t, entity.ID(1), resp.Task.ID)
		assert.Equal(t, "DB Task", resp.Task.Title)
		assert.Equal(t, entity.StatusInProgress, resp.Task.Status)

		mockRepo.AssertExpectations(t)
		mockCache.AssertExpectations(t)
	})

	t.Run("Fail - Task not found (DB) caches not-found marker", func(t *testing.T) {
		mockRepo := new(mocks.Repository)
		mockCache := new(mocks.CacheStore)
		mockMetrics := new(mocks.Metrics)

		mockCache.On("Get", mock.Anything, "task:999", mock.AnythingOfType("*entity.Task")).
			Return(errors.New("cache miss")).Once()

		notFoundErr := richerror.New("repo.GetTask").
			WithKind(richerror.KindNotFound).
			WithMessage("task not found")

		mockRepo.On("GetTask", mock.Anything, entity.ID(999)).
			Return(entity.Task{}, notFoundErr).Once()

		mockCache.On("Set", mock.Anything, "task:999", entity.Task{ID: 0}, 1*time.Minute).
			Return(nil).Once()

		mockMetrics.On("RecordTaskFetchedDuration", mock.Anything, mock.AnythingOfType("float64")).Return().Once()
		mockMetrics.On("IncTaskFetched", mock.Anything, "fail", "db").Return().Once()

		svc := service.NewService(mockRepo, mockCache, nopLogger, mockMetrics, vld)

		resp, err := svc.GetTask(context.Background(), param.GetTaskByIDRequest{ID: 999})

		assert.Error(t, err)
		assert.Empty(t, resp)
		assert.True(t, richerror.IsKind(err, richerror.KindNotFound))

		mockCache.AssertExpectations(t)
	})

	t.Run("Fail - DB error", func(t *testing.T) {
		mockRepo := new(mocks.Repository)
		mockCache := new(mocks.CacheStore)
		mockMetrics := new(mocks.Metrics)

		mockCache.On("Get", mock.Anything, "task:1", mock.AnythingOfType("*entity.Task")).
			Return(errors.New("cache miss")).Once()

		mockRepo.On("GetTask", mock.Anything, entity.ID(1)).
			Return(entity.Task{}, errors.New("connection refused")).Once()

		mockMetrics.On("RecordTaskFetchedDuration", mock.Anything, mock.AnythingOfType("float64")).Return().Once()
		mockMetrics.On("IncTaskFetched", mock.Anything, "fail", "db").Return().Once()

		svc := service.NewService(mockRepo, mockCache, nopLogger, mockMetrics, vld)

		resp, err := svc.GetTask(context.Background(), param.GetTaskByIDRequest{ID: 1})

		assert.Error(t, err)
		assert.Empty(t, resp)

		mockCache.AssertNotCalled(t, "Set")
	})

	t.Run("Success - Cache set failure is best-effort", func(t *testing.T) {
		mockRepo := new(mocks.Repository)
		mockCache := new(mocks.CacheStore)
		mockMetrics := new(mocks.Metrics)

		dbTask := entity.Task{
			ID:      1,
			Title:   "Task",
			Status:  entity.StatusDone,
			Version: 1,
		}

		mockCache.On("Get", mock.Anything, "task:1", mock.AnythingOfType("*entity.Task")).
			Return(errors.New("cache miss")).Once()

		mockRepo.On("GetTask", mock.Anything, entity.ID(1)).
			Return(dbTask, nil).Once()

		mockCache.On("Set", mock.Anything, "task:1", dbTask, 5*time.Minute).
			Return(errors.New("redis down")).Once()

		mockMetrics.On("RecordTaskFetchedDuration", mock.Anything, mock.AnythingOfType("float64")).Return().Once()
		mockMetrics.On("IncTaskFetched", mock.Anything, "success", "db").Return().Once()

		svc := service.NewService(mockRepo, mockCache, nopLogger, mockMetrics, vld)

		resp, err := svc.GetTask(context.Background(), param.GetTaskByIDRequest{ID: 1})

		assert.NoError(t, err)
		assert.Equal(t, entity.ID(1), resp.Task.ID)
		assert.Equal(t, entity.StatusDone, resp.Task.Status)
	})
}
