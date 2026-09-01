// Package config loads process configuration from the environment.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

const (
	defaultHTTPAddr         = ":8080"
	defaultGRPCAddr         = ":4317"
	defaultLogLevel         = "info"
	defaultLogFormat        = "json"
	defaultShutdownTimeout  = 15 * time.Second
	defaultHTTPMaxBodyBytes = 16 << 20
	defaultClickHouseAddr   = "127.0.0.1:9000"
	defaultClickHouseDB     = "rasat"
	defaultClickHouseUser   = "default"
	defaultCHPingTimeout    = 2 * time.Second
	defaultCHDialTimeout    = 5 * time.Second
	defaultCHMigrateTimeout = 30 * time.Second
	defaultCHInsertTimeout  = 30 * time.Second
	defaultCHQueryTimeout   = 10 * time.Second
	defaultQueryMaxWindow   = 168 * time.Hour
	defaultStreamBuffer     = 64
	defaultStreamMaxClients = 64
	defaultStreamWriteTO    = 10 * time.Second
	defaultStreamMaxPerSec  = 100

	readHeaderTimeout = 5 * time.Second
	readTimeout       = 30 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 120 * time.Second
)

// ErrInvalid is returned when environment configuration cannot be used.
var ErrInvalid = errors.New("invalid configuration")

// Config is process configuration. HTTP timeouts are fixed on purpose:
// ingest can raise them later via new env vars without changing the shape.
type Config struct {
	HTTPAddr                 string
	GRPCAddr                 string
	LogLevel                 string
	LogFormat                string
	ShutdownTimeout          time.Duration
	HTTPMaxBodyBytes         int64
	ReadHeaderTimeout        time.Duration
	ReadTimeout              time.Duration
	WriteTimeout             time.Duration
	IdleTimeout              time.Duration
	ClickHouseAddr           string
	ClickHouseDatabase       string
	ClickHouseUser           string
	ClickHousePassword       string
	ClickHousePingTimeout    time.Duration
	ClickHouseDialTimeout    time.Duration
	ClickHouseMigrateTimeout time.Duration
	ClickHouseInsertTimeout  time.Duration
	ClickHouseQueryTimeout   time.Duration
	QueryMaxWindow           time.Duration
	StreamBuffer             int
	StreamMaxClients         int
	StreamWriteTimeout       time.Duration
	StreamMaxPerSec          int
}

// Load reads RASAT_* from the process environment.
func Load() (Config, error) {
	return LoadFrom(os.Getenv)
}

