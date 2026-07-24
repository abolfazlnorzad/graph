package postgresdb_test

import (
	"testing"

	postgresdb2 "github.com/abolfazlnorzad/graph/pkg/postgresdb"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestBuildDSN_DefaultConfig(t *testing.T) {
	cfg := postgresdb2.NewConfig()
	got := postgresdb2.BuildDSN(cfg)
	want := "postgres://root:@127.0.0.1:5432/dbname?sslmode=disable"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildDSN_CustomConfig(t *testing.T) {
	cfg := postgresdb2.Config{
		Host:     "db.example.com",
		Port:     5433,
		Username: "admin",
		Password: "secret",
		DBName:   "mydb",
		SSLMode:  "require",
	}
	got := postgresdb2.BuildDSN(cfg)
	want := "postgres://admin:secret@db.example.com:5433/mydb?sslmode=require"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestNewConfig_Defaults(t *testing.T) {
	cfg := postgresdb2.NewConfig()

	if cfg.Host != "127.0.0.1" {
		t.Errorf("Host: got %q, want %q", cfg.Host, "127.0.0.1")
	}
	if cfg.Port != 5432 {
		t.Errorf("Port: got %d, want %d", cfg.Port, 5432)
	}
	if cfg.Username != "root" {
		t.Errorf("Username: got %q, want %q", cfg.Username, "root")
	}
	if cfg.Password != "" {
		t.Errorf("Password: got %q, want empty", cfg.Password)
	}
	if cfg.DBName != "dbname" {
		t.Errorf("DBName: got %q, want %q", cfg.DBName, "dbname")
	}
	if cfg.Schema != "public" {
		t.Errorf("Schema: got %q, want %q", cfg.Schema, "public")
	}
	if cfg.SSLMode != "disable" {
		t.Errorf("SSLMode: got %q, want %q", cfg.SSLMode, "disable")
	}
	if cfg.MaxConns != 10 {
		t.Errorf("MaxConns: got %d, want %d", cfg.MaxConns, 10)
	}
	if cfg.MinConns != 2 {
		t.Errorf("MinConns: got %d, want %d", cfg.MinConns, 2)
	}
	if cfg.MaxConnLifetime != 3600 {
		t.Errorf("MaxConnLifetime: got %d, want %d", cfg.MaxConnLifetime, 3600)
	}
	if cfg.MaxConnIdleTime != 600 {
		t.Errorf("MaxConnIdleTime: got %d, want %d", cfg.MaxConnIdleTime, 600)
	}
	if cfg.HealthCheckPeriod != 60 {
		t.Errorf("HealthCheckPeriod: got %d, want %d", cfg.HealthCheckPeriod, 60)
	}
	if cfg.PathOfMigrations != "./migrations" {
		t.Errorf("PathOfMigrations: got %q, want %q", cfg.PathOfMigrations, "./migrations")
	}
}

func TestNewConfig_WithHost(t *testing.T) {
	cfg := postgresdb2.NewConfig(postgresdb2.WithHost("remote.host"))
	if cfg.Host != "remote.host" {
		t.Errorf("Host: got %q, want %q", cfg.Host, "remote.host")
	}
}

func TestNewConfig_WithPort(t *testing.T) {
	cfg := postgresdb2.NewConfig(postgresdb2.WithPort(5433))
	if cfg.Port != 5433 {
		t.Errorf("Port: got %d, want %d", cfg.Port, 5433)
	}
}

func TestNewConfig_WithUsername(t *testing.T) {
	cfg := postgresdb2.NewConfig(postgresdb2.WithUsername("admin"))
	if cfg.Username != "admin" {
		t.Errorf("Username: got %q, want %q", cfg.Username, "admin")
	}
}

func TestNewConfig_WithPassword(t *testing.T) {
	cfg := postgresdb2.NewConfig(postgresdb2.WithPassword("s3cret"))
	if cfg.Password != "s3cret" {
		t.Errorf("Password: got %q, want %q", cfg.Password, "s3cret")
	}
}

func TestNewConfig_WithDBName(t *testing.T) {
	cfg := postgresdb2.NewConfig(postgresdb2.WithDBName("production"))
	if cfg.DBName != "production" {
		t.Errorf("DBName: got %q, want %q", cfg.DBName, "production")
	}
}

func TestNewConfig_WithSSLMode(t *testing.T) {
	cfg := postgresdb2.NewConfig(postgresdb2.WithSSLMode("require"))
	if cfg.SSLMode != "require" {
		t.Errorf("SSLMode: got %q, want %q", cfg.SSLMode, "require")
	}
}

func TestNewConfig_WithMaxConns(t *testing.T) {
	cfg := postgresdb2.NewConfig(postgresdb2.WithMaxConns(20))
	if cfg.MaxConns != 20 {
		t.Errorf("MaxConns: got %d, want %d", cfg.MaxConns, 20)
	}
}

func TestNewConfig_WithMinConns(t *testing.T) {
	cfg := postgresdb2.NewConfig(postgresdb2.WithMinConns(5))
	if cfg.MinConns != 5 {
		t.Errorf("MinConns: got %d, want %d", cfg.MinConns, 5)
	}
}

func TestNewConfig_WithMaxConnLifetime(t *testing.T) {
	cfg := postgresdb2.NewConfig(postgresdb2.WithMaxConnLifetime(7200))
	if cfg.MaxConnLifetime != 7200 {
		t.Errorf("MaxConnLifetime: got %d, want %d", cfg.MaxConnLifetime, 7200)
	}
}

func TestNewConfig_WithMaxConnIdleTime(t *testing.T) {
	cfg := postgresdb2.NewConfig(postgresdb2.WithMaxConnIdleTime(300))
	if cfg.MaxConnIdleTime != 300 {
		t.Errorf("MaxConnIdleTime: got %d, want %d", cfg.MaxConnIdleTime, 300)
	}
}

func TestNewConfig_WithHealthCheckPeriod(t *testing.T) {
	cfg := postgresdb2.NewConfig(postgresdb2.WithHealthCheckPeriod(120))
	if cfg.HealthCheckPeriod != 120 {
		t.Errorf("HealthCheckPeriod: got %d, want %d", cfg.HealthCheckPeriod, 120)
	}
}

func TestNewConfig_WithPathOfMigrations(t *testing.T) {
	cfg := postgresdb2.NewConfig(postgresdb2.WithPathOfMigrations("/opt/migrations"))
	if cfg.PathOfMigrations != "/opt/migrations" {
		t.Errorf("PathOfMigrations: got %q, want %q", cfg.PathOfMigrations, "/opt/migrations")
	}
}

func TestConnect_NewWithConfigError(t *testing.T) {
	// pgxpool rejects negative MaxConns
	cfg := postgresdb2.Config{
		Host:              "127.0.0.1",
		Port:              5432,
		Username:          "test",
		Password:          "test",
		DBName:            "test",
		SSLMode:           "disable",
		MaxConns:          -1,
		MinConns:          0,
		MaxConnLifetime:   0,
		MaxConnIdleTime:   0,
		HealthCheckPeriod: 0,
	}
	_, err := postgresdb2.Connect(cfg)
	if err == nil {
		t.Fatal("expected error with negative MaxConns")
	}
}

func TestConnect_PingError(t *testing.T) {
	// pgxpool allows pool creation with valid config even when host is unreachable.
	// Ping then fails because the backend is not reachable.
	cfg := postgresdb2.Config{
		Host:              "127.0.0.1",
		Port:              1,
		Username:          "test",
		Password:          "test",
		DBName:            "test",
		SSLMode:           "disable",
		MaxConns:          1,
		MinConns:          0,
		MaxConnLifetime:   1,
		MaxConnIdleTime:   1,
		HealthCheckPeriod: 1,
	}
	_, err := postgresdb2.Connect(cfg)
	if err == nil {
		t.Fatal("expected ping error with unreachable host")
	}
}

func TestClose_WithRealPool(t *testing.T) {
	// Create a real pool that connects to nothing — pgxpool allows this
	// as long as the DSN is parseable. The pool object is valid even
	// though the backend is unreachable.
	dsn := postgresdb2.BuildDSN(postgresdb2.Config{
		Host:     "127.0.0.1",
		Port:     1,
		Username: "test",
		Password: "test",
		DBName:   "test",
		SSLMode:  "disable",
	})
	poolConfig, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		t.Fatalf("unexpected parse error: %v", err)
	}
	poolConfig.MaxConns = 1
	poolConfig.MinConns = 0

	ctx := t.Context()
	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		t.Skipf("cannot create pool for Close test: %v", err)
	}

	db := &postgresdb2.Database{Pool: pool}
	db.Close()
}

func TestNewConfig_MultipleOptions(t *testing.T) {
	cfg := postgresdb2.NewConfig(
		postgresdb2.WithHost("prod.db"),
		postgresdb2.WithPort(5432),
		postgresdb2.WithUsername("app"),
		postgresdb2.WithPassword("pass"),
		postgresdb2.WithDBName("graph"),
		postgresdb2.WithSSLMode("require"),
		postgresdb2.WithMaxConns(50),
		postgresdb2.WithMinConns(10),
	)

	got := postgresdb2.BuildDSN(cfg)
	want := "postgres://app:pass@prod.db:5432/graph?sslmode=require"
	if got != want {
		t.Errorf("postgresdb.BuildDSN with options: got %q, want %q", got, want)
	}
}
