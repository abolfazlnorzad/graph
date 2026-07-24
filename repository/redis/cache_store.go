package redis

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/abolfazlnorzad/graph/pkg/msg"
	"github.com/abolfazlnorzad/graph/pkg/richerror"
	"github.com/redis/go-redis/v9"
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

	data, err := json.Marshal(value)
	if err != nil {
		return richerror.New(OP).
			WithErr(err).
			WithKind(richerror.KindUnexpected).
			WithMessage("failed to marshal value")
	}

	if err := a.client.Set(ctx, key, data, expiration).Err(); err != nil {
		return richerror.New(OP).
			WithErr(err).
			WithKind(richerror.KindUnexpected).
			WithUserMsgKey(msg.ErrUnexpected)
	}

	return nil
}

func (a *Adapter) Get(ctx context.Context, key string, dest any) error {
	const OP = "redisadapter.Get"

	res, err := a.client.Get(ctx, key).Result()
	if err != nil {
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

	err = json.Unmarshal([]byte(res), dest)
	if err != nil {
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

func (a *Adapter) DeleteByPrefix(ctx context.Context, prefix string) error {
	var cursor uint64
	var err error
	var keys []string

	for {
		keys, cursor, err = a.client.Scan(ctx, cursor, prefix+"*", 100).Result()
		if err != nil {
			return err
		}

		if len(keys) > 0 {
			if err := a.client.Unlink(ctx, keys...).Err(); err != nil {
				return err
			}
		}

		if cursor == 0 {
			break
		}
	}

	return nil
}
