package httpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/abolfazlnorzad/graph/delivery/httpserver/middleware"
	"github.com/abolfazlnorzad/graph/delivery/httpserver/taskhandler"
	"github.com/abolfazlnorzad/graph/pkg/config"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

type Server struct {
	config  config.Config
	logger  *slog.Logger
	Router  *gin.Engine
	handler taskhandler.Handler
	httpSrv *http.Server
}

func New(cfg config.Config, logger *slog.Logger, handler taskhandler.Handler) Server {
	return Server{
		config:  cfg,
		logger:  logger,
		handler: handler,
		Router:  gin.New(),
	}
}

func (s *Server) Serve() {
	s.Router.Use(middleware.ErrorHandler())

	s.Router.Use(middleware.OTelMetricsMiddleware())

	s.Router.Use(s.requestLogger())

	s.Router.Use(gin.Recovery())

	s.Router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	s.Router.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))
	s.handler.SetRoutes(s.Router)

	address := fmt.Sprintf(":%d", s.config.HTTPServer.Port)
	s.httpSrv = &http.Server{
		Addr:    address,
		Handler: s.Router,
	}

	s.logger.Info("starting http server", slog.String("address", address))

	if err := s.httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		s.logger.Error("error starting server", slog.Any("error", err))
	}
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.httpSrv != nil {
		return s.httpSrv.Shutdown(ctx)
	}
	return nil
}

func (s Server) requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery

		c.Next()

		latency := time.Since(start)
		status := c.Writer.Status()
		clientIP := c.ClientIP()
		method := c.Request.Method
		contentLength := c.Request.ContentLength
		responseSize := c.Writer.Size()
		protocol := c.Request.Proto

		errMsg := ""
		if len(c.Errors) > 0 {
			errMsg = c.Errors.String()
		}

		s.logger.Info("http request",
			slog.String("client_ip", clientIP),
			slog.String("method", method),
			slog.String("path", path),
			slog.String("query", query),
			slog.Int("status", status),
			slog.Duration("latency", latency),
			slog.Int64("content_length", contentLength),
			slog.Int("response_size", responseSize),
			slog.String("protocol", protocol),
			slog.String("error", errMsg),
		)
	}
}
