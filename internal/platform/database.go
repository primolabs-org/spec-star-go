package platform

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

type DatabaseConfig struct {
	DSN               string
	MaxConnections    int32
	MinConnections    int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

func LoadDatabaseConfig() (DatabaseConfig, error) {
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		return DatabaseConfig{}, fmt.Errorf("DATABASE_URL is required")
	}

	cfg := DatabaseConfig{
		DSN:               dsn,
		MaxConnections:    3,
		MinConnections:    0,
		MaxConnLifetime:   5 * time.Minute,
		MaxConnIdleTime:   30 * time.Second,
		HealthCheckPeriod: 15 * time.Second,
	}

	if v := os.Getenv("DB_MAX_CONNECTIONS"); v != "" {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			return DatabaseConfig{}, fmt.Errorf("invalid DB_MAX_CONNECTIONS %q: %w", v, err)
		}
		cfg.MaxConnections = int32(n)
	}

	if v := os.Getenv("DB_MIN_CONNECTIONS"); v != "" {
		n, err := strconv.ParseInt(v, 10, 32)
		if err != nil {
			return DatabaseConfig{}, fmt.Errorf("invalid DB_MIN_CONNECTIONS %q: %w", v, err)
		}
		cfg.MinConnections = int32(n)
	}

	if v := os.Getenv("DB_MAX_CONN_LIFETIME"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return DatabaseConfig{}, fmt.Errorf("invalid DB_MAX_CONN_LIFETIME %q: %w", v, err)
		}
		cfg.MaxConnLifetime = d
	}

	if v := os.Getenv("DB_MAX_CONN_IDLE_TIME"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return DatabaseConfig{}, fmt.Errorf("invalid DB_MAX_CONN_IDLE_TIME %q: %w", v, err)
		}
		cfg.MaxConnIdleTime = d
	}

	if v := os.Getenv("DB_HEALTH_CHECK_PERIOD"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return DatabaseConfig{}, fmt.Errorf("invalid DB_HEALTH_CHECK_PERIOD %q: %w", v, err)
		}
		cfg.HealthCheckPeriod = d
	}

	return cfg, nil
}

func buildPoolConfig(cfg DatabaseConfig) (*pgxpool.Config, error) {
	poolCfg, err := pgxpool.ParseConfig(cfg.DSN)
	if err != nil {
		return nil, fmt.Errorf("parsing connection string: %w", err)
	}

	poolCfg.MaxConns = cfg.MaxConnections
	poolCfg.MinConns = cfg.MinConnections
	poolCfg.MaxConnLifetime = cfg.MaxConnLifetime
	poolCfg.MaxConnIdleTime = cfg.MaxConnIdleTime
	poolCfg.HealthCheckPeriod = cfg.HealthCheckPeriod

	return poolCfg, nil
}

func NewPool(ctx context.Context, cfg DatabaseConfig) (*pgxpool.Pool, error) {
	poolCfg, err := buildPoolConfig(cfg)
	if err != nil {
		return nil, err
	}
	return pgxpool.NewWithConfig(ctx, poolCfg)
}
