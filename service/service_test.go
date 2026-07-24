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
