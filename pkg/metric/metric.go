package metric

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

var (
	globalMeterProvider *sdkmetric.MeterProvider
	globalMeter         metric.Meter
	once                sync.Once
)

type Config struct {
	Enabled     bool          `koanf:"enabled"`
	Exporter    string        `koanf:"exporter"`
	Endpoint    string        `koanf:"endpoint"`
	ServiceName string        `koanf:"service_name"`
	Interval    time.Duration `koanf:"interval"`
}

func Init(cfg Config) error {
	if !cfg.Enabled {
		return nil
	}

	var initErr error
	once.Do(func() {
		res := resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String(cfg.ServiceName),
		)

		var reader sdkmetric.Reader
		switch cfg.Exporter {
		case "otlp":
			exporter, expErr := otlpmetricgrpc.New(context.Background(),
				otlpmetricgrpc.WithEndpoint(cfg.Endpoint),
				otlpmetricgrpc.WithInsecure(),
			)
			if expErr != nil {
				initErr = fmt.Errorf("failed to create OTLP metric exporter: %w", expErr)
				return
			}
			if cfg.Interval > 0 {
				reader = sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(cfg.Interval))
			} else {
				reader = sdkmetric.NewPeriodicReader(exporter)
			}
		case "prometheus":
			promExporter, expErr := prometheus.New()
			if expErr != nil {
				initErr = fmt.Errorf("failed to create Prometheus metric exporter: %w", expErr)
				return
			}
			reader = promExporter
		default:
			initErr = fmt.Errorf("unsupported metric exporter: %s", cfg.Exporter)
			return
		}

		mp := sdkmetric.NewMeterProvider(
			sdkmetric.WithReader(reader),
			sdkmetric.WithResource(res),
		)

		globalMeterProvider = mp
		globalMeter = mp.Meter(cfg.ServiceName)

		otel.SetMeterProvider(mp)
	})

	return initErr
}

func Meter() metric.Meter {
	if globalMeter == nil {
		return otel.Meter("noop")
	}
	return globalMeter
}

func Close() error {
	if globalMeterProvider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		err := globalMeterProvider.Shutdown(ctx)
		globalMeterProvider = nil
		globalMeter = nil
		return err
	}
	return nil
}
