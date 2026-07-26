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
		pool.Exec(ctx, "DELETE FROM task_audit_logs")
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

	_, err = pool.Exec(ctx, `
		CREATE TABLE IF NOT EXISTS task_audit_logs (
			id BIGSERIAL PRIMARY KEY,
			task_id BIGINT NOT NULL,
			action VARCHAR(50) NOT NULL,
			previous_state JSONB,
			new_state JSONB,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
			CONSTRAINT fk_audit_task FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE CASCADE
		)
	`)
	require.NoError(t, err)

	db := &postgresdb.Database{Pool: pool}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	return pgrepo.NewTaskRepo(db, logger)
}

// createTaskHelper is a test helper that creates a task using CreateTaskWithAuditLog.
func createTaskHelper(t *testing.T, repo *pgrepo.TaskRepo, title string, status entity.TaskStatus) entity.Task {
	t.Helper()
	created, err := repo.CreateTaskWithAuditLog(context.Background(), entity.Task{
		Title:   title,
		Status:  status,
		Version: 1,
	}, entity.TaskAuditLog{
		Action: entity.ActionCreate,
		NewState: map[string]any{
			"title":  title,
			"status": status,
		},
	})
	require.NoError(t, err)
	return created
}

func TestTaskRepo_CreateTaskWithAuditLog(t *testing.T) {
	repo := setupPostgresTest(t)
	ctx := context.Background()

	created, err := repo.CreateTaskWithAuditLog(ctx, entity.Task{
		Title:   "Audit Create Test",
		Status:  entity.StatusTodo,
		Version: 1,
	}, entity.TaskAuditLog{
		Action: entity.ActionCreate,
		NewState: map[string]any{
			"title":  "Audit Create Test",
			"status": "TODO",
		},
	})

	require.NoError(t, err)
	assert.NotZero(t, created.ID)
	assert.Equal(t, "Audit Create Test", created.Title)

	// Verify audit log was created
	logs, total, err := repo.GetAuditLogsByTaskID(ctx, created.ID, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, logs, 1)
	assert.Equal(t, entity.ActionCreate, logs[0].Action)
	assert.Equal(t, created.ID, logs[0].TaskID)
}

