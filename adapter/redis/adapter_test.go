package redisadapter_test

import (
	"context"
	"testing"

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

func TestAdapter_Client(t *testing.T) {
	a := &redisadapter.Adapter{}
	client := a.Client()
	assert.Nil(t, client)
}

func TestAdapter_Close_ConnectionFailure(t *testing.T) {
	cfg := redisadapter.Config{Host: "localhost", Port: 19999, DB: 0}
	a, err := redisadapter.New(context.Background(), cfg)
	// New fails on connection, so a should be nil
	assert.Nil(t, a)
	assert.Error(t, err)
}
