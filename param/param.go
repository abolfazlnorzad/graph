package param

import (
	"time"

	"github.com/abolfazlnorzad/graph/entity"
)

type AuditLogResponse struct {
	ID            entity.ID         `json:"id"`
	TaskID        entity.ID         `json:"task_id"`
	Action        entity.TaskAction `json:"action"`
	PreviousState map[string]any    `json:"previous_state"`
	NewState      map[string]any    `json:"new_state"`

	CreatedAt time.Time `json:"created_at"`
}

type TaskResponse struct {
	ID          entity.ID          `json:"id"`
	Title       string             `json:"title"`
	Description *string            `json:"description"`
	Status      entity.TaskStatus  `json:"status"`
	Assignee    *string            `json:"assignee"`
	Version     int16              `json:"version"`
	AuditLogs   []AuditLogResponse `json:"audit_logs,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type CreateTaskRequest struct {
	Title       string            `json:"title"`
	Description *string           `json:"description"`
	Status      entity.TaskStatus `json:"status"`
	Assignee    *string           `json:"assignee"`
	Version     int16             `json:"version"`
}

type CreateTaskResponse struct {
	Task TaskResponse `json:"task"`
}

type GetTaskByIDRequest struct {
	ID            entity.ID `json:"id"`
	AuditPage     int       `json:"audit_page"`
	AuditPageSize int       `json:"audit_page_size"`
}
type GetTaskByIDResponse struct {
	Task            TaskResponse        `json:"task"`
	AuditPagination *PaginationResponse `json:"audit_pagination,omitempty"`
}

type ListTasksRequest struct {
	Pagination PaginationRequest `json:"pagination"`
	Filter     TaskFilter        `json:"filter"`
}

type ListTasksResponse struct {
	Pagination PaginationResponse `json:"pagination"`
	Tasks      []TaskResponse     `json:"tasks"`
}

type UpdateTaskRequest struct {
	ID          entity.ID          `json:"id"`
	Title       *string            `json:"title"`
	Description *string            `json:"description"`
	Status      *entity.TaskStatus `json:"status"`
	Assignee    *string            `json:"assignee"`
	Version     int16              `json:"version"`
}

type UpdateTaskResponse struct {
	Task TaskResponse `json:"task"`
}

type DeleteTaskRequest struct {
	ID entity.ID `json:"id"`
}
type DeleteTaskResponse struct{}

type ChangeTaskStatusRequest struct {
	ID      entity.ID         `json:"id"`
	Status  entity.TaskStatus `json:"status"`
	Version int16             `json:"version"`
}

type ChangeTaskStatusResponse struct {
	Task TaskResponse `json:"task"`
}
