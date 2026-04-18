package platform

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestLoadDatabaseConfig_ValidWithDefaults(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/testdb")
	t.Setenv("DB_MAX_CONNECTIONS", "")
	t.Setenv("DB_MIN_CONNECTIONS", "")
	t.Setenv("DB_MAX_CONN_LIFETIME", "")
	t.Setenv("DB_MAX_CONN_IDLE_TIME", "")
	t.Setenv("DB_HEALTH_CHECK_PERIOD", "")

	cfg, err := LoadDatabaseConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.DSN != "postgres://user:pass@localhost:5432/testdb" {
		t.Fatalf("expected DSN %q, got %q", "postgres://user:pass@localhost:5432/testdb", cfg.DSN)
	}
	if cfg.MaxConnections != 3 {
		t.Fatalf("expected MaxConnections 3, got %d", cfg.MaxConnections)
	}
	if cfg.MinConnections != 0 {
		t.Fatalf("expected MinConnections 0, got %d", cfg.MinConnections)
	}
	if cfg.MaxConnLifetime != 5*time.Minute {
		t.Fatalf("expected MaxConnLifetime 5m, got %v", cfg.MaxConnLifetime)
	}
	if cfg.MaxConnIdleTime != 30*time.Second {
		t.Fatalf("expected MaxConnIdleTime 30s, got %v", cfg.MaxConnIdleTime)
	}
	if cfg.HealthCheckPeriod != 15*time.Second {
		t.Fatalf("expected HealthCheckPeriod 15s, got %v", cfg.HealthCheckPeriod)
	}
}

func TestLoadDatabaseConfig_ValidWithAllOverrides(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/testdb")
	t.Setenv("DB_MAX_CONNECTIONS", "5")
	t.Setenv("DB_MIN_CONNECTIONS", "1")
	t.Setenv("DB_MAX_CONN_LIFETIME", "10m")
	t.Setenv("DB_MAX_CONN_IDLE_TIME", "1m")
	t.Setenv("DB_HEALTH_CHECK_PERIOD", "30s")

	cfg, err := LoadDatabaseConfig()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.MaxConnections != 5 {
		t.Fatalf("expected MaxConnections 5, got %d", cfg.MaxConnections)
	}
	if cfg.MinConnections != 1 {
		t.Fatalf("expected MinConnections 1, got %d", cfg.MinConnections)
	}
	if cfg.MaxConnLifetime != 10*time.Minute {
		t.Fatalf("expected MaxConnLifetime 10m, got %v", cfg.MaxConnLifetime)
	}
	if cfg.MaxConnIdleTime != 1*time.Minute {
		t.Fatalf("expected MaxConnIdleTime 1m, got %v", cfg.MaxConnIdleTime)
	}
	if cfg.HealthCheckPeriod != 30*time.Second {
		t.Fatalf("expected HealthCheckPeriod 30s, got %v", cfg.HealthCheckPeriod)
	}
}

func TestLoadDatabaseConfig_MissingDatabaseURL(t *testing.T) {
	t.Setenv("DATABASE_URL", "")

	_, err := LoadDatabaseConfig()
	if err == nil {
		t.Fatal("expected error for missing DATABASE_URL")
	}
	if !strings.Contains(err.Error(), "DATABASE_URL is required") {
		t.Fatalf("expected error to mention DATABASE_URL, got: %v", err)
	}
}

