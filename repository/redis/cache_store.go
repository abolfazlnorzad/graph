package redis

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/abolfazlnorzad/graph/pkg/msg"
	"github.com/abolfazlnorzad/graph/pkg/richerror"
	"github.com/abolfazlnorzad/graph/pkg/trace"
	"github.com/redis/go-redis/v9"
	"go.opentelemetry.io/otel/attribute"
)

type Adapter struct {
	client *redis.Client
}

func NewAdapter(client *redis.Client) *Adapter {
	return &Adapter{client: client}
}

func (a *Adapter) Close() error {
	if a == nil || a.client == nil {
		return nil
	}

	return a.client.Close()
}

func (a *Adapter) Set(ctx context.Context, key string, value any, expiration time.Duration) error {
	const OP = "redisadapter.Set"

	ctx, span := trace.Tracer().Start(ctx, OP)
	defer span.End()

	span.SetAttributes(
		attribute.String("cache.key", key),
		attribute.String("cache.operation", "set"),
	)

	data, err := json.Marshal(value)
	if err != nil {
		trace.RecordError(span, err)
		return richerror.New(OP).
			WithErr(err).
			WithKind(richerror.KindUnexpected).
			WithMessage("failed to marshal value")
	}

	if err := a.client.Set(ctx, key, data, expiration).Err(); err != nil {
		trace.RecordError(span, err)
		return richerror.New(OP).
			WithErr(err).
			WithKind(richerror.KindUnexpected).
			WithUserMsgKey(msg.ErrUnexpected)
	}

	return nil
}

func (a *Adapter) Get(ctx context.Context, key string, dest any) error {
	const OP = "redisadapter.Get"

	ctx, span := trace.Tracer().Start(ctx, OP)
	defer span.End()

	span.SetAttributes(
		attribute.String("cache.key", key),
		attribute.String("cache.operation", "get"),
	)

	res, err := a.client.Get(ctx, key).Result()
	if err != nil {
		trace.RecordError(span, err)
		if errors.Is(err, redis.Nil) {
			return richerror.New(OP).
				WithKind(richerror.KindNotFound).
				WithUserMsgKey(msg.ErrNotFound).
				WithErr(err)
		}

		return richerror.New(OP).
			WithErr(err).
			WithKind(richerror.KindUnexpected).
			WithUserMsgKey(msg.ErrUnexpected)
	}

	span.SetAttributes(attribute.Bool("cache.hit", true))

	err = json.Unmarshal([]byte(res), dest)
	if err != nil {
		trace.RecordError(span, err)
		return richerror.New(OP).
			WithErr(err).
			WithKind(richerror.KindUnexpected).
			WithMessage("failed to unmarshal cache data")
	}

	return nil
}

func (a *Adapter) Delete(ctx context.Context, keys ...string) error {
	const OP = "redisadapter.Delete"

	if len(keys) == 0 {
		return nil
	}

	ctx, span := trace.Tracer().Start(ctx, OP)
	defer span.End()

	span.SetAttributes(
		attribute.StringSlice("cache.keys", keys),
		attribute.String("cache.operation", "delete"),
	)

	if err := a.client.Del(ctx, keys...).Err(); err != nil {
		trace.RecordError(span, err)
		return richerror.New(OP).
			WithErr(err).
			WithKind(richerror.KindUnexpected).
			WithUserMsgKey(msg.ErrUnexpected)
	}

	return nil
}

func (a *Adapter) GetTTL(ctx context.Context, key string) (time.Duration, bool, error) {
	const OP = "redisadapter.GetTTL"

	ctx, span := trace.Tracer().Start(ctx, OP)
	defer span.End()

	span.SetAttributes(
		attribute.String("cache.key", key),
		attribute.String("cache.operation", "get_ttl"),
	)

	ttl, err := a.client.TTL(ctx, key).Result()
	if err != nil {
		trace.RecordError(span, err)
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

func (a *Adapter) DeleteByPrefix(ctx context.Context, prefix string) error {
	const OP = "redisadapter.DeleteByPrefix"

	ctx, span := trace.Tracer().Start(ctx, OP)
	defer span.End()

	span.SetAttributes(
		attribute.String("cache.prefix", prefix),
		attribute.String("cache.operation", "delete_by_prefix"),
	)

	var cursor uint64
	var err error
	var keys []string

	for {
		keys, cursor, err = a.client.Scan(ctx, cursor, prefix+"*", 100).Result()
		if err != nil {
			trace.RecordError(span, err)
			return err
		}

		if len(keys) > 0 {
			if err := a.client.Unlink(ctx, keys...).Err(); err != nil {
				trace.RecordError(span, err)
				return err
			}
		}

		if cursor == 0 {
			break
		}
	}

	span.SetAttributes(attribute.Int("cache.deleted_count", len(keys)))
	return nil
}
