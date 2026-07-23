package metric_test

import (
	"testing"
	"time"

	"github.com/abolfazlnorzad/graph/pkg/metric"
)

func TestInit_Disabled(t *testing.T) {
	metric.ResetGlobalsForTest()

	err := metric.Init(metric.Config{Enabled: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if metric.Meter() == nil {
		t.Error("globalMeter should be nil when disabled")
	}
}

func TestInit_UnsupportedExporter(t *testing.T) {
	metric.ResetGlobalsForTest()

	err := metric.Init(metric.Config{
		Enabled:     true,
		Exporter:    "unknown",
		ServiceName: "test",
	})
	if err == nil {
		t.Fatal("expected error for unsupported exporter")
	}
}

func TestInit_OTLP(t *testing.T) {
	metric.ResetGlobalsForTest()

	err := metric.Init(metric.Config{
		Enabled:     true,
		Exporter:    "otlp",
		Endpoint:    "localhost:4317",
		ServiceName: "test-service",
		Interval:    5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if metric.Meter() == nil {
		t.Fatal("globalMeter should be set")
	}
}

func TestInit_OTLP_NoInterval(t *testing.T) {
	metric.ResetGlobalsForTest()

	err := metric.Init(metric.Config{
		Enabled:     true,
		Exporter:    "otlp",
		Endpoint:    "localhost:4317",
		ServiceName: "test-service",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestInit_Prometheus(t *testing.T) {
	metric.ResetGlobalsForTest()

	err := metric.Init(metric.Config{
		Enabled:     true,
		Exporter:    "prometheus",
		ServiceName: "test-service",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if metric.Meter() == nil {
		t.Fatal("globalMeter should be set")
	}
}

func TestMeter(t *testing.T) {
	metric.ResetGlobalsForTest()

	metric.Init(metric.Config{
		Enabled:     true,
		Exporter:    "prometheus",
		ServiceName: "test-service",
	})

	m := metric.Meter()
	if m == nil {
		t.Fatal("Meter() returned nil")
	}
}

func TestMeter_NoopWhenNotInitialized(t *testing.T) {
	metric.ResetGlobalsForTest()

	m := metric.Meter()
	if m == nil {
		t.Fatal("Meter() should return a no-op meter, not nil")
	}
}

func TestClose(t *testing.T) {
	metric.ResetGlobalsForTest()

	metric.Init(metric.Config{
		Enabled:     true,
		Exporter:    "prometheus",
		ServiceName: "test-service",
	})

	err := metric.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}

func TestClose_NilProvider(t *testing.T) {
	metric.ResetGlobalsForTest()

	err := metric.Close()
	if err != nil {
		t.Errorf("Close on nil provider returned error: %v", err)
	}
}
