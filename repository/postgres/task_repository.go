package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/abolfazlnorzad/graph/entity"
	"github.com/abolfazlnorzad/graph/pkg/msg"
	"github.com/abolfazlnorzad/graph/pkg/postgresdb"
	"github.com/abolfazlnorzad/graph/pkg/richerror"
	"github.com/abolfazlnorzad/graph/pkg/trace"
	"github.com/abolfazlnorzad/graph/service"
	"github.com/jackc/pgx/v5"
	"go.opentelemetry.io/otel/attribute"
)

var _ service.Repository = (*TaskRepo)(nil)

type TaskRepo struct {
	db     *postgresdb.Database
	logger *slog.Logger
}

func NewTaskRepo(db *postgresdb.Database, logger *slog.Logger) *TaskRepo {
	return &TaskRepo{
		db:     db,
		logger: logger,
	}
}

func (r *TaskRepo) GetTask(ctx context.Context, id entity.ID) (entity.Task, error) {
	const op = "postgres.GetTask"

	ctx, span := trace.Tracer().Start(ctx, op)
	defer span.End()

	span.SetAttributes(attribute.Int64("task.id", int64(id)))

	query := `
		SELECT id, title, description, status, assignee, version, created_at, updated_at, deleted_at
		FROM tasks
		WHERE id = $1 AND deleted_at IS NULL
	`

	var t entity.Task
	err := r.db.Pool.QueryRow(ctx, query, id).Scan(
		&t.ID, &t.Title, &t.Description, &t.Status, &t.Assignee,
		&t.Version, &t.CreatedAt, &t.UpdatedAt, &t.DeletedAt,
	)

	if err != nil {
		trace.RecordError(span, err)
		if errors.Is(err, pgx.ErrNoRows) {
			return entity.Task{}, richerror.New(op).
				WithErr(err).
				WithKind(richerror.KindNotFound).
				WithUserMsgKey(msg.ErrNotFound)
		}
		return entity.Task{}, richerror.New(op).WithErr(err)
	}

	return t, nil
}

func (r *TaskRepo) DeleteTask(ctx context.Context, id entity.ID) error {
	const op = "postgres.DeleteTask"

	ctx, span := trace.Tracer().Start(ctx, op)
	defer span.End()

	span.SetAttributes(attribute.Int64("task.id", int64(id)))

	query := `
		UPDATE tasks 
		SET deleted_at = CURRENT_TIMESTAMP 
		WHERE id = $1 AND deleted_at IS NULL
	`

	cmdTag, err := r.db.Pool.Exec(ctx, query, id)
	if err != nil {
		trace.RecordError(span, err)
		return richerror.New(op).WithErr(err)
	}

	if cmdTag.RowsAffected() == 0 {
		err := richerror.New(op).
			WithKind(richerror.KindNotFound).
			WithMessage("task not found for deletion")
		trace.RecordError(span, err)
		return err
	}

	return nil
}

