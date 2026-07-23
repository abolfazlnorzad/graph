package redisadapter

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/abolfazlnorzad/graph/pkg/msg"
	"github.com/abolfazlnorzad/graph/pkg/richerror"
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

func (a *Adapter) Close() error {
	if a == nil || a.client == nil {
		return nil
	}

	return a.client.Close()
}

func (a *Adapter) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	const OP = "redisadapter.Set"

	if err := a.client.Set(ctx, key, value, expiration).Err(); err != nil {
		return richerror.New(OP).
			WithErr(err).
			WithKind(richerror.KindUnexpected).
			WithUserMsgKey(msg.ErrUnexpected)
	}

	return nil
}

func (a *Adapter) Get(ctx context.Context, key string) (string, error) {
	const OP = "redisadapter.Get"

	res, err := a.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", richerror.New(OP).
				WithKind(richerror.KindNotFound).
				WithUserMsgKey(msg.ErrNotFound).
				WithErr(err)
		}

		return "", richerror.New(OP).
			WithErr(err).
			WithKind(richerror.KindUnexpected).
			WithUserMsgKey(msg.ErrUnexpected)
	}

	return res, nil
}

func (a *Adapter) Delete(ctx context.Context, keys ...string) error {
	const OP = "redisadapter.Delete"

	if len(keys) == 0 {
		return nil
	}

	if err := a.client.Del(ctx, keys...).Err(); err != nil {
		return richerror.New(OP).
			WithErr(err).
			WithKind(richerror.KindUnexpected).
			WithUserMsgKey(msg.ErrUnexpected)
	}

	return nil
}

func (a *Adapter) GetTTL(ctx context.Context, key string) (time.Duration, bool, error) {
	const OP = "redisadapter.GetTTL"

	ttl, err := a.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, false, richerror.New(OP).
			WithErr(err).
			WithKind(richerror.KindUnexpected).
			WithUserMsgKey(msg.ErrUnexpected)
	}

	if ttl == -2*time.Nanosecond {
		return 0, false, nil
	}

	if ttl == -1*time.Nanosecond {
		return -1 * time.Second, true, nil
	}

	return ttl, true, nil
}
