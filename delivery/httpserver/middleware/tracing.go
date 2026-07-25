package middleware

import (
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

func OTelTracingMiddleware() gin.HandlerFunc {
	tracer := otel.Tracer("delivery.httpserver")

	return func(c *gin.Context) {
		spanName := c.Request.Method + " " + c.FullPath()
		if spanName == " " {
			spanName = c.Request.Method + " " + c.Request.URL.Path
		}

		ctx, span := tracer.Start(c.Request.Context(), spanName,
			trace.WithAttributes(
				attribute.String("http.method", c.Request.Method),
				attribute.String("http.url", c.Request.URL.String()),
				attribute.String("http.target", c.Request.URL.Path),
				attribute.String("http.host", c.Request.Host),
				attribute.String("http.scheme", func() string {
					if c.Request.TLS != nil {
						return "https"
					}
					return "http"
				}()),
				attribute.String("http.user_agent", c.Request.UserAgent()),
				attribute.String("net.peer.ip", c.ClientIP()),
			),
		)
		defer span.End()

		c.Request = c.Request.WithContext(ctx)
		c.Next()

		// Set response attributes after handler completes
		span.SetAttributes(
			attribute.Int("http.status_code", c.Writer.Status()),
			attribute.Int("http.response_content_length", c.Writer.Size()),
		)

		if c.Writer.Status() >= 400 {
			span.SetStatus(1, "") // 1 = Error status code
		}
	}
}
