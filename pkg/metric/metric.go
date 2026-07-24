package metric

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/metric"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

type Config struct {
	Enabled     bool          `koanf:"enabled"`
	Exporter    string        `koanf:"exporter"`
	Endpoint    string        `koanf:"endpoint"`
	ServiceName string        `koanf:"service_name"`
	Interval    time.Duration `koanf:"interval"`
}

type TelemetryManager struct {
	provider *sdkmetric.MeterProvider
	meter    metric.Meter
}

func New(cfg Config) (*TelemetryManager, error) {
	if !cfg.Enabled {
		return &TelemetryManager{
			meter: otel.Meter("noop"),
		}, nil
	}

	res := resource.NewWithAttributes(
		semconv.SchemaURL,
		semconv.ServiceNameKey.String(cfg.ServiceName),
	)

	var reader sdkmetric.Reader
	switch cfg.Exporter {
	case "otlp":
		exporter, err := otlpmetricgrpc.New(context.Background(),
			otlpmetricgrpc.WithEndpoint(cfg.Endpoint),
			otlpmetricgrpc.WithInsecure(),
		)
		if err != nil {
			return nil, fmt.Errorf("failed to create OTLP exporter: %w", err)
		}

		if cfg.Interval > 0 {
			reader = sdkmetric.NewPeriodicReader(exporter, sdkmetric.WithInterval(cfg.Interval))
		} else {
			reader = sdkmetric.NewPeriodicReader(exporter)
		}
	case "prometheus":
		promExporter, err := prometheus.New()
		if err != nil {
			return nil, fmt.Errorf("failed to create Prometheus exporter: %w", err)
		}
		reader = promExporter
	default:
		return nil, fmt.Errorf("unsupported metric exporter: %s", cfg.Exporter)
	}

	mp := sdkmetric.NewMeterProvider(
		sdkmetric.WithReader(reader),
		sdkmetric.WithResource(res),
	)

	otel.SetMeterProvider(mp)

	return &TelemetryManager{
		provider: mp,
		meter:    mp.Meter(cfg.ServiceName),
	}, nil
}

func (m *TelemetryManager) Meter() metric.Meter {
	return m.meter
}

func (m *TelemetryManager) Close(ctx context.Context) error {
	if m.provider != nil {
		return m.provider.Shutdown(ctx)
	}
	return nil
}
