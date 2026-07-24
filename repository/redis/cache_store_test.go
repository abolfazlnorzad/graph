package redis_test

import (
	"context"
	"testing"
	"time"

	redisadapter "github.com/abolfazlnorzad/graph/repository/redis"
	"github.com/abolfazlnorzad/graph/service"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

func setupRedisTest(t *testing.T) *redisadapter.Adapter {
	t.Helper()
	ctx := context.Background()

	redisContainer, err := tcredis.RunContainer(ctx,
		testcontainers.WithImage("redis:7-alpine"),
	)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = redisContainer.Terminate(ctx)
	})

	endpoint, err := redisContainer.ConnectionString(ctx)
	require.NoError(t, err)

	opts, err := goredis.ParseURL(endpoint)
	require.NoError(t, err)

	client := goredis.NewClient(opts)

	t.Cleanup(func() {
		client.FlushDB(ctx)
		client.Close()
	})

	return redisadapter.NewAdapter(client)
}

type testData struct {
	Name  string `json:"name"`
	Value int    `json:"value"`
}

func TestAdapter_SetAndGet(t *testing.T) {
	a := setupRedisTest(t)
	ctx := context.Background()

	original := testData{Name: "test", Value: 42}
	err := a.Set(ctx, "test:key", original, time.Minute)
	require.NoError(t, err)

	var result testData
	err = a.Get(ctx, "test:key", &result)
	require.NoError(t, err)
	assert.Equal(t, original, result)
}

func TestAdapter_Get_NotFound(t *testing.T) {
	a := setupRedisTest(t)
	ctx := context.Background()

	var dest testData
	err := a.Get(ctx, "nonexistent:key", &dest)
	assert.Error(t, err)
}

func TestAdapter_Delete(t *testing.T) {
	a := setupRedisTest(t)
	ctx := context.Background()

	err := a.Set(ctx, "delete:key", "value", time.Minute)
	require.NoError(t, err)

	err = a.Delete(ctx, "delete:key")
	require.NoError(t, err)

	var dest string
	err = a.Get(ctx, "delete:key", &dest)
	assert.Error(t, err)
}

func TestAdapter_DeleteByPrefix(t *testing.T) {
	a := setupRedisTest(t)
	ctx := context.Background()

	_ = a.Set(ctx, "prefix:1", "a", time.Minute)
	_ = a.Set(ctx, "prefix:2", "b", time.Minute)
	_ = a.Set(ctx, "prefix:3", "c", time.Minute)
	_ = a.Set(ctx, "other:1", "x", time.Minute)

	err := a.DeleteByPrefix(ctx, "prefix:")
	require.NoError(t, err)

	var dest string
	assert.Error(t, a.Get(ctx, "prefix:1", &dest))
	assert.Error(t, a.Get(ctx, "prefix:2", &dest))
	assert.Error(t, a.Get(ctx, "prefix:3", &dest))

	assert.NoError(t, a.Get(ctx, "other:1", &dest))
}

func TestAdapter_GetTTL(t *testing.T) {
	a := setupRedisTest(t)
	ctx := context.Background()

	err := a.Set(ctx, "ttl:key", "value", 10*time.Second)
	require.NoError(t, err)

	ttl, exists, err := a.GetTTL(ctx, "ttl:key")
	require.NoError(t, err)
	assert.True(t, exists)
	assert.True(t, ttl > 0 && ttl <= 10*time.Second)

	ttl, exists, err = a.GetTTL(ctx, "ttl:nonexistent")
	require.NoError(t, err)
	assert.False(t, exists)
	assert.Equal(t, time.Duration(0), ttl)
}

func TestAdapter_EmptyKeyDelete(t *testing.T) {
	a := setupRedisTest(t)
	ctx := context.Background()

	err := a.Delete(ctx)
	assert.NoError(t, err)
}

func TestAdapter_NilAdapter(t *testing.T) {
	var a *redisadapter.Adapter
	err := a.Close()
	assert.NoError(t, err)
}

var _ service.CacheStore = (*redisadapter.Adapter)(nil)
