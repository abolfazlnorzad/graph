package service_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/mock"

	"github.com/abolfazlnorzad/graph/entity"
	"github.com/abolfazlnorzad/graph/param"
	"github.com/abolfazlnorzad/graph/service"
	"github.com/abolfazlnorzad/graph/service/mocks"
	"github.com/abolfazlnorzad/graph/validation"
)

func BenchmarkCreateTask(b *testing.B) {
	nopLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	vld := validation.NewValidator()

	mockRepo := new(mocks.Repository)
	mockCache := new(mocks.CacheStore)
	mockMetrics := new(mocks.Metrics)

	createdTask := entity.Task{
		ID: 1, Title: "Benchmark Task", Status: entity.StatusTodo, Version: 1,
	}

	mockRepo.On("CreateTask", mock.Anything, mock.AnythingOfType("entity.Task")).
		Return(createdTask, nil)
	mockCache.On("DeleteByPrefix", mock.Anything, mock.Anything).Return(nil)
	mockCache.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	mockMetrics.On("RecordTaskCreatedDuration", mock.Anything, mock.AnythingOfType("float64")).Return()
	mockMetrics.On("IncTaskCreated", mock.Anything, mock.Anything, mock.Anything).Return()
	mockMetrics.On("IncTasksCount", mock.Anything, mock.Anything, mock.Anything).Return()

	svc := service.NewService(mockRepo, mockCache, nopLogger, mockMetrics, vld)
	req := param.CreateTaskRequest{Title: "Benchmark", Status: entity.StatusTodo}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.CreateTask(context.Background(), req)
	}
}

func BenchmarkGetTask(b *testing.B) {
	nopLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	vld := validation.NewValidator()

	mockRepo := new(mocks.Repository)
	mockCache := new(mocks.CacheStore)
	mockMetrics := new(mocks.Metrics)

	cachedTask := entity.Task{
		ID: 1, Title: "Cached Task", Status: entity.StatusTodo, Version: 1,
	}

	mockCache.On("Get", mock.Anything, mock.Anything, mock.Anything).
		Run(func(args mock.Arguments) {
			dest := args.Get(2).(*entity.Task)
			*dest = cachedTask
		}).Return(nil)
	mockMetrics.On("RecordTaskFetchedDuration", mock.Anything, mock.AnythingOfType("float64")).Return()
	mockMetrics.On("IncTaskFetched", mock.Anything, mock.Anything, mock.Anything).Return()

	svc := service.NewService(mockRepo, mockCache, nopLogger, mockMetrics, vld)
	req := param.GetTaskByIDRequest{ID: 1}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.GetTask(context.Background(), req)
	}
}

func BenchmarkUpdateTask(b *testing.B) {
	nopLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	vld := validation.NewValidator()

	mockRepo := new(mocks.Repository)
	mockCache := new(mocks.CacheStore)
	mockMetrics := new(mocks.Metrics)

	existingTask := entity.Task{
		ID: 1, Title: "Old Title", Status: entity.StatusTodo, Version: 1,
	}

	mockRepo.On("GetTask", mock.Anything, mock.Anything).
		Return(existingTask, nil)
	mockRepo.On("UpdateTask", mock.Anything, mock.Anything).
		Return(nil)
	mockCache.On("DeleteByPrefix", mock.Anything, mock.Anything).Return(nil)
	mockCache.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockMetrics.On("RecordTaskUpdatedDuration", mock.Anything, mock.AnythingOfType("float64")).Return()
	mockMetrics.On("IncTaskUpdated", mock.Anything, mock.Anything, mock.Anything).Return()

	svc := service.NewService(mockRepo, mockCache, nopLogger, mockMetrics, vld)

	newTitle := "Updated"
	req := param.UpdateTaskRequest{ID: 1, Title: &newTitle, Version: 1}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.UpdateTask(context.Background(), req)
	}
}

func BenchmarkDeleteTask(b *testing.B) {
	nopLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	vld := validation.NewValidator()

	mockRepo := new(mocks.Repository)
	mockCache := new(mocks.CacheStore)
	mockMetrics := new(mocks.Metrics)

	mockRepo.On("DeleteTask", mock.Anything, mock.Anything).Return(nil)
	mockCache.On("DeleteByPrefix", mock.Anything, mock.Anything).Return(nil)
	mockCache.On("Delete", mock.Anything, mock.Anything).Return(nil)
	mockMetrics.On("RecordTaskDeletedDuration", mock.Anything, mock.AnythingOfType("float64")).Return()
	mockMetrics.On("IncTaskDeleted", mock.Anything, mock.Anything, mock.Anything).Return()
	mockMetrics.On("DecTasksCount", mock.Anything, mock.Anything, mock.Anything).Return()

	svc := service.NewService(mockRepo, mockCache, nopLogger, mockMetrics, vld)
	req := param.DeleteTaskRequest{ID: 1}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.DeleteTask(context.Background(), req)
	}
}

func BenchmarkListTask(b *testing.B) {
	nopLogger := slog.New(slog.NewTextHandler(io.Discard, nil))
	vld := validation.NewValidator()

	mockRepo := new(mocks.Repository)
	mockCache := new(mocks.CacheStore)
	mockMetrics := new(mocks.Metrics)

	tasks := []entity.Task{
		{ID: 1, Title: "Task 1", Status: entity.StatusTodo, Version: 1},
		{ID: 2, Title: "Task 2", Status: entity.StatusDone, Version: 2},
	}

	mockCache.On("Get", mock.Anything, mock.Anything, mock.Anything).
		Return(errors.New("cache miss"))
	mockRepo.On("ListTask", mock.Anything, mock.Anything).
		Return(tasks, int64(2), nil)
	mockCache.On("Set", mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(nil)
	mockMetrics.On("RecordTaskListedDuration", mock.Anything, mock.AnythingOfType("float64")).Return()
	mockMetrics.On("IncTaskListed", mock.Anything, mock.Anything).Return()

	svc := service.NewService(mockRepo, mockCache, nopLogger, mockMetrics, vld)
	req := param.ListTasksRequest{
		Pagination: param.PaginationRequest{PageNumber: 1, PageSize: 10},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		svc.ListTask(context.Background(), req)
	}
}
