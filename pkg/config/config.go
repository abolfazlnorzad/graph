package config

import (
	"github.com/abolfazlnorzad/graph/pkg/logger"
	"github.com/abolfazlnorzad/graph/pkg/metric"
	"github.com/abolfazlnorzad/graph/pkg/trace"
)

type Config struct {
	HTTPServer HTTPServer    `koanf:"http_server"`
	Logger     logger.Config `koanf:"logger"`
	Trace      trace.Config  `koanf:"trace"`
	Metric     metric.Config `koanf:"metric"`
}
type HTTPServer struct {
	Port int `koanf:"port"`
}
