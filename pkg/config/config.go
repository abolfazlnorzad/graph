package config

import (
	"github.com/abolfazlnorzad/graph/pkg/logger"
	"github.com/abolfazlnorzad/graph/pkg/metric"
	pgconfig "github.com/abolfazlnorzad/graph/pkg/postgresdb"
	redisconfig "github.com/abolfazlnorzad/graph/pkg/redis"
	"github.com/abolfazlnorzad/graph/pkg/trace"
)

type Config struct {
	HTTPServer HTTPServer        `koanf:"http_server"`
	Logger     logger.Config     `koanf:"logger"`
	Trace      trace.Config      `koanf:"trace"`
	Metric     metric.Config     `koanf:"metric"`
	Postgres   pgconfig.Config   `koanf:"postgres"`
	Redis      redisconfig.Config `koanf:"redis"`
}

type HTTPServer struct {
	Port int `koanf:"port"`
}
