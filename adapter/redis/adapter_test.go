package redisadapter_test

import (
	"context"
	"testing"
	"time"

	redisadapter "github.com/abolfazlnorzad/graph/adapter/redis"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfig_Validate_EmptyHost(t *testing.T) {
	cfg := redisadapter.Config{Port: 6379, DB: 0}
	errs := cfg.Validate()

	require.Contains(t, errs, "host")
	assert.Contains(t, errs["host"].Error(), "empty")
}

func TestConfig_Validate_InvalidPort_Zero(t *testing.T) {
	cfg := redisadapter.Config{Host: "localhost", Port: 0, DB: 0}
	errs := cfg.Validate()

	require.Contains(t, errs, "port")
}

func TestConfig_Validate_InvalidPort_Negative(t *testing.T) {
	cfg := redisadapter.Config{Host: "localhost", Port: -1, DB: 0}
	errs := cfg.Validate()

	require.Contains(t, errs, "port")
}

func TestConfig_Validate_InvalidPort_TooHigh(t *testing.T) {
	cfg := redisadapter.Config{Host: "localhost", Port: 70000, DB: 0}
	errs := cfg.Validate()

	require.Contains(t, errs, "port")
}

func TestConfig_Validate_InvalidDB_Negative(t *testing.T) {
	cfg := redisadapter.Config{Host: "localhost", Port: 6379, DB: -1}
	errs := cfg.Validate()

	require.Contains(t, errs, "db")
}

func TestConfig_Validate_InvalidDB_TooHigh(t *testing.T) {
	cfg := redisadapter.Config{Host: "localhost", Port: 6379, DB: 16}
	errs := cfg.Validate()

	require.Contains(t, errs, "db")
}

func TestConfig_Validate_Valid(t *testing.T) {
	cfg := redisadapter.Config{Host: "localhost", Port: 6379, DB: 0}
	errs := cfg.Validate()

	assert.Empty(t, errs)
}

func TestConfig_Validate_MultipleErrors(t *testing.T) {
	cfg := redisadapter.Config{Port: 99999, DB: 20}
	errs := cfg.Validate()

	assert.Contains(t, errs, "host")
	assert.Contains(t, errs, "port")
	assert.Contains(t, errs, "db")
	assert.Len(t, errs, 3)
}

func TestFormatValidationErrors_Empty(t *testing.T) {
	result := redisadapter.FormatValidationErrors(map[string]error{})
	assert.Empty(t, result)
}

func TestFormatValidationErrors_SingleError(t *testing.T) {
	errs := map[string]error{"host": assert.AnError}
	result := redisadapter.FormatValidationErrors(errs)

	assert.Contains(t, result, "validation errors:")
	assert.Contains(t, result, "host:")
}

func TestFormatValidationErrors_MultipleErrors(t *testing.T) {
	errs := map[string]error{
		"host": assert.AnError,
		"port": assert.AnError,
	}
	result := redisadapter.FormatValidationErrors(errs)

	assert.Contains(t, result, "validation errors:")
	assert.Contains(t, result, "host:")
	assert.Contains(t, result, "port:")
	assert.Contains(t, result, "; ")
}

func TestNew_NilContext(t *testing.T) {
	_, err := redisadapter.New(nil, redisadapter.Config{Host: "localhost", Port: 6379})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "context cannot be nil")
}

func TestNew_InvalidConfig(t *testing.T) {
	_, err := redisadapter.New(context.Background(), redisadapter.Config{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid redis configuration")
}

func TestNew_ConnectionFailure(t *testing.T) {
	cfg := redisadapter.Config{Host: "localhost", Port: 19999, DB: 0}
	_, err := redisadapter.New(context.Background(), cfg)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "redis connection failed")
}

func TestAdapter_Close_NilAdapter(t *testing.T) {
	var a *redisadapter.Adapter
	err := a.Close()
	assert.NoError(t, err)
}

func TestAdapter_Close_NilClient(t *testing.T) {
	a := &redisadapter.Adapter{}
	err := a.Close()
	assert.NoError(t, err)
}

func TestAdapter_Close_ConnectionFailure(t *testing.T) {
	cfg := redisadapter.Config{Host: "localhost", Port: 19999, DB: 0}
	a, err := redisadapter.New(context.Background(), cfg)
	assert.Nil(t, a)
	assert.Error(t, err)
}

func setupIntegrationTest(t *testing.T) *redisadapter.Adapter {
	cfg := redisadapter.Config{Host: "localhost", Port: 6379, DB: 15}
	a, err := redisadapter.New(context.Background(), cfg)
	if err != nil {
		t.Skipf("Skipping integration test: Redis is not available on %s:%d", cfg.Host, cfg.Port)
	}
	return a
}

func TestAdapter_SetAndGet(t *testing.T) {
	a := setupIntegrationTest(t)
	defer a.Close()
	ctx := context.Background()
	key := "test_set_get_key"

	_ = a.Delete(ctx, key)

	type testData struct {
		Value string `json:"value"`
	}

	original := testData{Value: "my_test_value"}
	err := a.Set(ctx, key, original, 1*time.Minute)
	require.NoError(t, err)

	var result testData
	err = a.Get(ctx, key, &result)
	require.NoError(t, err)
	assert.Equal(t, original, result)

	_ = a.Delete(ctx, key)
}

func TestAdapter_Get_NotFound(t *testing.T) {
	a := setupIntegrationTest(t)
	defer a.Close()
	ctx := context.Background()

	var dest string
	err := a.Get(ctx, "non_existent_random_key", &dest)
	require.Error(t, err)

	assert.Contains(t, err.Error(), "redisadapter.Get")
}

func TestAdapter_Delete(t *testing.T) {
	a := setupIntegrationTest(t)
	defer a.Close()
	ctx := context.Background()
	key := "test_delete_key"

	err := a.Set(ctx, key, "value_to_delete", 1*time.Minute)
	require.NoError(t, err)

	err = a.Delete(ctx, key)
	require.NoError(t, err)

	var dest string
	err = a.Get(ctx, key, &dest)
	require.Error(t, err)
}

func TestAdapter_GetTTL(t *testing.T) {
	a := setupIntegrationTest(t)
	defer a.Close()
	ctx := context.Background()

	ttl, exists, err := a.GetTTL(ctx, "ttl_not_exist_key")
	require.NoError(t, err)
	assert.False(t, exists)
	assert.Equal(t, time.Duration(0), ttl)

	keyExp := "ttl_exists_exp_key"
	err = a.Set(ctx, keyExp, "val", 10*time.Second)
	require.NoError(t, err)

	ttl, exists, err = a.GetTTL(ctx, keyExp)
	require.NoError(t, err)
	assert.True(t, exists)
	assert.True(t, ttl > 0 && ttl <= 10*time.Second)

	keyPersist := "ttl_exists_persist_key"
	err = a.Set(ctx, keyPersist, "val", 0)
	require.NoError(t, err)

	ttl, exists, err = a.GetTTL(ctx, keyPersist)
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, -1*time.Second, ttl)

	_ = a.Delete(ctx, keyExp, keyPersist)
}