// LoadFrom reads RASAT_* via getenv. Empty values mean defaults.
func LoadFrom(getenv func(string) string) (Config, error) {
	if getenv == nil {
		getenv = os.Getenv
	}

	cfg := Config{
		HTTPAddr:           envOr(getenv, "RASAT_HTTP_ADDR", defaultHTTPAddr),
		GRPCAddr:           envOr(getenv, "RASAT_GRPC_ADDR", defaultGRPCAddr),
		LogLevel:           strings.ToLower(envOr(getenv, "RASAT_LOG_LEVEL", defaultLogLevel)),
		LogFormat:          strings.ToLower(envOr(getenv, "RASAT_LOG_FORMAT", defaultLogFormat)),
		ReadHeaderTimeout:  readHeaderTimeout,
		ReadTimeout:        readTimeout,
		WriteTimeout:       writeTimeout,
		IdleTimeout:        idleTimeout,
		ClickHouseAddr:     envOr(getenv, "RASAT_CLICKHOUSE_ADDR", defaultClickHouseAddr),
		ClickHouseDatabase: envOr(getenv, "RASAT_CLICKHOUSE_DATABASE", defaultClickHouseDB),
		ClickHouseUser:     envOr(getenv, "RASAT_CLICKHOUSE_USER", defaultClickHouseUser),
		ClickHousePassword: getenv("RASAT_CLICKHOUSE_PASSWORD"),
	}

	shutdown, err := durationEnv(getenv, "RASAT_SHUTDOWN_TIMEOUT", defaultShutdownTimeout)
	if err != nil {
		return Config{}, err
	}
	cfg.ShutdownTimeout = shutdown

	maxBody, err := int64Env(getenv, "RASAT_HTTP_MAX_BODY", defaultHTTPMaxBodyBytes)
	if err != nil {
		return Config{}, err
	}
	cfg.HTTPMaxBodyBytes = maxBody

	pingTimeout, err := durationEnv(getenv, "RASAT_CLICKHOUSE_PING_TIMEOUT", defaultCHPingTimeout)
	if err != nil {
		return Config{}, err
	}
	cfg.ClickHousePingTimeout = pingTimeout

	dialTimeout, err := durationEnv(getenv, "RASAT_CLICKHOUSE_DIAL_TIMEOUT", defaultCHDialTimeout)
	if err != nil {
		return Config{}, err
	}
	cfg.ClickHouseDialTimeout = dialTimeout

	migrateTimeout, err := durationEnv(getenv, "RASAT_CLICKHOUSE_MIGRATE_TIMEOUT", defaultCHMigrateTimeout)
	if err != nil {
		return Config{}, err
	}
	cfg.ClickHouseMigrateTimeout = migrateTimeout

	insertTimeout, err := durationEnv(getenv, "RASAT_CLICKHOUSE_INSERT_TIMEOUT", defaultCHInsertTimeout)
	if err != nil {
		return Config{}, err
	}
	cfg.ClickHouseInsertTimeout = insertTimeout

	queryTimeout, err := durationEnv(getenv, "RASAT_CLICKHOUSE_QUERY_TIMEOUT", defaultCHQueryTimeout)
	if err != nil {
		return Config{}, err
	}
	cfg.ClickHouseQueryTimeout = queryTimeout

	maxWindow, err := durationEnv(getenv, "RASAT_QUERY_MAX_WINDOW", defaultQueryMaxWindow)
	if err != nil {
		return Config{}, err
	}
	cfg.QueryMaxWindow = maxWindow

	streamBuf, err := intEnv(getenv, "RASAT_STREAM_BUFFER", defaultStreamBuffer)
	if err != nil {
		return Config{}, err
	}
	cfg.StreamBuffer = streamBuf

	streamClients, err := intEnv(getenv, "RASAT_STREAM_MAX_CLIENTS", defaultStreamMaxClients)
	if err != nil {
		return Config{}, err
	}
	cfg.StreamMaxClients = streamClients

	streamWrite, err := durationEnv(getenv, "RASAT_STREAM_WRITE_TIMEOUT", defaultStreamWriteTO)
	if err != nil {
		return Config{}, err
	}
	cfg.StreamWriteTimeout = streamWrite

	streamMaxPerSec, err := intEnvAllowZero(getenv, "RASAT_STREAM_MAX_PER_SEC", defaultStreamMaxPerSec)
	if err != nil {
		return Config{}, err
	}
	cfg.StreamMaxPerSec = streamMaxPerSec

	if err := cfg.validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func (c Config) validate() error {
	switch c.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("%w: RASAT_LOG_LEVEL must be debug, info, warn, or error", ErrInvalid)
	}
	switch c.LogFormat {
	case "json", "text":
	default:
		return fmt.Errorf("%w: RASAT_LOG_FORMAT must be json or text", ErrInvalid)
	}
	if strings.TrimSpace(c.HTTPAddr) == "" {
		return fmt.Errorf("%w: RASAT_HTTP_ADDR must not be empty", ErrInvalid)
	}
	if c.HTTPMaxBodyBytes < 1 {
		return fmt.Errorf("%w: RASAT_HTTP_MAX_BODY must be > 0", ErrInvalid)
	}
	if c.ShutdownTimeout <= 0 {
		return fmt.Errorf("%w: RASAT_SHUTDOWN_TIMEOUT must be > 0", ErrInvalid)
	}
	if strings.TrimSpace(c.ClickHouseAddr) == "" {
		return fmt.Errorf("%w: RASAT_CLICKHOUSE_ADDR must not be empty", ErrInvalid)
	}
	if strings.TrimSpace(c.ClickHouseDatabase) == "" {
		return fmt.Errorf("%w: RASAT_CLICKHOUSE_DATABASE must not be empty", ErrInvalid)
	}
	if strings.TrimSpace(c.ClickHouseUser) == "" {
		return fmt.Errorf("%w: RASAT_CLICKHOUSE_USER must not be empty", ErrInvalid)
	}
	if c.ClickHousePingTimeout <= 0 {
		return fmt.Errorf("%w: RASAT_CLICKHOUSE_PING_TIMEOUT must be > 0", ErrInvalid)
	}
	if c.ClickHouseDialTimeout <= 0 {
		return fmt.Errorf("%w: RASAT_CLICKHOUSE_DIAL_TIMEOUT must be > 0", ErrInvalid)
	}
	if c.ClickHouseMigrateTimeout <= 0 {
		return fmt.Errorf("%w: RASAT_CLICKHOUSE_MIGRATE_TIMEOUT must be > 0", ErrInvalid)
	}
	if c.ClickHouseInsertTimeout <= 0 {
		return fmt.Errorf("%w: RASAT_CLICKHOUSE_INSERT_TIMEOUT must be > 0", ErrInvalid)
	}
	if c.ClickHouseQueryTimeout <= 0 {
		return fmt.Errorf("%w: RASAT_CLICKHOUSE_QUERY_TIMEOUT must be > 0", ErrInvalid)
	}
	if c.QueryMaxWindow <= 0 {
		return fmt.Errorf("%w: RASAT_QUERY_MAX_WINDOW must be > 0", ErrInvalid)
	}
	if c.StreamBuffer < 1 {
		return fmt.Errorf("%w: RASAT_STREAM_BUFFER must be > 0", ErrInvalid)
	}
	if c.StreamMaxClients < 1 {
		return fmt.Errorf("%w: RASAT_STREAM_MAX_CLIENTS must be > 0", ErrInvalid)
	}
	if c.StreamWriteTimeout <= 0 {
		return fmt.Errorf("%w: RASAT_STREAM_WRITE_TIMEOUT must be > 0", ErrInvalid)
	}
	return nil
}

