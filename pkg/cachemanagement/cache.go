package cachemanagement

import (
	"context"
	"errors"
	"fmt"
	"time"

	redisadapter "github.com/abolfazlnorzad/graph/adapter/redis"
	"github.com/abolfazlnorzad/graph/pkg/msg"
	"github.com/abolfazlnorzad/graph/pkg/richerror"
	"github.com/redis/go-redis/v9"
)

type RedisClient interface {
	Set(ctx context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd
	Get(ctx context.Context, key string) *redis.StringCmd
	Del(ctx context.Context, keys ...string) *redis.IntCmd
	TTL(ctx context.Context, key string) *redis.DurationCmd
}

type CacheManager struct {
	client RedisClient
}

func NewCacheManager(cache *redisadapter.Adapter) *CacheManager {
	return &CacheManager{
		client: cache.Client(),
	}
}

func NewCacheManagerWithClient(client RedisClient) *CacheManager {
	return &CacheManager{
		client: client,
	}
}

func (c *CacheManager) Set(ctx context.Context, key string, value any, expire time.Duration) error {
	return c.client.Set(ctx, key, value, expire).Err()
}

func (c *CacheManager) Get(ctx context.Context, key string) (string, error) {
	const OP = "cachemanager.Get"
	res, err := c.client.Get(ctx, key).Result()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return "", richerror.New(OP).WithKind(richerror.KindNotFound).WithUserMsgKey(msg.ErrNotFound).WithErr(err)
		}

		return "", richerror.New(OP).WithErr(err).WithKind(richerror.KindUnexpected).WithUserMsgKey(msg.ErrUnexpected).WithErr(err)
	}
	return res, nil
}

func (c *CacheManager) Delete(ctx context.Context, keys ...string) error {
	return c.client.Del(ctx, keys...).Err()
}

func (c *CacheManager) GetTTL(ctx context.Context, key string) (int64, bool, error) {
	ttl, err := c.client.TTL(ctx, key).Result()
	if err != nil {
		return 0, false, err
	}

	if ttl == -2*time.Second {
		return 0, false, fmt.Errorf("key does not exist")
	}

	if ttl == -1*time.Second {
		return -1, true, nil
	}

	return int64(ttl.Seconds()), true, nil
}
