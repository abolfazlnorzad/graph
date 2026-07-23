package entity

import (
	"time"
)

type TaskStatus string

const (
	StatusTodo       TaskStatus = "TODO"
	StatusInProgress TaskStatus = "IN_PROGRESS"
	StatusReview     TaskStatus = "REVIEW"
	StatusDone       TaskStatus = "DONE"
	StatusRejected   TaskStatus = "REJECTED"
	StatusBlocked    TaskStatus = "BLOCKED"
	StatusCanceled   TaskStatus = "CANCELED"
)

type Task struct {
	ID          int64
	Title       string
	Description *string
	Status      TaskStatus
	Assignee    *string
	Version     int

	CreatedAt   time.Time
	UpdatedAt   time.Time
	DeletedAt   *time.Time
}

func (s TaskStatus) IsValid() bool {
	switch s {
	case StatusTodo, StatusInProgress, StatusReview, StatusDone, StatusRejected, StatusBlocked, StatusCanceled:
		return true
	}
	return false
}

type TaskAction string

const (
	ActionCreate TaskAction = "CREATE"
	ActionUpdate TaskAction = "UPDATE"
	ActionDelete TaskAction = "DELETE"
)

type TaskAuditLog struct {
	ID            int64
	TaskID        int64
	Action        TaskAction
	PreviousState map[string]any
	NewState      map[string]any

	CreatedAt time.Time
}
