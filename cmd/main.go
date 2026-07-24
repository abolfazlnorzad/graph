// @title           Graph Task Management API
// @version         1.0
// @description     REST API for managing tasks (CRUD operations)
// @host            localhost:8080
// @BasePath        /
// @schemes         http
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/abolfazlnorzad/graph/adapter/appmetrics"
	"github.com/abolfazlnorzad/graph/delivery/httpserver"
	"github.com/abolfazlnorzad/graph/delivery/httpserver/taskhandler"
	"github.com/abolfazlnorzad/graph/pkg/migration"
	pgdb "github.com/abolfazlnorzad/graph/pkg/postgresdb"
	redispkg "github.com/abolfazlnorzad/graph/pkg/redis"
	_ "github.com/abolfazlnorzad/graph/repository/migrations"
	"github.com/abolfazlnorzad/graph/repository/postgres"
	redisrepo "github.com/abolfazlnorzad/graph/repository/redis"
	"github.com/abolfazlnorzad/graph/service"
	"github.com/abolfazlnorzad/graph/validation"

	"github.com/abolfazlnorzad/graph/pkg/config"
	"github.com/abolfazlnorzad/graph/pkg/logger"
	"github.com/abolfazlnorzad/graph/pkg/metric"
	"github.com/abolfazlnorzad/graph/pkg/trace"

	_ "github.com/abolfazlnorzad/graph/docs"
)

func main() {
	ctx := context.Background()

	// 1. Load config
	var cfg config.Config
	if err := config.Load(config.Option{
		YamlFilePath: "config.yml",
		Prefix:       "GRAPH",
		Delimiter:    ".",
		Separator:    "__",
	}, &cfg); err != nil {
		slog.Error("failed to load config", slog.Any("error", err))
		os.Exit(1)
	}

	// 2. Init logger
	if err := logger.Init(cfg.Logger); err != nil {
		slog.Error("failed to init logger", slog.Any("error", err))
		os.Exit(1)
	}
	defer logger.Close()
	log := logger.L()

	// 3. Init trace
	if err := trace.Init(cfg.Trace); err != nil {
		log.Error("failed to init trace", slog.Any("error", err))
		os.Exit(1)
	}
	defer trace.Close()

	// 4. Init metric
	meterMgr, err := metric.New(cfg.Metric)
	if err != nil {
		log.Error("failed to init metric", slog.Any("error", err))
		os.Exit(1)
	}
	defer meterMgr.Close(ctx)

	// 5. Connect PostgreSQL
	db, err := pgdb.Connect(cfg.Postgres)
	if err != nil {
		log.Error("failed to connect to postgres", slog.Any("error", err))
		os.Exit(1)
	}
	defer db.Close()

	// 6. Run migrations
	migrator, err := migration.NewFromEmbed(cfg.Postgres)
	if err != nil {
		log.Error("failed to create migrator", slog.Any("error", err))
		os.Exit(1)
	}
	if err := migrator.Up(); err != nil {
		log.Error("failed to run migrations", slog.Any("error", err))
		os.Exit(1)
	}

	// 7. Connect Redis
	redisClient, err := redispkg.New(ctx, cfg.Redis)
	if err != nil {
		log.Error("failed to connect to redis", slog.Any("error", err))
		os.Exit(1)
	}
	defer redisClient.Close()

	// 8. Init repositories
	taskRepo := postgres.NewTaskRepo(db, log)
	cacheStore := redisrepo.NewAdapter(redisClient)

	// 9. Init metrics adapter
	appMtr, err := appmetrics.NewAppMetrics(meterMgr.Meter())
	if err != nil {
		log.Error("failed to init app metrics", slog.Any("error", err))
		os.Exit(1)
	}

	// 10. Init service
	vld := validation.NewValidator()
	svc := service.NewService(taskRepo, cacheStore, log, appMtr, vld)

	// 11. Init handler & server
	handler := taskhandler.NewHandler(svc)
	srv := httpserver.New(cfg, log, handler)

	// 12. Start server in a background goroutine
	go func() {
		srv.Serve()
	}()

	// 13. Graceful Shutdown Mechanism
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	log.Info("received shutdown signal, initiating graceful shutdown...")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("server forced to shutdown", slog.Any("error", err))
	}

	log.Info("server exited gracefully. cleaning up resources...")
}