func TestLoadDatabaseConfig_InvalidNumericOverrides(t *testing.T) {
	tests := []struct {
		name   string
		envVar string
		value  string
		errMsg string
	}{
		{
			name:   "invalid DB_MAX_CONNECTIONS",
			envVar: "DB_MAX_CONNECTIONS",
			value:  "abc",
			errMsg: "invalid DB_MAX_CONNECTIONS",
		},
		{
			name:   "invalid DB_MIN_CONNECTIONS",
			envVar: "DB_MIN_CONNECTIONS",
			value:  "xyz",
			errMsg: "invalid DB_MIN_CONNECTIONS",
		},
		{
			name:   "invalid DB_MAX_CONN_LIFETIME",
			envVar: "DB_MAX_CONN_LIFETIME",
			value:  "not-a-duration",
			errMsg: "invalid DB_MAX_CONN_LIFETIME",
		},
		{
			name:   "invalid DB_MAX_CONN_IDLE_TIME",
			envVar: "DB_MAX_CONN_IDLE_TIME",
			value:  "???",
			errMsg: "invalid DB_MAX_CONN_IDLE_TIME",
		},
		{
			name:   "invalid DB_HEALTH_CHECK_PERIOD",
			envVar: "DB_HEALTH_CHECK_PERIOD",
			value:  "bad",
			errMsg: "invalid DB_HEALTH_CHECK_PERIOD",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("DATABASE_URL", "postgres://user:pass@localhost:5432/testdb")
			t.Setenv("DB_MAX_CONNECTIONS", "")
			t.Setenv("DB_MIN_CONNECTIONS", "")
			t.Setenv("DB_MAX_CONN_LIFETIME", "")
			t.Setenv("DB_MAX_CONN_IDLE_TIME", "")
			t.Setenv("DB_HEALTH_CHECK_PERIOD", "")

			t.Setenv(tc.envVar, tc.value)

			_, err := LoadDatabaseConfig()
			if err == nil {
				t.Fatalf("expected error for invalid %s", tc.envVar)
			}
			if !strings.Contains(err.Error(), tc.errMsg) {
				t.Fatalf("expected error to contain %q, got: %v", tc.errMsg, err)
			}
		})
	}
}

func TestBuildPoolConfig_ValidConfig(t *testing.T) {
	cfg := DatabaseConfig{
		DSN:               "postgres://user:pass@localhost:5432/testdb",
		MaxConnections:    5,
		MinConnections:    2,
		MaxConnLifetime:   10 * time.Minute,
		MaxConnIdleTime:   1 * time.Minute,
		HealthCheckPeriod: 30 * time.Second,
	}

	poolCfg, err := buildPoolConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if poolCfg.MaxConns != 5 {
		t.Fatalf("expected MaxConns 5, got %d", poolCfg.MaxConns)
	}
	if poolCfg.MinConns != 2 {
		t.Fatalf("expected MinConns 2, got %d", poolCfg.MinConns)
	}
	if poolCfg.MaxConnLifetime != 10*time.Minute {
		t.Fatalf("expected MaxConnLifetime 10m, got %v", poolCfg.MaxConnLifetime)
	}
	if poolCfg.MaxConnIdleTime != 1*time.Minute {
		t.Fatalf("expected MaxConnIdleTime 1m, got %v", poolCfg.MaxConnIdleTime)
	}
	if poolCfg.HealthCheckPeriod != 30*time.Second {
		t.Fatalf("expected HealthCheckPeriod 30s, got %v", poolCfg.HealthCheckPeriod)
	}
}

func TestBuildPoolConfig_InvalidDSN(t *testing.T) {
	cfg := DatabaseConfig{
		DSN: "host='unclosed",
	}

	_, err := buildPoolConfig(cfg)
	if err == nil {
		t.Fatal("expected error for invalid DSN")
	}
	if !strings.Contains(err.Error(), "parsing connection string") {
		t.Fatalf("expected error to mention parsing, got: %v", err)
	}
}

func TestNewPool_InvalidDSN(t *testing.T) {
	cfg := DatabaseConfig{
		DSN:               "host='unclosed",
		MaxConnections:    3,
		MinConnections:    0,
		MaxConnLifetime:   5 * time.Minute,
		MaxConnIdleTime:   30 * time.Second,
		HealthCheckPeriod: 15 * time.Second,
	}

	_, err := NewPool(context.Background(), cfg)
	if err == nil {
		t.Fatal("expected error for invalid DSN")
	}
}

func TestNewPool_ValidConfig(t *testing.T) {
	cfg := DatabaseConfig{
		DSN:               "postgres://user:pass@localhost:5432/testdb",
		MaxConnections:    3,
		MinConnections:    0,
		MaxConnLifetime:   5 * time.Minute,
		MaxConnIdleTime:   30 * time.Second,
		HealthCheckPeriod: 15 * time.Second,
	}

	pool, err := NewPool(context.Background(), cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer pool.Close()

	stat := pool.Stat()
	if stat.MaxConns() != 3 {
		t.Fatalf("expected pool MaxConns 3, got %d", stat.MaxConns())
	}
}
