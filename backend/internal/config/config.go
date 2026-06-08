package config

import (
	"os"
	"strconv"
	"strings"
)

const (
	defaultHost        = "0.0.0.0"
	defaultPort        = "8080"
	defaultAPIPrefix   = "/api/v1"
	defaultAppName     = "MVP Push Gateway"
	defaultEnvironment = "development"
	defaultNATSURL     = "nats://127.0.0.1:4222"
)

type Config struct {
	App      AppConfig
	Server   ServerConfig
	Postgres PostgresConfig
	Queue    QueueConfig
}

type AppConfig struct {
	Name        string
	Environment string
}

type ServerConfig struct {
	Host           string
	Port           string
	APIPrefix      string
	TrustedProxies []string
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

type QueueConfig struct {
	Backend      string
	NATS         NATSConfig
	ResultWriter ResultWriterConfig
}

type NATSConfig struct {
	URL                   string
	CredsPath             string
	StreamReplicas        int
	RouteConsumers        int
	SendConsumers         int
	ResultConsumers       int
	LatestPayloadKVBucket string
	InboundDedupeKVPrefix string
	HMACNonceKVPrefix     string
}

type ResultWriterConfig struct {
	BatchSize       int
	FlushIntervalMS int
}

func Load() Config {
	return Config{
		App: AppConfig{
			Name:        envOrDefault("MGP_APP_NAME", defaultAppName),
			Environment: envOrDefault("MGP_ENVIRONMENT", defaultEnvironment),
		},
		Server: ServerConfig{
			Host:           envOrDefault("MGP_HOST", defaultHost),
			Port:           envOrDefault("MGP_PORT", defaultPort),
			APIPrefix:      normalizePrefix(envOrDefault("MGP_API_PREFIX", defaultAPIPrefix)),
			TrustedProxies: parseCSV(os.Getenv("MGP_TRUSTED_PROXIES")),
		},
		Postgres: PostgresConfig{
			DSN:             os.Getenv("MGP_POSTGRES_DSN"),
			APIPool:         PoolConfig{MaxConns: envInt32OrDefault("MGP_POSTGRES_API_MAX_CONNS", 60), MinConns: 1},
			PlanningPool:    PoolConfig{MaxConns: envInt32OrDefault("MGP_POSTGRES_PLANNING_MAX_CONNS", 20), MinConns: 1},
			SendingPool:     PoolConfig{MaxConns: envInt32OrDefault("MGP_POSTGRES_SENDING_MAX_CONNS", 20), MinConns: 1},
			MaintenancePool: PoolConfig{MaxConns: envInt32OrDefault("MGP_POSTGRES_MAINTENANCE_MAX_CONNS", 6), MinConns: 1},
		},
		Queue: QueueConfig{
			Backend: normalizeQueueBackend(envOrDefault("MGP_QUEUE_BACKEND", "jetstream")),
			NATS: NATSConfig{
				URL:                   envOrDefault("MGP_NATS_URL", defaultNATSURL),
				CredsPath:             strings.TrimSpace(os.Getenv("MGP_NATS_CREDS")),
				StreamReplicas:        envIntOrDefault("MGP_NATS_STREAM_REPLICAS", 1),
				RouteConsumers:        envIntOrDefault("MGP_NATS_ROUTE_CONSUMERS", 20),
				SendConsumers:         envIntOrDefault("MGP_NATS_SEND_CONSUMERS", 20),
				ResultConsumers:       envIntOrDefault("MGP_NATS_RESULT_CONSUMERS", 10),
				LatestPayloadKVBucket: strings.TrimSpace(os.Getenv("MGP_NATS_LATEST_PAYLOAD_KV_BUCKET")),
				InboundDedupeKVPrefix: strings.TrimSpace(os.Getenv("MGP_NATS_INBOUND_DEDUPE_KV_PREFIX")),
				HMACNonceKVPrefix:     strings.TrimSpace(os.Getenv("MGP_NATS_HMAC_NONCE_KV_PREFIX")),
			},
			ResultWriter: ResultWriterConfig{
				BatchSize:       envIntOrDefault("MGP_RESULT_WRITER_BATCH_SIZE", 500),
				FlushIntervalMS: envIntOrDefault("MGP_RESULT_WRITER_FLUSH_INTERVAL_MS", 50),
			},
		},
	}
}

func parseCSV(value string) []string {
	parts := strings.FieldsFunc(value, func(char rune) bool {
		return char == ',' || char == '\n' || char == '\r' || char == '\t' || char == ' '
	})
	cleaned := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			cleaned = append(cleaned, part)
		}
	}
	return cleaned
}

func envOrDefault(key string, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envInt32OrDefault(key string, fallback int32) int32 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return int32(parsed)
}

func envIntOrDefault(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
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

func normalizeQueueBackend(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "jetstream", "nats", "nats-jetstream":
		return "jetstream"
	default:
		return "postgres"
	}
}
