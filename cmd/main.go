package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/abolfazlnorzad/graph/delivery/httpserver"
	"github.com/abolfazlnorzad/graph/delivery/httpserver/taskhandler"
	pgdb "github.com/abolfazlnorzad/graph/pkg/postgresdb"
	redispkg "github.com/abolfazlnorzad/graph/pkg/redis"
	"github.com/abolfazlnorzad/graph/repository/postgres"
	redisrepo "github.com/abolfazlnorzad/graph/repository/redis"
	"github.com/abolfazlnorzad/graph/service"
	"github.com/abolfazlnorzad/graph/validation"

	"github.com/abolfazlnorzad/graph/pkg/config"
	"github.com/abolfazlnorzad/graph/pkg/logger"
	"github.com/abolfazlnorzad/graph/pkg/metric"
	"github.com/abolfazlnorzad/graph/pkg/trace"
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

	// 6. Connect Redis
	redisClient, err := redispkg.New(ctx, cfg.Redis)
	if err != nil {
		log.Error("failed to connect to redis", slog.Any("error", err))
		os.Exit(1)
	}
	defer redisClient.Close()

	// 7. Init repositories
	taskRepo := postgres.NewTaskRepo(db, log)
	cacheStore := redisrepo.NewAdapter(redisClient)

	// 8. Init service
	vld := validation.NewValidator()
	svc := service.NewService(taskRepo, cacheStore, log, nil, vld)

	// 9. Init handler & server
	handler := taskhandler.NewHandler(svc)
	srv := httpserver.New(cfg, log, handler)

	// 10. Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-quit
		log.Info("shutting down...")
		srv.Shutdown(ctx)
	}()

	// 11. Start server
	srv.Serve()
}
