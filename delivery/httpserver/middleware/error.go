package middleware

import (
	"errors"

	"github.com/abolfazlnorzad/graph/pkg/mapper"
	"github.com/abolfazlnorzad/graph/pkg/richerror"
	"github.com/gin-gonic/gin"
)

type Envelope struct {
	Status  string         `json:"status"`
	Message string         `json:"message"`
	Code    string         `json:"code,omitempty"`
	Data    map[string]any `json:"data,omitempty"`
}

func ErrorHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()

		if len(c.Errors) == 0 {
			return
		}

		err := c.Errors.Last().Err

		code := mapper.MapToStatusCode(err)
		status := "error"
		msgKey := "error.unexpected"
		var meta map[string]any

		var re richerror.RichError
		if errors.As(err, &re) {
			msgKey = string(re.UserMsgKey())
			meta = re.Meta()

			if meta != nil {
				sensitiveKeys := []string{"file", "line", "op", "err", "error", "stack_trace", "trace_id"}
				for _, key := range sensitiveKeys {
					delete(meta, key)
				}
			}
		}

		c.JSON(code, Envelope{
			Status:  status,
			Message: msgKey,
			Code:    msgKey,
			Data:    meta,
		})
	}
}

func ErrorResponse(c *gin.Context, err error) {
	c.Error(err)
	c.Abort()
}

func JSONResponse(c *gin.Context, code int, data any) {
	c.JSON(code, Envelope{
		Status: "success",
		Data:   map[string]any{"result": data},
	})
}
