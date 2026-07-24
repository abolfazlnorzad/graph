package postgres

import (
	"context"
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

func (r *TaskRepo) CreateTask(ctx context.Context, t entity.Task) (entity.Task, error) {
	const op = "postgres.CreateTask"

	ctx, span := trace.Tracer().Start(ctx, op)
	defer span.End()

	span.SetAttributes(
		attribute.String("task.title", t.Title),
		attribute.String("task.status", string(t.Status)),
	)

	query := `
		INSERT INTO tasks (title, description, status, assignee, version)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at, updated_at
	`

	err := r.db.Pool.QueryRow(ctx, query,
		t.Title,
		t.Description,
		t.Status,
		t.Assignee,
		t.Version,
	).Scan(&t.ID, &t.CreatedAt, &t.UpdatedAt)

	if err != nil {
		trace.RecordError(span, err)
		return entity.Task{}, richerror.New(op).
			WithErr(err).
			WithKind(richerror.KindUnexpected).
			WithUserMsgKey(msg.ErrUnexpected)
	}

	span.SetAttributes(attribute.Int64("task.id", int64(t.ID)))
	return t, nil
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

func (r *TaskRepo) UpdateTask(ctx context.Context, t entity.Task) error {
	const op = "postgres.UpdateTask"

	ctx, span := trace.Tracer().Start(ctx, op)
	defer span.End()

	span.SetAttributes(
		attribute.Int64("task.id", int64(t.ID)),
		attribute.Int("task.version", int(t.Version)),
	)

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

	cmdTag, err := r.db.Pool.Exec(ctx, query,
		t.Title,
		t.Description,
		t.Status,
		t.Assignee,
		t.ID,
		expectedDBVersion,
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

	return nil
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

	var tasks []entity.Task
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
