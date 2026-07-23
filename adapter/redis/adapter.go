package redisadapter

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/redis/go-redis/extra/redisotel/v9"
	"github.com/redis/go-redis/v9"
)

type Config struct {
	Host     string `koanf:"host"`
	Port     int    `koanf:"port"`
	Password string `koanf:"password"`
	DB       int    `koanf:"db"`
}

func (config Config) Validate() map[string]error {
	errs := map[string]error{}
	if config.Host == "" {
		errs["host"] = fmt.Errorf("redis host is empty")
	}
	if config.Port <= 0 || config.Port > 65535 {
		errs["port"] = fmt.Errorf("invalid redis port: %d", config.Port)
	}
	// TODO: may be need to add maxDb in config or just remove the upper bound
	if config.DB < 0 || config.DB > 15 {
		errs["db"] = fmt.Errorf("invalid redis DB: %d, must be between 0 and 15", config.DB)
	}
	return errs
}

func FormatValidationErrors(errs map[string]error) string {
	if len(errs) == 0 {
		return ""
	}

	errorStrings := make([]string, 0, len(errs))
	for field, err := range errs {
		errorStrings = append(errorStrings, fmt.Sprintf("%s: %v", field, err))
	}

	return fmt.Sprintf("validation errors: %s", strings.Join(errorStrings, "; "))
}

type Adapter struct {
	client *redis.Client
}

func New(ctx context.Context, config Config) (*Adapter, error) {
	if ctx == nil {
		return nil, fmt.Errorf("context cannot be nil")
	}

	if validationErrors := config.Validate(); len(validationErrors) > 0 {
		return nil, fmt.Errorf("invalid redis configuration: %s", FormatValidationErrors(validationErrors))
	}

	addr := fmt.Sprintf("%s:%d", config.Host, config.Port)

	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: config.Password,
		DB:       config.DB,
	})
	if err := redisotel.InstrumentTracing(rdb); err != nil {
		return nil, fmt.Errorf("failed to instrument redis with opentelemetry: %w", err)
	}
	if err := rdb.Ping(ctx).Err(); err != nil {
		slog.Error("failed to connect to redis", slog.String("addr", addr), slog.Int("db", config.DB), slog.Any("error", err))

		if cErr := rdb.Close(); cErr != nil {
			slog.Error("failed to close redis client after connection failure", slog.Any("error", cErr))
		}

		return nil, fmt.Errorf("redis connection failed: %w", err)
	}

	slog.Info("redis is up and running", slog.String("addr", addr), slog.Int("db", config.DB))

	return &Adapter{client: rdb}, nil
}

func (a *Adapter) Client() *redis.Client {
	return a.client
}

func (a *Adapter) Close() error {
	if a == nil || a.client == nil {
		return nil
	}

	return a.client.Close()
}
