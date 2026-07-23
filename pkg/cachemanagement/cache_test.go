package cachemanagement_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/abolfazlnorzad/graph/pkg/cachemanagement"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockRedisClient struct {
	store  map[string]string
	ttlMap map[string]time.Duration
	setErr error
	getErr error
	delErr error
	ttlErr error
}

func newMockRedisClient() *mockRedisClient {
	return &mockRedisClient{
		store:  make(map[string]string),
		ttlMap: make(map[string]time.Duration),
	}
}

func (m *mockRedisClient) Set(_ context.Context, key string, value any, expiration time.Duration) *redis.StatusCmd {
	if m.setErr != nil {
		return redis.NewStatusResult("", m.setErr)
	}
	m.store[key] = fmt.Sprintf("%v", value)
	m.ttlMap[key] = expiration
	return redis.NewStatusResult("OK", nil)
}

func (m *mockRedisClient) Get(_ context.Context, key string) *redis.StringCmd {
	if m.getErr != nil {
		return redis.NewStringResult("", m.getErr)
	}
	val, ok := m.store[key]
	if !ok {
		return redis.NewStringResult("", redis.Nil)
	}
	return redis.NewStringResult(val, nil)
}

func (m *mockRedisClient) Del(_ context.Context, keys ...string) *redis.IntCmd {
	if m.delErr != nil {
		return redis.NewIntResult(0, m.delErr)
	}
	for _, k := range keys {
		delete(m.store, k)
		delete(m.ttlMap, k)
	}
	return redis.NewIntResult(int64(len(keys)), nil)
}

func (m *mockRedisClient) TTL(_ context.Context, key string) *redis.DurationCmd {
	if m.ttlErr != nil {
		return redis.NewDurationResult(0, m.ttlErr)
	}
	_, exists := m.store[key]
	if !exists {
		return redis.NewDurationResult(-2*time.Second, nil)
	}
	t, ok := m.ttlMap[key]
	if !ok {
		return redis.NewDurationResult(-1*time.Second, nil)
	}
	return redis.NewDurationResult(t, nil)
}

func TestSet_Success(t *testing.T) {
	mock := newMockRedisClient()
	cm := cachemanagement.NewCacheManagerWithClient(mock)

	err := cm.Set(context.Background(), "key1", "value1", 10*time.Second)
	require.NoError(t, err)
	assert.Equal(t, "value1", mock.store["key1"])
}

func TestSet_Error(t *testing.T) {
	mock := newMockRedisClient()
	mock.setErr = fmt.Errorf("connection lost")
	cm := cachemanagement.NewCacheManagerWithClient(mock)

	err := cm.Set(context.Background(), "key1", "value1", 10*time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection lost")
}

func TestGet_Success(t *testing.T) {
	mock := newMockRedisClient()
	mock.store["key1"] = "hello"
	cm := cachemanagement.NewCacheManagerWithClient(mock)

	val, err := cm.Get(context.Background(), "key1")
	require.NoError(t, err)
	assert.Equal(t, "hello", val)
}

func TestGet_NotFound(t *testing.T) {
	mock := newMockRedisClient()
	cm := cachemanagement.NewCacheManagerWithClient(mock)

	_, err := cm.Get(context.Background(), "nonexistent")
	assert.ErrorIs(t, err, redis.Nil)
}

func TestGet_Error(t *testing.T) {
	mock := newMockRedisClient()
	mock.getErr = fmt.Errorf("redis error")
	cm := cachemanagement.NewCacheManagerWithClient(mock)

	_, err := cm.Get(context.Background(), "key1")
	assert.Error(t, err)
}

func TestDelete_Single(t *testing.T) {
	mock := newMockRedisClient()
	mock.store["k1"] = "v1"
	cm := cachemanagement.NewCacheManagerWithClient(mock)

	err := cm.Delete(context.Background(), "k1")
	require.NoError(t, err)
	assert.Empty(t, mock.store)
}

func TestDelete_Multiple(t *testing.T) {
	mock := newMockRedisClient()
	mock.store["k1"] = "v1"
	mock.store["k2"] = "v2"
	mock.store["k3"] = "v3"
	cm := cachemanagement.NewCacheManagerWithClient(mock)

	err := cm.Delete(context.Background(), "k1", "k2")
	require.NoError(t, err)
	assert.Len(t, mock.store, 1)
	assert.Equal(t, "v3", mock.store["k3"])
}

func TestDelete_Error(t *testing.T) {
	mock := newMockRedisClient()
	mock.delErr = fmt.Errorf("del failed")
	cm := cachemanagement.NewCacheManagerWithClient(mock)

	err := cm.Delete(context.Background(), "key1")
	assert.Error(t, err)
}

func TestGetTTL_ExistsWithExpiry(t *testing.T) {
	mock := newMockRedisClient()
	mock.store["key1"] = "val"
	mock.ttlMap["key1"] = 30 * time.Second
	cm := cachemanagement.NewCacheManagerWithClient(mock)

	ttl, exists, err := cm.GetTTL(context.Background(), "key1")
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, int64(30), ttl)
}

func TestGetTTL_ExistsNoExpiry(t *testing.T) {
	mock := newMockRedisClient()
	mock.store["key1"] = "val"
	cm := cachemanagement.NewCacheManagerWithClient(mock)

	ttl, exists, err := cm.GetTTL(context.Background(), "key1")
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, int64(-1), ttl)
}

func TestGetTTL_NotExists(t *testing.T) {
	mock := newMockRedisClient()
	cm := cachemanagement.NewCacheManagerWithClient(mock)

	_, _, err := cm.GetTTL(context.Background(), "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "key does not exist")
}

func TestGetTTL_Error(t *testing.T) {
	mock := newMockRedisClient()
	mock.ttlErr = fmt.Errorf("ttl error")
	cm := cachemanagement.NewCacheManagerWithClient(mock)

	_, _, err := cm.GetTTL(context.Background(), "key1")
	assert.Error(t, err)
}

func TestSet_Overwrite(t *testing.T) {
	mock := newMockRedisClient()
	cm := cachemanagement.NewCacheManagerWithClient(mock)

	_ = cm.Set(context.Background(), "key1", "first", 10*time.Second)
	_ = cm.Set(context.Background(), "key1", "second", 20*time.Second)

	val, _ := cm.Get(context.Background(), "key1")
	assert.Equal(t, "second", val)
	assert.Equal(t, 20*time.Second, mock.ttlMap["key1"])
}
