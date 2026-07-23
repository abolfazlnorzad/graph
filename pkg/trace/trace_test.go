package trace_test

import (
	"testing"
	"time"

	"github.com/abolfazlnorzad/graph/pkg/trace"
)

func TestInit_Disabled(t *testing.T) {
	trace.ResetGlobalsForTest()

	err := trace.Init(trace.Config{Enabled: false})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trace.Tracer() == nil {
		t.Error("globalTracer should be nil when disabled")
	}
}

func TestInit_UnsupportedExporter(t *testing.T) {
	trace.ResetGlobalsForTest()

	err := trace.Init(trace.Config{
		Enabled:     true,
		Exporter:    "unknown",
		ServiceName: "test",
		SampleRate:  1.0,
	})
	if err == nil {
		t.Fatal("expected error for unsupported exporter")
	}
}

func TestInit_OTLP(t *testing.T) {
	trace.ResetGlobalsForTest()

	err := trace.Init(trace.Config{
		Enabled:     true,
		Exporter:    "otlp",
		Endpoint:    "localhost:4317",
		ServiceName: "test-service",
		SampleRate:  1.0,
		Timeout:     5 * time.Second,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if trace.Tracer() == nil {
		t.Fatal("globalTracer should be set")
	}
}

func TestInit_SampleRateZero(t *testing.T) {
	trace.ResetGlobalsForTest()

	err := trace.Init(trace.Config{
		Enabled:     true,
		Exporter:    "otlp",
		Endpoint:    "localhost:4317",
		ServiceName: "test-service",
		SampleRate:  0,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTracer(t *testing.T) {
	trace.ResetGlobalsForTest()

	trace.Init(trace.Config{
		Enabled:     true,
		Exporter:    "otlp",
		Endpoint:    "localhost:4317",
		ServiceName: "test-service",
		SampleRate:  1.0,
	})

	tr := trace.Tracer()
	if tr == nil {
		t.Fatal("Tracer() returned nil")
	}
}

func TestTracer_NoOpWhenNotInitialized(t *testing.T) {
	trace.ResetGlobalsForTest()

	tr := trace.Tracer()
	if tr == nil {
		t.Fatal("Tracer() should return no-op tracer when not initialized")
	}
}

func TestClose(t *testing.T) {
	trace.ResetGlobalsForTest()

	trace.Init(trace.Config{
		Enabled:     true,
		Exporter:    "otlp",
		Endpoint:    "localhost:4317",
		ServiceName: "test-service",
		SampleRate:  1.0,
	})

	err := trace.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}

func TestClose_NilProvider(t *testing.T) {
	trace.ResetGlobalsForTest()

	err := trace.Close()
	if err != nil {
		t.Errorf("Close on nil provider returned error: %v", err)
	}
}