// ListTask retrieves a paginated list of tasks.
//
// Current implementation uses OFFSET-based pagination which is simple but degrades
// at scale — PostgreSQL must scan and discard all rows before the offset, making
// page N progressively slower (O(N*pageSize) total work).
//
// For datasets exceeding ~1M rows, replace with keyset (cursor) pagination:
//
//	SELECT ... WHERE id > $cursor ORDER BY id LIMIT $pageSize
//
// Keyset pagination maintains O(pageSize) per query regardless of page depth,
// leverages the primary key index directly, and avoids the "last pages are slow"
// problem. The tradeoff: random page access (e.g., "go to page 47") is no longer
// possible — only forward/backward navigation. This is acceptable for infinite
// scroll UIs but not for traditional numbered pagination with random page jumps.
func (r *TaskRepo) ListTask(ctx context.Context, criteria service.ListTaskCriteria) ([]entity.Task, int64, error) {
	const op = "postgres.ListTask"

	ctx, span := trace.Tracer().Start(ctx, op)
	defer span.End()

	span.SetAttributes(
		attribute.Int("page.number", criteria.PageNumber),
		attribute.Int("page.size", criteria.PageSize),
	)

	whereClauses := []string{"deleted_at IS NULL"}
	args := []any{}
	argID := 1

	if criteria.Status != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("status = $%d", argID))
		args = append(args, *criteria.Status)
		span.SetAttributes(attribute.String("filter.status", string(*criteria.Status)))
		argID++
	}
	if criteria.Assignee != nil {
		whereClauses = append(whereClauses, fmt.Sprintf("assignee = $%d", argID))
		args = append(args, *criteria.Assignee)
		span.SetAttributes(attribute.String("filter.assignee", *criteria.Assignee))
		argID++
	}

	whereQuery := strings.Join(whereClauses, " AND ")

	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM tasks WHERE %s", whereQuery)
	var total int64
	if err := r.db.Pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		trace.RecordError(span, err)
		return nil, 0, richerror.New(op).WithErr(err)
	}

	span.SetAttributes(attribute.Int64("result.total", total))

	if total == 0 {
		return []entity.Task{}, 0, nil
	}

	limit := criteria.PageSize
	offset := (criteria.PageNumber - 1) * criteria.PageSize

	selectQuery := fmt.Sprintf(`
		SELECT id, title, description, status, assignee, version, created_at, updated_at, deleted_at
		FROM tasks 
		WHERE %s 
		ORDER BY created_at DESC 
		LIMIT $%d OFFSET $%d
	`, whereQuery, argID, argID+1)

	args = append(args, limit, offset)

	rows, err := r.db.Pool.Query(ctx, selectQuery, args...)
	if err != nil {
		trace.RecordError(span, err)
		return nil, 0, richerror.New(op).WithErr(err)
	}
	defer rows.Close()

	tasks := make([]entity.Task, 0, min(total, int64(criteria.PageSize)))
	for rows.Next() {
		var t entity.Task
		err := rows.Scan(
			&t.ID, &t.Title, &t.Description, &t.Status, &t.Assignee,
			&t.Version, &t.CreatedAt, &t.UpdatedAt, &t.DeletedAt,
		)
		if err != nil {
			trace.RecordError(span, err)
			return nil, 0, richerror.New(op).WithErr(err)
		}
		tasks = append(tasks, t)
	}

	if err = rows.Err(); err != nil {
		trace.RecordError(span, err)
		return nil, 0, richerror.New(op).WithErr(err)
	}

	span.SetAttributes(attribute.Int("result.count", len(tasks)))
	return tasks, total, nil
}

func (r *TaskRepo) CreateTaskWithAuditLog(ctx context.Context, t entity.Task, auditLog entity.TaskAuditLog) (entity.Task, error) {
	const op = "postgres.CreateTaskWithAuditLog"

	ctx, span := trace.Tracer().Start(ctx, op)
	defer span.End()

	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		trace.RecordError(span, err)
		return entity.Task{}, richerror.New(op).WithErr(err)
	}
	defer tx.Rollback(ctx)

	query := `
		INSERT INTO tasks (title, description, status, assignee, version)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`

	err = tx.QueryRow(ctx, query,
		t.Title, t.Description, t.Status, t.Assignee, t.Version,
	).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)

	if err != nil {
		trace.RecordError(span, err)
		return entity.Task{}, richerror.New(op).
			WithErr(err).
			WithKind(richerror.KindUnexpected).
			WithUserMsgKey(msg.ErrUnexpected)
	}

	prevJSON, _ := json.Marshal(auditLog.PreviousState)
	newJSON, _ := json.Marshal(auditLog.NewState)

	auditQuery := `
		INSERT INTO task_audit_logs (task_id, action, previous_state, new_state)
		VALUES ($1, $2, $3, $4)
	`
	_, err = tx.Exec(ctx, auditQuery, t.ID, auditLog.Action, prevJSON, newJSON)
	if err != nil {
		trace.RecordError(span, err)
		return entity.Task{}, richerror.New(op).WithErr(err)
	}

	if err := tx.Commit(ctx); err != nil {
		trace.RecordError(span, err)
		return entity.Task{}, richerror.New(op).WithErr(err)
	}

	span.SetAttributes(attribute.Int64("task.id", int64(t.ID)))
	return t, nil
}

