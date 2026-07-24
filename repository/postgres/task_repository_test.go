package postgres_test

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/abolfazlnorzad/graph/entity"
	"github.com/abolfazlnorzad/graph/pkg/postgresdb"
	pgrepo "github.com/abolfazlnorzad/graph/repository/postgres"
	"github.com/abolfazlnorzad/graph/service"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func setupPostgresTest(t *testing.T) *pgrepo.TaskRepo {
	t.Helper()
	ctx := context.Background()

	pgContainer, err := tcpostgres.RunContainer(ctx,
		testcontainers.WithImage("postgres:16-alpine"),
		tcpostgres.WithDatabase("testdb"),
		tcpostgres.WithUsername("testuser"),
		tcpostgres.WithPassword("testpass"),
		testcontainers.WithWaitStrategy(
			wait.ForLog("database system is ready to accept connections").
				WithOccurrence(2).
				WithStartupTimeout(30*time.Second),
		),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = pgContainer.Terminate(ctx)
	})

	connStr, err := pgContainer.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := pgxpool.New(ctx, connStr)
	require.NoError(t, err)

	t.Cleanup(func() {
		pool.Exec(ctx, "DELETE FROM tasks")
		pool.Close()
	})

	// Run migration
	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS tasks (
			id BIGSERIAL PRIMARY KEY,
			title VARCHAR(255) NOT NULL,
			description TEXT,
			status VARCHAR(50) NOT NULL DEFAULT 'TODO',
			assignee VARCHAR(255),
			version SMALLINT NOT NULL DEFAULT 1,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
			deleted_at TIMESTAMP WITH TIME ZONE
		)
	`)
	require.NoError(t, err)

	db := &postgresdb.Database{Pool: pool}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	return pgrepo.NewTaskRepo(db, logger)
}

func TestTaskRepo_CreateTask(t *testing.T) {
	repo := setupPostgresTest(t)
	ctx := context.Background()

	created, err := repo.CreateTask(ctx, entity.Task{
		Title:   "Integration Test Task",
		Status:  entity.StatusTodo,
		Version: 1,
	})

	require.NoError(t, err)
	assert.NotZero(t, created.ID)
	assert.Equal(t, "Integration Test Task", created.Title)
	assert.Equal(t, entity.StatusTodo, created.Status)
	assert.False(t, created.CreatedAt.IsZero())
	assert.False(t, created.UpdatedAt.IsZero())
}

func TestTaskRepo_GetTask(t *testing.T) {
	repo := setupPostgresTest(t)
	ctx := context.Background()

	created, err := repo.CreateTask(ctx, entity.Task{
		Title:   "Get Test",
		Status:  entity.StatusInProgress,
		Version: 1,
	})
	require.NoError(t, err)

	fetched, err := repo.GetTask(ctx, created.ID)

	require.NoError(t, err)
	assert.Equal(t, created.ID, fetched.ID)
	assert.Equal(t, "Get Test", fetched.Title)
	assert.Equal(t, entity.StatusInProgress, fetched.Status)
}

func TestTaskRepo_GetTask_NotFound(t *testing.T) {
	repo := setupPostgresTest(t)
	ctx := context.Background()

	_, err := repo.GetTask(ctx, 99999)
	assert.Error(t, err)
}

func TestTaskRepo_UpdateTask(t *testing.T) {
	repo := setupPostgresTest(t)
	ctx := context.Background()

	created, err := repo.CreateTask(ctx, entity.Task{
		Title:   "Update Test",
		Status:  entity.StatusTodo,
		Version: 1,
	})
	require.NoError(t, err)

	newTitle := "Updated Title"
	newStatus := entity.StatusDone
	err = repo.UpdateTask(ctx, entity.Task{
		ID:      created.ID,
		Title:   newTitle,
		Status:  newStatus,
		Version: 2,
	})
	require.NoError(t, err)

	fetched, err := repo.GetTask(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, newTitle, fetched.Title)
	assert.Equal(t, newStatus, fetched.Status)
}

func TestTaskRepo_UpdateTask_VersionConflict(t *testing.T) {
	repo := setupPostgresTest(t)
	ctx := context.Background()

	created, err := repo.CreateTask(ctx, entity.Task{
		Title:   "Conflict Test",
		Status:  entity.StatusTodo,
		Version: 1,
	})
	require.NoError(t, err)

	err = repo.UpdateTask(ctx, entity.Task{
		ID:      created.ID,
		Title:   "Should Fail",
		Status:  entity.StatusTodo,
		Version: 1,
	})
	assert.Error(t, err)
}

func TestTaskRepo_DeleteTask(t *testing.T) {
	repo := setupPostgresTest(t)
	ctx := context.Background()

	created, err := repo.CreateTask(ctx, entity.Task{
		Title:   "Delete Test",
		Status:  entity.StatusTodo,
		Version: 1,
	})
	require.NoError(t, err)

	err = repo.DeleteTask(ctx, created.ID)
	require.NoError(t, err)

	_, err = repo.GetTask(ctx, created.ID)
	assert.Error(t, err)
}

func TestTaskRepo_DeleteTask_NotFound(t *testing.T) {
	repo := setupPostgresTest(t)
	ctx := context.Background()

	err := repo.DeleteTask(ctx, 99999)
	assert.Error(t, err)
}

func TestTaskRepo_ListTask(t *testing.T) {
	repo := setupPostgresTest(t)
	ctx := context.Background()

	for i := 0; i < 3; i++ {
		_, err := repo.CreateTask(ctx, entity.Task{
			Title:   fmt.Sprintf("List Task %d", i),
			Status:  entity.StatusTodo,
			Version: 1,
		})
		require.NoError(t, err)
	}

	tasks, total, err := repo.ListTask(ctx, service.ListTaskCriteria{
		PageSize:   10,
		PageNumber: 1,
	})

	require.NoError(t, err)
	assert.Len(t, tasks, 3)
	assert.Equal(t, int64(3), total)
}

func TestTaskRepo_ListTask_Empty(t *testing.T) {
	repo := setupPostgresTest(t)
	ctx := context.Background()

	tasks, total, err := repo.ListTask(ctx, service.ListTaskCriteria{
		PageSize:   10,
		PageNumber: 1,
	})

	require.NoError(t, err)
	assert.Empty(t, tasks)
	assert.Equal(t, int64(0), total)
}

func TestTaskRepo_ListTask_WithStatusFilter(t *testing.T) {
	repo := setupPostgresTest(t)
	ctx := context.Background()

	_, _ = repo.CreateTask(ctx, entity.Task{Title: "Todo", Status: entity.StatusTodo, Version: 1})
	_, _ = repo.CreateTask(ctx, entity.Task{Title: "Done", Status: entity.StatusDone, Version: 1})

	status := entity.StatusTodo
	tasks, total, err := repo.ListTask(ctx, service.ListTaskCriteria{
		PageSize:   10,
		PageNumber: 1,
		Status:     &status,
	})

	require.NoError(t, err)
	assert.Len(t, tasks, 1)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, entity.StatusTodo, tasks[0].Status)
}

func TestTaskRepo_ListTask_Pagination(t *testing.T) {
	repo := setupPostgresTest(t)
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		_, _ = repo.CreateTask(ctx, entity.Task{
			Title:   fmt.Sprintf("Page Task %d", i),
			Status:  entity.StatusTodo,
			Version: 1,
		})
	}

	tasks1, total1, err := repo.ListTask(ctx, service.ListTaskCriteria{
		PageSize:   2,
		PageNumber: 1,
	})
	require.NoError(t, err)
	assert.Len(t, tasks1, 2)
	assert.Equal(t, int64(5), total1)

	tasks3, _, err := repo.ListTask(ctx, service.ListTaskCriteria{
		PageSize:   2,
		PageNumber: 3,
	})
	require.NoError(t, err)
	assert.Len(t, tasks3, 1)
}
