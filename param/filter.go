package param

import "github.com/abolfazlnorzad/graph/entity"

type TaskFilter struct {
	Status   *entity.TaskStatus `json:"status" form:"status" query:"status"`
	Assignee *string            `json:"assignee" form:"assignee" query:"assignee"`
}
