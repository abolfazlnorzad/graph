package taskhandler

import (
	"net/http"
	"strconv"

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

func (h Handler) ListTasks(c *gin.Context) {
	var req param.ListTasksRequest

	pageNumber, _ := strconv.Atoi(c.DefaultQuery("page_number", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "25"))

	req.Pagination = param.PaginationRequest{
		PageNumber: pageNumber,
		PageSize:   pageSize,
	}

	if statusStr := c.Query("status"); statusStr != "" {
		status := entity.TaskStatus(statusStr)
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

	c.JSON(http.StatusOK, Envelope{
		Status: "success",
		Data: map[string]any{
			"tasks":      resp.Tasks,
			"pagination": resp.Pagination,
		},
	})
}

type Envelope struct {
	Status string `json:"status"`
	Data   any    `json:"data,omitempty"`
}