func envOr(getenv func(string) string, key, fallback string) string {
	if v := strings.TrimSpace(getenv(key)); v != "" {
		return v
	}
	return fallback
}

func durationEnv(getenv func(string) string, key string, fallback time.Duration) (time.Duration, error) {
	raw := strings.TrimSpace(getenv(key))
	if raw == "" {
		return fallback, nil
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		return 0, fmt.Errorf("%w: parse %s: %w", ErrInvalid, key, err)
	}
	if d <= 0 {
		return 0, fmt.Errorf("%w: %s must be > 0", ErrInvalid, key)
	}
	return d, nil
}

func int64Env(getenv func(string) string, key string, fallback int64) (int64, error) {
	raw := strings.TrimSpace(getenv(key))
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: parse %s: %w", ErrInvalid, key, err)
	}
	return n, nil
}

func intEnv(getenv func(string) string, key string, fallback int) (int, error) {
	n, err := int64Env(getenv, key, int64(fallback))
	if err != nil {
		return 0, err
	}
	if n < 1 || n > 1_000_000 {
		return 0, fmt.Errorf("%w: %s must be 1..1000000", ErrInvalid, key)
	}
	return int(n), nil
}

func intEnvAllowZero(getenv func(string) string, key string, fallback int) (int, error) {
	raw := strings.TrimSpace(getenv(key))
	if raw == "" {
		return fallback, nil
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: parse %s: %w", ErrInvalid, key, err)
	}
	if n < 0 || n > 1_000_000 {
		return 0, fmt.Errorf("%w: %s must be 0..1000000", ErrInvalid, key)
	}
	return int(n), nil
}