func (r *TaskRepo) UpdateTaskWithAuditLog(ctx context.Context, t entity.Task, auditLog entity.TaskAuditLog) error {
	const op = "postgres.UpdateTaskWithAuditLog"

	ctx, span := trace.Tracer().Start(ctx, op)
	defer span.End()

	span.SetAttributes(
		attribute.Int64("task.id", int64(t.ID)),
		attribute.Int("task.version", int(t.Version)),
	)

	tx, err := r.db.Pool.Begin(ctx)
	if err != nil {
		trace.RecordError(span, err)
		return richerror.New(op).WithErr(err)
	}
	defer tx.Rollback(ctx)

	query := `
		UPDATE tasks
		SET title = $1,
		    description = $2,
		    status = $3,
		    assignee = $4,
		    version = version + 1,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = $5 AND version = $6 AND deleted_at IS NULL
	`

	expectedDBVersion := t.Version - 1
	cmdTag, err := tx.Exec(ctx, query,
		t.Title, t.Description, t.Status, t.Assignee,
		t.ID, expectedDBVersion,
	)

	if err != nil {
		trace.RecordError(span, err)
		return richerror.New(op).WithErr(err)
	}

	if cmdTag.RowsAffected() == 0 {
		err := richerror.New(op).
			WithKind(richerror.KindConflict).
			WithMessage("task not found or version conflict during update")
		trace.RecordError(span, err)
		return err
	}

	prevJSON, _ := json.Marshal(auditLog.PreviousState)
	newJSON, _ := json.Marshal(auditLog.NewState)

	auditQuery := `
		INSERT INTO task_audit_logs (task_id, action, previous_state, new_state)
		VALUES ($1, $2, $3, $4)
	`
	_, err = tx.Exec(ctx, auditQuery, t.ID, auditLog.Action, prevJSON, newJSON)
	if err != nil {
		trace.RecordError(span, err)
		return richerror.New(op).WithErr(err)
	}

	if err := tx.Commit(ctx); err != nil {
		trace.RecordError(span, err)
		return richerror.New(op).WithErr(err)
	}

	return nil
}

func (r *TaskRepo) GetAuditLogsByTaskID(ctx context.Context, taskID entity.ID, page, size int) ([]entity.TaskAuditLog, int64, error) {
	const op = "postgres.GetAuditLogsByTaskID"

	ctx, span := trace.Tracer().Start(ctx, op)
	defer span.End()

	span.SetAttributes(
		attribute.Int64("audit.task_id", int64(taskID)),
		attribute.Int("page.number", page),
		attribute.Int("page.size", size),
	)

	var total int64
	countQuery := `SELECT COUNT(*) FROM task_audit_logs WHERE task_id = $1`
	if err := r.db.Pool.QueryRow(ctx, countQuery, taskID).Scan(&total); err != nil {
		trace.RecordError(span, err)
		return nil, 0, richerror.New(op).WithErr(err)
	}

	if total == 0 {
		return []entity.TaskAuditLog{}, 0, nil
	}

	offset := (page - 1) * size
	query := `
		SELECT id, task_id, action, previous_state, new_state, created_at
		FROM task_audit_logs
		WHERE task_id = $1
		ORDER BY created_at DESC
		LIMIT $2 OFFSET $3
	`

	rows, err := r.db.Pool.Query(ctx, query, taskID, size, offset)
	if err != nil {
		trace.RecordError(span, err)
		return nil, 0, richerror.New(op).WithErr(err)
	}
	defer rows.Close()

	var logs []entity.TaskAuditLog
	for rows.Next() {
		var l entity.TaskAuditLog
		var prevJSON, newJSON []byte
		err := rows.Scan(&l.ID, &l.TaskID, &l.Action, &prevJSON, &newJSON, &l.CreatedAt)
		if err != nil {
			trace.RecordError(span, err)
			return nil, 0, richerror.New(op).WithErr(err)
		}
		_ = json.Unmarshal(prevJSON, &l.PreviousState)
		_ = json.Unmarshal(newJSON, &l.NewState)
		logs = append(logs, l)
	}

	if err = rows.Err(); err != nil {
		trace.RecordError(span, err)
		return nil, 0, richerror.New(op).WithErr(err)
	}

	span.SetAttributes(attribute.Int("result.count", len(logs)))
	return logs, total, nil
}
