package taskhandler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/abolfazlnorzad/graph/delivery/httpserver/middleware"
	"github.com/abolfazlnorzad/graph/entity"
	"github.com/abolfazlnorzad/graph/param"
	"github.com/abolfazlnorzad/graph/service"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	svc service.Service
}

func NewHandler(svc service.Service) Handler {
	return Handler{svc: svc}
}

func (h Handler) SetRoutes(g *gin.Engine) {
	g.POST("tasks", h.CreateTask)
	g.GET("tasks/:id", h.GetTask)
	g.PATCH("tasks/:id", h.UpdateTask)
	g.DELETE("tasks/:id", h.DeleteTask)
	g.GET("tasks", h.ListTasks)
}

// CreateTask godoc
// @Summary      Create a new task
// @Description  Create a task with title, status, and optional description/assignee
// @Tags         tasks
// @Accept       json
// @Produce      json
// @Param        request  body      param.CreateTaskRequest  true  "Task to create"
// @Success      201      {object}  middleware.Envelope{data=param.TaskResponse}
// @Failure      400      {object}  middleware.Envelope
// @Failure      500      {object}  middleware.Envelope
// @Router       /tasks [post]
func (h Handler) CreateTask(c *gin.Context) {
	var req param.CreateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, err)
		return
	}

	resp, err := h.svc.CreateTask(c.Request.Context(), req)
	if err != nil {
		middleware.ErrorResponse(c, err)
		return
	}

	middleware.JSONResponse(c, http.StatusCreated, resp.Task)
}

// GetTask godoc
// @Summary      Get a task by ID
// @Description  Retrieve a single task by its ID
// @Tags         tasks
// @Produce      json
// @Param        id   path      int  true  "Task ID"
// @Success      200  {object}  middleware.Envelope{data=param.TaskResponse}
// @Failure      404  {object}  middleware.Envelope
// @Failure      500  {object}  middleware.Envelope
// @Router       /tasks/{id} [get]
func (h Handler) GetTask(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		middleware.ErrorResponse(c, err)
		return
	}

	resp, err := h.svc.GetTask(c.Request.Context(), param.GetTaskByIDRequest{ID: entity.ID(id)})
	if err != nil {
		middleware.ErrorResponse(c, err)
		return
	}

	middleware.JSONResponse(c, http.StatusOK, resp.Task)
}

// UpdateTask godoc
// @Summary      Update a task
// @Description  Update task fields (title, description, status, assignee) with optimistic locking
// @Tags         tasks
// @Accept       json
// @Produce      json
// @Param        id       path      int                      true  "Task ID"
// @Param        request  body      param.UpdateTaskRequest  true  "Fields to update"
// @Success      200      {object}  middleware.Envelope{data=param.TaskResponse}
// @Failure      400      {object}  middleware.Envelope
// @Failure      404      {object}  middleware.Envelope
// @Failure      409      {object}  middleware.Envelope
// @Failure      500      {object}  middleware.Envelope
// @Router       /tasks/{id} [patch]
func (h Handler) UpdateTask(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		middleware.ErrorResponse(c, err)
		return
	}

	var req param.UpdateTaskRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.ErrorResponse(c, err)
		return
	}
	req.ID = entity.ID(id)

	resp, err := h.svc.UpdateTask(c.Request.Context(), req)
	if err != nil {
		middleware.ErrorResponse(c, err)
		return
	}

	middleware.JSONResponse(c, http.StatusOK, resp.Task)
}

// DeleteTask godoc
// @Summary      Delete a task
// @Description  Soft delete a task by ID
// @Tags         tasks
// @Produce      json
// @Param        id   path      int  true  "Task ID"
// @Success      204  "No Content"
// @Failure      404  {object}  middleware.Envelope
// @Failure      500  {object}  middleware.Envelope
// @Router       /tasks/{id} [delete]
func (h Handler) DeleteTask(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		middleware.ErrorResponse(c, err)
		return
	}

	_, err = h.svc.DeleteTask(c.Request.Context(), param.DeleteTaskRequest{ID: entity.ID(id)})
	if err != nil {
		middleware.ErrorResponse(c, err)
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// ListTasks godoc
// @Summary      List tasks
// @Description  Get paginated list of tasks with optional status/assignee filters
// @Tags         tasks
// @Produce      json
// @Param        page_number  query     int     false  "Page number"       default(1)
// @Param        page_size    query     int     false  "Page size"         default(25)
// @Param        status       query     string  false  "Filter by status"  Enums(TODO, IN_PROGRESS, REVIEW, DONE, REJECTED, BLOCKED, CANCELED)
// @Param        assignee     query     string  false  "Filter by assignee"
// @Success      200          {object}  middleware.Envelope{data=object{tasks=[]param.TaskResponse, pagination=param.PaginationResponse}}
// @Failure      500          {object}  middleware.Envelope
// @Router       /tasks [get]
func (h Handler) ListTasks(c *gin.Context) {
	var req param.ListTasksRequest

	pageNumber, _ := strconv.Atoi(c.DefaultQuery("page_number", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "25"))

	req.Pagination = param.PaginationRequest{
		PageNumber: pageNumber,
		PageSize:   pageSize,
	}

	if statusStr := c.Query("status"); statusStr != "" {
		status := entity.TaskStatus(strings.ToUpper(statusStr))
		req.Filter.Status = &status
	}
	if assignee := c.Query("assignee"); assignee != "" {
		req.Filter.Assignee = &assignee
	}

	resp, err := h.svc.ListTask(c.Request.Context(), req)
	if err != nil {
		middleware.ErrorResponse(c, err)
		return
	}

	middleware.JSONResponse(c, http.StatusOK, map[string]any{
		"tasks":      resp.Tasks,
		"pagination": resp.Pagination,
	})
}

type Envelope struct {
	Status string `json:"status"`
	Data   any    `json:"data,omitempty"`
}
