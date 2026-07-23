package trace

import (
	"context"
	"fmt"
	"sync"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	"go.opentelemetry.io/otel/trace"
	"go.opentelemetry.io/otel/trace/noop"
)

var (
	globalTracerProvider *sdktrace.TracerProvider
	globalTracer         trace.Tracer
	once                 sync.Once
	globalTimeout        time.Duration
)

type Config struct {
	Enabled     bool          `koanf:"enabled"`
	Exporter    string        `koanf:"exporter"`
	Endpoint    string        `koanf:"endpoint"`
	ServiceName string        `koanf:"service_name"`
	SampleRate  float64       `koanf:"sample_rate"`
	Timeout     time.Duration `koanf:"timeout"`
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
		if cfg.Timeout > 0 {
			globalTimeout = cfg.Timeout
		} else {
			globalTimeout = 5 * time.Second
		}

		var exporter sdktrace.SpanExporter
		var err error
		switch cfg.Exporter {
		case "otlp":
			exporter, err = otlptracegrpc.New(context.Background(),
				otlptracegrpc.WithEndpoint(cfg.Endpoint),
				otlptracegrpc.WithInsecure(),
			)
			if err != nil {
				initErr = fmt.Errorf("failed to create OTLP trace exporter: %w", err)
				return
			}
		default:
			initErr = fmt.Errorf("unsupported trace exporter: %s", cfg.Exporter)
			return
		}

		sampler := sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRate))
		if cfg.SampleRate <= 0 {
			sampler = sdktrace.AlwaysSample()
		}

		tp := sdktrace.NewTracerProvider(
			sdktrace.WithBatcher(exporter),
			sdktrace.WithResource(res),
			sdktrace.WithSampler(sampler),
		)

		globalTracerProvider = tp
		globalTracer = tp.Tracer(cfg.ServiceName)

		otel.SetTracerProvider(tp)

		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))
	})

	return initErr
}

func Tracer() trace.Tracer {
	if globalTracer == nil {
		return noop.NewTracerProvider().Tracer("")
	}
	return globalTracer
}

func RecordError(span trace.Span, err error) {
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
	}
}

func Close() error {
	if globalTracerProvider != nil {
		ctx, cancel := context.WithTimeout(context.Background(), globalTimeout)
		defer cancel()

		err := globalTracerProvider.Shutdown(ctx)
		globalTracerProvider = nil
		globalTracer = nil
		return err
	}
	return nil
}


