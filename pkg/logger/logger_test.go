package logger_test

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/abolfazlnorzad/graph/pkg/logger"
)

func TestMapLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"unknown", slog.LevelInfo},
		{"", slog.LevelInfo},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := logger.MapLevel(tt.input)
			if result != tt.expected {
				t.Errorf("mapLevel(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

func TestResolveLogPath_CustomPath(t *testing.T) {
	cfg := logger.Config{FilePath: "logs/app.log"}
	path, err := logger.ResolveLogPath(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(path, "logs/app.log") {
		t.Errorf("path = %q, want suffix logs/app.log", path)
	}
}

func TestResolveLogPath_AbsolutePath(t *testing.T) {
	cfg := logger.Config{FilePath: "/tmp/app.log"}
	_, err := logger.ResolveLogPath(cfg)
	if err == nil {
		t.Fatal("expected error for absolute path, got nil")
	}
}

func TestResolveLogPath_DirectoryTraversal(t *testing.T) {
	cfg := logger.Config{FilePath: "../etc/passwd"}
	_, err := logger.ResolveLogPath(cfg)
	if err == nil {
		t.Fatal("expected error for directory traversal, got nil")
	}
}

func TestResolveLogPath_EmptyPath(t *testing.T) {
	cfg := logger.Config{}
	path, err := logger.ResolveLogPath(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasSuffix(path, "logs/app.log") {
		t.Errorf("path = %q, want suffix logs/app.log", path)
	}
}

func TestInit_Success(t *testing.T) {
	origDir, _ := os.Getwd()
	dir := t.TempDir()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	logger.ResetGlobalsForTest()

	err := logger.Init(logger.Config{
		Level:    "info",
		FilePath: "test.log",
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	l := logger.L()
	if l == nil {
		t.Fatal("L() returned nil")
	}

	l.Info("test message", "key", "value")

	logger.Close()

	data, err := os.ReadFile(filepath.Join(dir, "test.log"))
	if err != nil {
		t.Fatalf("failed to read log file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("log file is empty")
	}
	if !strings.Contains(string(data), "test message") {
		t.Errorf("log file missing 'test message': %s", data)
	}
}

func TestInit_InvalidPath(t *testing.T) {
	logger.ResetGlobalsForTest()

	err := logger.Init(logger.Config{
		FilePath: "/absolutely/not/allowed/path.log",
	})
	if err == nil {
		t.Fatal("expected error for absolute path, got nil")
	}
}

func TestClose(t *testing.T) {
	origDir, _ := os.Getwd()
	dir := t.TempDir()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	logger.ResetGlobalsForTest()

	err := logger.Init(logger.Config{
		Level:    "info",
		FilePath: "close_test.log",
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	err = logger.Close()
	if err != nil {
		t.Errorf("Close returned error: %v", err)
	}
}

func TestClose_DoubleCall(t *testing.T) {
	origDir, _ := os.Getwd()
	dir := t.TempDir()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	logger.ResetGlobalsForTest()

	err := logger.Init(logger.Config{
		Level:    "info",
		FilePath: "double_close.log",
	})
	if err != nil {
		t.Fatalf("Init failed: %v", err)
	}

	logger.Close()
	err = logger.Close()
	if err != nil {
		t.Errorf("second Close returned error: %v", err)
	}
}

func TestNew_Independence(t *testing.T) {
	origDir, _ := os.Getwd()
	dir := t.TempDir()
	os.Chdir(dir)
	defer os.Chdir(origDir)

	l1, c1, err := logger.New(logger.Config{
		Level:    "info",
		FilePath: "l1.log",
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer c1.Close()

	l2, c2, err := logger.New(logger.Config{
		Level:    "debug",
		FilePath: "l2.log",
	})
	if err != nil {
		t.Fatalf("New failed: %v", err)
	}
	defer c2.Close()

	l1.Info("from l1")
	l2.Debug("from l2")

	c1.Close()
	c2.Close()

	data1, _ := os.ReadFile(filepath.Join(dir, "l1.log"))
	data2, _ := os.ReadFile(filepath.Join(dir, "l2.log"))

	if !strings.Contains(string(data1), "from l1") {
		t.Error("l1 log missing 'from l1'")
	}
	if !strings.Contains(string(data2), "from l2") {
		t.Error("l2 log missing 'from l2'")
	}
}

func TestNew_InvalidPath(t *testing.T) {
	_, _, err := logger.New(logger.Config{
		FilePath: "/no/such/dir/file.log",
	})
	if err == nil {
		t.Fatal("expected error for invalid path, got nil")
	}
}

func TestL_NotInitialized(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic when L() called before Init()")
		}
	}()

	logger.ResetGlobalsForTest()
	logger.L()
}
