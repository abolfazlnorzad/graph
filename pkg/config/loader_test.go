package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/abolfazlnorzad/graph/pkg/config"
)

func TestDefaultCallbackEnv(t *testing.T) {
	tests := []struct {
		name      string
		source    string
		prefix    string
		separator string
		expected  string
	}{
		{
			name:      "strips prefix and maps separator to dot",
			source:    "APP_HOST",
			prefix:    "APP_",
			separator: "_",
			expected:  "host",
		},
		{
			name:      "nested key with multiple separators",
			source:    "APP_DATABASE_HOST",
			prefix:    "APP_",
			separator: "_",
			expected:  "database.host",
		},
		{
			name:      "empty prefix does not strip",
			source:    "MY_KEY",
			prefix:    "",
			separator: "_",
			expected:  "my.key",
		},
		{
			name:      "no separator in key",
			source:    "APP_PORT",
			prefix:    "APP_",
			separator: "_",
			expected:  "port",
		},
		{
			name:      "lowercases entire key",
			source:    "APP_SOME_KEY",
			prefix:    "APP_",
			separator: "_",
			expected:  "some.key",
		},
		{
			name:      "prefix not present in key",
			source:    "HOST",
			prefix:    "APP_",
			separator: "_",
			expected:  "host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := config.DefaultCallbackEnv(tt.source, tt.prefix, tt.separator)
			if result != tt.expected {
				t.Errorf("defaultCallbackEnv(%q, %q, %q) = %q, want %q",
					tt.source, tt.prefix, tt.separator, result, tt.expected)
			}
		})
	}
}

func TestLoad_EnvOnly(t *testing.T) {
	t.Setenv("TESTAPP_HOST", "example.com")
	t.Setenv("TESTAPP_PORT", "9090")

	var cfg struct {
		Host string `koanf:"host"`
		Port string `koanf:"port"`
	}

	err := config.Load(config.Option{
		Prefix:    "TESTAPP_",
		Delimiter: ".",
		Separator: "_",
	}, &cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Host != "example.com" {
		t.Errorf("Host = %q, want %q", cfg.Host, "example.com")
	}
	if cfg.Port != "9090" {
		t.Errorf("Port = %q, want %q", cfg.Port, "9090")
	}
}

func TestLoad_YamlOnly(t *testing.T) {
	var cfg struct {
		Host     string `koanf:"host"`
		Port     int    `koanf:"port"`
		Database struct {
			Name string `koanf:"name"`
			User string `koanf:"user"`
		} `koanf:"database"`
	}

	err := config.Load(config.Option{
		Delimiter:    ".",
		YamlFilePath: filepath.Join("testdata", "valid.yaml"),
	}, &cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Host != "localhost" {
		t.Errorf("Host = %q, want %q", cfg.Host, "localhost")
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want %d", cfg.Port, 8080)
	}
	if cfg.Database.Name != "mydb" {
		t.Errorf("Database.Name = %q, want %q", cfg.Database.Name, "mydb")
	}
	if cfg.Database.User != "admin" {
		t.Errorf("Database.User = %q, want %q", cfg.Database.User, "admin")
	}
}

func TestLoad_YamlAndEnv(t *testing.T) {
	t.Setenv("YAE_HOST", "env-override.com")

	var cfg struct {
		Host string `koanf:"host"`
		Port int    `koanf:"port"`
	}

	err := config.Load(config.Option{
		Prefix:       "YAE_",
		Delimiter:    ".",
		Separator:    "_",
		YamlFilePath: filepath.Join("testdata", "valid.yaml"),
	}, &cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Host != "env-override.com" {
		t.Errorf("Host = %q, want %q (env should override yaml)", cfg.Host, "env-override.com")
	}
	if cfg.Port != 8080 {
		t.Errorf("Port = %d, want %d", cfg.Port, 8080)
	}
}

func TestLoad_CustomCallback(t *testing.T) {
	t.Setenv("CUSTOM_APP_HOST", "custom.example.com")

	var cfg struct {
		Host string `koanf:"host"`
	}

	customCallback := func(source string) string {
		return "host"
	}

	err := config.Load(config.Option{
		Prefix:      "CUSTOM_APP_",
		Delimiter:   ".",
		Separator:   "_",
		CallbackEnv: customCallback,
	}, &cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Host != "custom.example.com" {
		t.Errorf("Host = %q, want %q", cfg.Host, "custom.example.com")
	}
}

func TestLoad_EmptyYamlPath(t *testing.T) {
	t.Setenv("EYP_NAME", "vira")

	var cfg struct {
		Name string `koanf:"name"`
	}

	err := config.Load(config.Option{
		Prefix:    "EYP_",
		Delimiter: ".",
		Separator: "_",
	}, &cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Name != "vira" {
		t.Errorf("Name = %q, want %q", cfg.Name, "vira")
	}
}

func TestLoad_InvalidYamlFile(t *testing.T) {
	var cfg struct {
		Host string `koanf:"host"`
	}

	err := config.Load(config.Option{
		Delimiter:    ".",
		YamlFilePath: filepath.Join("testdata", "nonexistent.yaml"),
	}, &cfg)
	if err == nil {
		t.Fatal("expected error for non-existent yaml file, got nil")
	}
}

func TestLoad_InvalidYamlContent(t *testing.T) {
	var cfg struct {
		Host string `koanf:"host"`
	}

	err := config.Load(config.Option{
		Delimiter:    ".",
		YamlFilePath: filepath.Join("testdata", "invalid.yaml"),
	}, &cfg)
	if err == nil {
		t.Fatal("expected error for malformed yaml, got nil")
	}
}

func TestLoad_EmptyOptions(t *testing.T) {
	var cfg struct {
		Host string `koanf:"host"`
	}

	err := config.Load(config.Option{}, &cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestLoad_EnvPrefixMapping(t *testing.T) {
	t.Setenv("MYAPP_SERVER_HOST", "mapped.example.com")
	t.Setenv("MYAPP_SERVER_PORT", "3000")

	var cfg struct {
		Server struct {
			Host string `koanf:"host"`
			Port string `koanf:"port"`
		} `koanf:"server"`
	}

	err := config.Load(config.Option{
		Prefix:    "MYAPP_",
		Delimiter: ".",
		Separator: "_",
	}, &cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.Server.Host != "mapped.example.com" {
		t.Errorf("Server.Host = %q, want %q", cfg.Server.Host, "mapped.example.com")
	}
	if cfg.Server.Port != "3000" {
		t.Errorf("Server.Port = %q, want %q", cfg.Server.Port, "3000")
	}
}

func TestLoad_UnmarshalError(t *testing.T) {
	var notAPointer struct {
		Host string `koanf:"host"`
	}

	err := config.Load(config.Option{
		Delimiter: ".",
	}, notAPointer)
	if err == nil {
		t.Fatal("expected error when passing non-pointer to Unmarshal, got nil")
	}
}

func TestLoad_YamlFileParseError(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "bad.yaml")
	if err := os.WriteFile(tmpFile, []byte("{{invalid yaml"), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	var cfg struct {
		Host string `koanf:"host"`
	}

	err := config.Load(config.Option{
		Delimiter:    ".",
		YamlFilePath: tmpFile,
	}, &cfg)
	if err == nil {
		t.Fatal("expected error for parse-failing yaml, got nil")
	}
}
