package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

func OTelMetricsMiddleware() gin.HandlerFunc {
	meter := otel.Meter("delivery.httpserver")

	requestsTotal, _ := meter.Int64Counter(
		"requests_total",
		metric.WithDescription("Total number of HTTP requests processed"),
	)

	requestLatency, _ := meter.Float64Histogram(
		"request_latency_histogram",
		metric.WithDescription("Latency of HTTP requests in seconds"),
		metric.WithUnit("s"),
		metric.WithExplicitBucketBoundaries(
			0.001, 0.002, 0.005, 0.01, 0.025, 0.05,
			0.1, 0.25, 0.5, 1, 2.5, 5, 10,
		),
	)

	return func(c *gin.Context) {
		start := time.Now()

		c.Next()

		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())
		method := c.Request.Method

		path := c.FullPath()
		if path == "" {
			path = "unknown"
		}

		attrs := metric.WithAttributes(
			attribute.String("method", method),
			attribute.String("path", path),
			attribute.String("status", status),
		)

		ctx := c.Request.Context()
		requestsTotal.Add(ctx, 1, attrs)
		requestLatency.Record(ctx, duration, attrs)
	}
}
