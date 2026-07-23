package config

import "github.com/abolfazlnorzad/graph/pkg/logger"

type Config struct {
	Logger logger.Config `koanf:"logger"`
}
