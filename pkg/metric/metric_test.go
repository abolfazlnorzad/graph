package metric_test

import (
	"context"
	"testing"
	"time"

	"github.com/abolfazlnorzad/graph/pkg/metric"
)

func TestNew_Disabled(t *testing.T) {
	m, err := metric.New(metric.Config{Enabled: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Meter() == nil {
		t.Error("meter should not be nil when disabled (noop)")
	}
}

func TestNew_UnsupportedExporter(t *testing.T) {
	_, err := metric.New(metric.Config{
		Enabled:     true,
		Exporter:    "unknown",
		ServiceName: "test",
	})
	if err == nil {
		t.Fatal("expected error for unsupported exporter")
	}
}

func TestNew_OTLP(t *testing.T) {
	m, err := metric.New(metric.Config{
		Enabled:     true,
		Exporter:    "otlp",
		Endpoint:    "localhost:4317",
		ServiceName: "test-service",
		Interval:    5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Meter() == nil {
		t.Fatal("meter should be set")
	}
}

func TestNew_OTLP_NoInterval(t *testing.T) {
	_, err := metric.New(metric.Config{
		Enabled:     true,
		Exporter:    "otlp",
		Endpoint:    "localhost:4317",
		ServiceName: "test-service",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNew_Prometheus(t *testing.T) {
	m, err := metric.New(metric.Config{
		Enabled:     true,
		Exporter:    "prometheus",
		ServiceName: "test-service",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m.Meter() == nil {
		t.Fatal("meter should be set")
	}
}

func TestMeter(t *testing.T) {
	m, err := metric.New(metric.Config{
		Enabled:     true,
		Exporter:    "prometheus",
		ServiceName: "test-service",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m.Meter() == nil {
		t.Fatal("Meter() returned nil")
	}
}

func TestMeter_NoopWhenDisabled(t *testing.T) {
	m, err := metric.New(metric.Config{Enabled: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if m.Meter() == nil {
		t.Fatal("Meter() should return a no-op meter, not nil")
	}
}

func TestClose(t *testing.T) {
	m, err := metric.New(metric.Config{
		Enabled:     true,
		Exporter:    "prometheus",
		ServiceName: "test-service",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := m.Close(context.Background()); err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}

func TestClose_NilProvider(t *testing.T) {
	m, err := metric.New(metric.Config{Enabled: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := m.Close(context.Background()); err != nil {
		t.Errorf("Close on nil provider returned error: %v", err)
	}
}