func TestTaskRepo_CreateTaskWithAuditLog_VerifyTaskAndAudit(t *testing.T) {
	repo := setupPostgresTest(t)
	ctx := context.Background()

	// Create task with audit log
	created, err := repo.CreateTaskWithAuditLog(ctx, entity.Task{
		Title:   "Transaction Test",
		Status:  entity.StatusInProgress,
		Version: 1,
	}, entity.TaskAuditLog{
		Action: entity.ActionCreate,
		NewState: map[string]any{
			"title":  "Transaction Test",
			"status": "IN_PROGRESS",
		},
	})
	require.NoError(t, err)

	// Verify task exists in DB
	fetched, err := repo.GetTask(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Transaction Test", fetched.Title)
	assert.Equal(t, entity.StatusInProgress, fetched.Status)

	// Verify audit log exists
	logs, total, err := repo.GetAuditLogsByTaskID(ctx, created.ID, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, logs, 1)
	assert.Equal(t, entity.ActionCreate, logs[0].Action)

	// Verify new_state was stored correctly
	assert.Equal(t, "Transaction Test", logs[0].NewState["title"])
}

func TestTaskRepo_GetTask(t *testing.T) {
	repo := setupPostgresTest(t)
	ctx := context.Background()

	created := createTaskHelper(t, repo, "Get Test", entity.StatusInProgress)

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

func TestTaskRepo_UpdateTaskWithAuditLog(t *testing.T) {
	repo := setupPostgresTest(t)
	ctx := context.Background()

	created := createTaskHelper(t, repo, "Audit Update Test", entity.StatusTodo)

	err := repo.UpdateTaskWithAuditLog(ctx, entity.Task{
		ID:      created.ID,
		Title:   "Updated Title",
		Status:  entity.StatusDone,
		Version: 2,
	}, entity.TaskAuditLog{
		Action: entity.ActionUpdate,
		PreviousState: map[string]any{
			"title":  "Audit Update Test",
			"status": "TODO",
		},
		NewState: map[string]any{
			"title":  "Updated Title",
			"status": "DONE",
		},
	})
	require.NoError(t, err)

	// Verify task was updated
	fetched, err := repo.GetTask(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Updated Title", fetched.Title)
	assert.Equal(t, entity.StatusDone, fetched.Status)

	// Verify audit log was created
	logs, total, err := repo.GetAuditLogsByTaskID(ctx, created.ID, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, logs, 1)
	assert.Equal(t, entity.ActionUpdate, logs[0].Action)
}

func TestTaskRepo_UpdateTaskWithAuditLog_VersionConflict(t *testing.T) {
	repo := setupPostgresTest(t)
	ctx := context.Background()

	created := createTaskHelper(t, repo, "Conflict Audit Test", entity.StatusTodo)

	// Try to update with wrong version
	err := repo.UpdateTaskWithAuditLog(ctx, entity.Task{
		ID:      created.ID,
		Title:   "Should Fail",
		Version: 1, // wrong version, should be 2
	}, entity.TaskAuditLog{
		Action:   entity.ActionUpdate,
		NewState: map[string]any{"title": "Should Fail"},
	})
	assert.Error(t, err)

	// Verify task was NOT updated
	fetched, err := repo.GetTask(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, "Conflict Audit Test", fetched.Title)
}

func TestTaskRepo_DeleteTask(t *testing.T) {
	repo := setupPostgresTest(t)
	ctx := context.Background()

	created := createTaskHelper(t, repo, "Delete Test", entity.StatusTodo)

	err := repo.DeleteTask(ctx, created.ID)
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
		createTaskHelper(t, repo, fmt.Sprintf("List Task %d", i), entity.StatusTodo)
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

	createTaskHelper(t, repo, "Todo", entity.StatusTodo)
	createTaskHelper(t, repo, "Done", entity.StatusDone)

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
		createTaskHelper(t, repo, fmt.Sprintf("Page Task %d", i), entity.StatusTodo)
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

func TestTaskRepo_GetAuditLogsByTaskID(t *testing.T) {
	repo := setupPostgresTest(t)
	ctx := context.Background()

	created, err := repo.CreateTaskWithAuditLog(ctx, entity.Task{
		Title:   "Audit Log Test",
		Status:  entity.StatusTodo,
		Version: 1,
	}, entity.TaskAuditLog{
		Action:   entity.ActionCreate,
		NewState: map[string]any{"title": "Audit Log Test"},
	})
	require.NoError(t, err)

	// Update to create a second audit log
	err = repo.UpdateTaskWithAuditLog(ctx, entity.Task{
		ID:      created.ID,
		Title:   "Updated",
		Version: 2,
	}, entity.TaskAuditLog{
		Action:   entity.ActionUpdate,
		NewState: map[string]any{"title": "Updated"},
	})
	require.NoError(t, err)

	// Get audit logs
	logs, total, err := repo.GetAuditLogsByTaskID(ctx, created.ID, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, logs, 2)

	// Verify order: most recent first
	assert.Equal(t, entity.ActionUpdate, logs[0].Action)
	assert.Equal(t, entity.ActionCreate, logs[1].Action)
}

func TestTaskRepo_GetAuditLogsByTaskID_Pagination(t *testing.T) {
	repo := setupPostgresTest(t)
	ctx := context.Background()

	created := createTaskHelper(t, repo, "Pagination Audit Test", entity.StatusTodo)

	// Create 5 audit logs via updates
	for i := 0; i < 5; i++ {
		err := repo.UpdateTaskWithAuditLog(ctx, entity.Task{
			ID:      created.ID,
			Title:   fmt.Sprintf("Update %d", i),
			Version: int16(i + 2),
		}, entity.TaskAuditLog{
			Action:   entity.ActionUpdate,
			NewState: map[string]any{"iteration": i},
		})
		require.NoError(t, err)
	}

	// Get page 1 (2 items)
	logs1, total, err := repo.GetAuditLogsByTaskID(ctx, created.ID, 1, 2)
	require.NoError(t, err)
	assert.Equal(t, int64(5), total)
	assert.Len(t, logs1, 2)

	// Get page 3 (1 item)
	logs3, _, err := repo.GetAuditLogsByTaskID(ctx, created.ID, 3, 2)
	require.NoError(t, err)
	assert.Len(t, logs3, 1)
}

func TestTaskRepo_GetAuditLogsByTaskID_Empty(t *testing.T) {
	repo := setupPostgresTest(t)
	ctx := context.Background()

	logs, total, err := repo.GetAuditLogsByTaskID(ctx, 99999, 1, 10)
	require.NoError(t, err)
	assert.Equal(t, int64(0), total)
	assert.Empty(t, logs)
}
