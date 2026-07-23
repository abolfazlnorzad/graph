package entity

import (
	"time"

	"github.com/abolfazlnorzad/graph/pkg/types"
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
	ID          types.ID
	Title       string
	Description *string
	Status      TaskStatus
	Assignee    *string
	Version     int16

	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
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
	ID            types.ID
	TaskID        types.ID
	Action        TaskAction
	PreviousState map[string]any
	NewState      map[string]any

	CreatedAt time.Time
}
