package config

import (
	"os"
	"strings"
)

const (
	defaultHost        = "0.0.0.0"
	defaultPort        = "8080"
	defaultAPIPrefix   = "/api/v1"
	defaultAppName     = "MVP Push Gateway"
	defaultEnvironment = "development"
)

type Config struct {
	App      AppConfig
	Server   ServerConfig
	Postgres PostgresConfig
}

type AppConfig struct {
	Name        string
	Environment string
}

type ServerConfig struct {
	Host      string
	Port      string
	APIPrefix string
}

type PostgresConfig struct {
	DSN             string
	APIPool         PoolConfig
	PlanningPool    PoolConfig
	SendingPool     PoolConfig
	MaintenancePool PoolConfig
}

type PoolConfig struct {
	MaxConns int32
	MinConns int32
}

func Load() Config {
	return Config{
		App: AppConfig{
			Name:        envOrDefault("MGP_APP_NAME", defaultAppName),
			Environment: envOrDefault("MGP_ENVIRONMENT", defaultEnvironment),
		},
		Server: ServerConfig{
			Host:      envOrDefault("MGP_HOST", defaultHost),
			Port:      envOrDefault("MGP_PORT", defaultPort),
			APIPrefix: normalizePrefix(envOrDefault("MGP_API_PREFIX", defaultAPIPrefix)),
		},
		Postgres: PostgresConfig{
			DSN: os.Getenv("MGP_POSTGRES_DSN"),
			// Step 1 only defines the pool shape. Real PostgreSQL connections start in the data layer step.
			APIPool:         PoolConfig{MaxConns: 10, MinConns: 1},
			PlanningPool:    PoolConfig{MaxConns: 5, MinConns: 1},
			SendingPool:     PoolConfig{MaxConns: 10, MinConns: 1},
			MaintenancePool: PoolConfig{MaxConns: 3, MinConns: 1},
		},
	}
}

func envOrDefault(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func normalizePrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" || prefix == "/" {
		return defaultAPIPrefix
	}
	if !strings.HasPrefix(prefix, "/") {
		prefix = "/" + prefix
	}
	return strings.TrimRight(prefix, "/")
}
