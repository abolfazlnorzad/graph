package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"go.opentelemetry.io/otel/trace"
	"gopkg.in/natefinch/lumberjack.v2"
)

var (
	globalLogger *slog.Logger
	globalWriter io.Closer
	once         sync.Once
)

type Config struct {
	Level            string `koanf:"level"`
	FilePath         string `koanf:"file_path"`
	UseLocalTime     bool   `koanf:"use_local_time"`
	FileMaxSizeInMB  int    `koanf:"file_max_size_in_mb"`
	FileMaxAgeInDays int    `koanf:"file_max_age_in_days"`
}

type tracingHandler struct {
	slog.Handler
}

func (h *tracingHandler) Handle(ctx context.Context, r slog.Record) error {
	spanContext := trace.SpanContextFromContext(ctx)
	if spanContext.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", spanContext.TraceID().String()),
			slog.String("span_id", spanContext.SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

func (h *tracingHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &tracingHandler{Handler: h.Handler.WithAttrs(attrs)}
}

func (h *tracingHandler) WithGroup(name string) slog.Handler {
	return &tracingHandler{Handler: h.Handler.WithGroup(name)}
}

func resolveLogPath(cfg Config) (string, error) {
	if cfg.FilePath != "" {
		if filepath.IsAbs(cfg.FilePath) {
			return "", fmt.Errorf("absolute paths are not allowed")
		}
		cleanPath := filepath.Clean(cfg.FilePath)
		if strings.HasPrefix(cleanPath, "..") {
			return "", fmt.Errorf("path traversal is not allowed")
		}
		workingDir, _ := os.Getwd()
		logPath := filepath.Join(workingDir, cleanPath)
		return logPath, nil
	}
	exePath, _ := os.Executable()
	return filepath.Join(filepath.Dir(exePath), "logs", "app.log"), nil
}

func Init(cfg Config) error {
	var initError error
	once.Do(func() {
		logPath, err := resolveLogPath(cfg)
		if err != nil {
			initError = err
			return
		}

		if err := os.MkdirAll(filepath.Dir(logPath), 0o750); err != nil {
			initError = fmt.Errorf("failed to create log dir: %w", err)
			return
		}

		fileWriter := &lumberjack.Logger{
			Filename:  logPath,
			LocalTime: cfg.UseLocalTime,
			MaxSize:   cfg.FileMaxSizeInMB,
			MaxAge:    cfg.FileMaxAgeInDays,
		}

		baseHandler := slog.NewJSONHandler(io.MultiWriter(fileWriter, os.Stdout), &slog.HandlerOptions{
			Level: mapLevel(cfg.Level),
		})

		globalLogger = slog.New(&tracingHandler{Handler: baseHandler})
		globalWriter = fileWriter
	})
	return initError
}

func L() *slog.Logger {
	if globalLogger == nil {
		panic("logger not initialized. Call logger.Init first")
	}
	return globalLogger
}

func Close() error {
	if globalWriter != nil {
		err := globalWriter.Close()
		globalWriter = nil
		return err
	}
	return nil
}

// New creates an independent logger instance with its own file writer.
func New(cfg Config) (*slog.Logger, io.Closer, error) {
	logPath, err := resolveLogPath(cfg)
	if err != nil {
		return nil, nil, err
	}

	if err := os.MkdirAll(filepath.Dir(logPath), 0o750); err != nil {
		return nil, nil, fmt.Errorf("failed to create log dir: %w", err)
	}

	fileWriter := &lumberjack.Logger{
		Filename:  logPath,
		LocalTime: cfg.UseLocalTime,
		MaxSize:   cfg.FileMaxSizeInMB,
		MaxAge:    cfg.FileMaxAgeInDays,
	}

	baseHandler := slog.NewJSONHandler(io.MultiWriter(fileWriter, os.Stdout), &slog.HandlerOptions{
		Level: mapLevel(cfg.Level),
	})

	l := slog.New(&tracingHandler{Handler: baseHandler})
	return l, fileWriter, nil
}

func mapLevel(levelStr string) slog.Level {
	switch levelStr {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}
