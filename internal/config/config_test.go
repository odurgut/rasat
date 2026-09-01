package config

import (
	"errors"
	"testing"
	"time"
)

func getenvMap(m map[string]string) func(string) string {
	return func(key string) string {
		return m[key]
	}
}

func TestLoadFromDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := LoadFrom(getenvMap(nil))
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if cfg.HTTPAddr != defaultHTTPAddr {
		t.Fatalf("HTTPAddr: got %q", cfg.HTTPAddr)
	}
	if cfg.GRPCAddr != defaultGRPCAddr {
		t.Fatalf("GRPCAddr: got %q", cfg.GRPCAddr)
	}
	if cfg.LogLevel != defaultLogLevel || cfg.LogFormat != defaultLogFormat {
		t.Fatalf("log: %s %s", cfg.LogLevel, cfg.LogFormat)
	}
	if cfg.ShutdownTimeout != defaultShutdownTimeout {
		t.Fatalf("ShutdownTimeout: got %s", cfg.ShutdownTimeout)
	}
	if cfg.HTTPMaxBodyBytes != defaultHTTPMaxBodyBytes {
		t.Fatalf("HTTPMaxBodyBytes: got %d", cfg.HTTPMaxBodyBytes)
	}
	if cfg.ReadHeaderTimeout != readHeaderTimeout {
		t.Fatal("ReadHeaderTimeout must be set")
	}
	if cfg.ClickHouseAddr != defaultClickHouseAddr {
		t.Fatalf("ClickHouseAddr: %q", cfg.ClickHouseAddr)
	}
	if cfg.ClickHouseDatabase != defaultClickHouseDB {
		t.Fatalf("ClickHouseDatabase: %q", cfg.ClickHouseDatabase)
	}
	if cfg.ClickHousePingTimeout != defaultCHPingTimeout {
		t.Fatalf("ClickHousePingTimeout: %s", cfg.ClickHousePingTimeout)
	}
	if cfg.ClickHouseMigrateTimeout != defaultCHMigrateTimeout {
		t.Fatalf("ClickHouseMigrateTimeout: %s", cfg.ClickHouseMigrateTimeout)
	}
	if cfg.ClickHouseInsertTimeout != defaultCHInsertTimeout {
		t.Fatalf("ClickHouseInsertTimeout: %s", cfg.ClickHouseInsertTimeout)
	}
	if cfg.ClickHouseQueryTimeout != defaultCHQueryTimeout {
		t.Fatalf("ClickHouseQueryTimeout: %s", cfg.ClickHouseQueryTimeout)
	}
	if cfg.QueryMaxWindow != defaultQueryMaxWindow {
		t.Fatalf("QueryMaxWindow: %s", cfg.QueryMaxWindow)
	}
	if cfg.StreamBuffer != defaultStreamBuffer {
		t.Fatalf("StreamBuffer: %d", cfg.StreamBuffer)
	}
	if cfg.StreamMaxClients != defaultStreamMaxClients {
		t.Fatalf("StreamMaxClients: %d", cfg.StreamMaxClients)
	}
	if cfg.StreamWriteTimeout != defaultStreamWriteTO {
		t.Fatalf("StreamWriteTimeout: %s", cfg.StreamWriteTimeout)
	}
	if cfg.StreamMaxPerSec != defaultStreamMaxPerSec {
		t.Fatalf("StreamMaxPerSec: %d", cfg.StreamMaxPerSec)
	}
}

func TestLoadFromOverrides(t *testing.T) {
	t.Parallel()

	cfg, err := LoadFrom(getenvMap(map[string]string{
		"RASAT_HTTP_ADDR":                  "127.0.0.1:9",
		"RASAT_GRPC_ADDR":                  "127.0.0.1:4317",
		"RASAT_LOG_LEVEL":                  "DEBUG",
		"RASAT_LOG_FORMAT":                 "TEXT",
		"RASAT_SHUTDOWN_TIMEOUT":           "3s",
		"RASAT_HTTP_MAX_BODY":              "1024",
		"RASAT_CLICKHOUSE_ADDR":            "ch:9000",
		"RASAT_CLICKHOUSE_DATABASE":        "rasat",
		"RASAT_CLICKHOUSE_USER":            "rasat",
		"RASAT_CLICKHOUSE_PASSWORD":        "secret",
		"RASAT_CLICKHOUSE_PING_TIMEOUT":    "1s",
		"RASAT_CLICKHOUSE_DIAL_TIMEOUT":    "4s",
		"RASAT_CLICKHOUSE_MIGRATE_TIMEOUT": "9s",
		"RASAT_CLICKHOUSE_INSERT_TIMEOUT":  "7s",
		"RASAT_CLICKHOUSE_QUERY_TIMEOUT":   "4s",
		"RASAT_QUERY_MAX_WINDOW":           "24h",
		"RASAT_STREAM_BUFFER":              "8",
		"RASAT_STREAM_MAX_CLIENTS":         "16",
		"RASAT_STREAM_WRITE_TIMEOUT":       "2s",
		"RASAT_STREAM_MAX_PER_SEC":         "0",
	}))
	if err != nil {
		t.Fatalf("LoadFrom: %v", err)
	}
	if cfg.HTTPAddr != "127.0.0.1:9" {
		t.Fatalf("HTTPAddr: %q", cfg.HTTPAddr)
	}
	if cfg.GRPCAddr != "127.0.0.1:4317" {
		t.Fatalf("GRPCAddr: %q", cfg.GRPCAddr)
	}
	if cfg.LogLevel != "debug" || cfg.LogFormat != "text" {
		t.Fatalf("log: %s %s", cfg.LogLevel, cfg.LogFormat)
	}
	if cfg.ShutdownTimeout != 3*time.Second {
		t.Fatalf("ShutdownTimeout: %s", cfg.ShutdownTimeout)
	}
	if cfg.HTTPMaxBodyBytes != 1024 {
		t.Fatalf("HTTPMaxBodyBytes: %d", cfg.HTTPMaxBodyBytes)
	}
	if cfg.ClickHouseAddr != "ch:9000" || cfg.ClickHouseDatabase != "rasat" {
		t.Fatalf("clickhouse: %s %s", cfg.ClickHouseAddr, cfg.ClickHouseDatabase)
	}
	if cfg.ClickHouseUser != "rasat" || cfg.ClickHousePassword != "secret" {
		t.Fatal("clickhouse auth")
	}
	if cfg.ClickHousePingTimeout != time.Second || cfg.ClickHouseDialTimeout != 4*time.Second {
		t.Fatalf("clickhouse timeouts: %s %s", cfg.ClickHousePingTimeout, cfg.ClickHouseDialTimeout)
	}
	if cfg.ClickHouseMigrateTimeout != 9*time.Second {
		t.Fatalf("migrate timeout: %s", cfg.ClickHouseMigrateTimeout)
	}
	if cfg.ClickHouseInsertTimeout != 7*time.Second {
		t.Fatalf("insert timeout: %s", cfg.ClickHouseInsertTimeout)
	}
	if cfg.ClickHouseQueryTimeout != 4*time.Second {
		t.Fatalf("query timeout: %s", cfg.ClickHouseQueryTimeout)
	}
	if cfg.QueryMaxWindow != 24*time.Hour {
		t.Fatalf("query window: %s", cfg.QueryMaxWindow)
	}
	if cfg.StreamBuffer != 8 || cfg.StreamMaxClients != 16 {
		t.Fatalf("stream bounds: %d %d", cfg.StreamBuffer, cfg.StreamMaxClients)
	}
	if cfg.StreamWriteTimeout != 2*time.Second {
		t.Fatalf("stream write: %s", cfg.StreamWriteTimeout)
	}
	if cfg.StreamMaxPerSec != 0 {
		t.Fatalf("stream max per sec: %d", cfg.StreamMaxPerSec)
	}
}

func TestLoadFromErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		env  map[string]string
	}{
		{name: "bad level", env: map[string]string{"RASAT_LOG_LEVEL": "verbose"}},
		{name: "bad format", env: map[string]string{"RASAT_LOG_FORMAT": "xml"}},
		{name: "zero body", env: map[string]string{"RASAT_HTTP_MAX_BODY": "0"}},
		{name: "bad body", env: map[string]string{"RASAT_HTTP_MAX_BODY": "nope"}},
		{name: "zero shutdown", env: map[string]string{"RASAT_SHUTDOWN_TIMEOUT": "0s"}},
		{name: "bad shutdown", env: map[string]string{"RASAT_SHUTDOWN_TIMEOUT": "forever"}},
		{name: "zero ping", env: map[string]string{"RASAT_CLICKHOUSE_PING_TIMEOUT": "0s"}},
		{name: "bad ping", env: map[string]string{"RASAT_CLICKHOUSE_PING_TIMEOUT": "nope"}},
		{name: "zero migrate", env: map[string]string{"RASAT_CLICKHOUSE_MIGRATE_TIMEOUT": "0s"}},
		{name: "zero insert", env: map[string]string{"RASAT_CLICKHOUSE_INSERT_TIMEOUT": "0s"}},
		{name: "zero query", env: map[string]string{"RASAT_CLICKHOUSE_QUERY_TIMEOUT": "0s"}},
		{name: "zero window", env: map[string]string{"RASAT_QUERY_MAX_WINDOW": "0s"}},
		{name: "zero stream buffer", env: map[string]string{"RASAT_STREAM_BUFFER": "0"}},
		{name: "bad stream buffer", env: map[string]string{"RASAT_STREAM_BUFFER": "nope"}},
		{name: "zero stream clients", env: map[string]string{"RASAT_STREAM_MAX_CLIENTS": "0"}},
		{name: "zero stream write", env: map[string]string{"RASAT_STREAM_WRITE_TIMEOUT": "0s"}},
		{name: "bad stream max per sec", env: map[string]string{"RASAT_STREAM_MAX_PER_SEC": "-1"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := LoadFrom(getenvMap(tt.env))
			if err == nil {
				t.Fatal("expected error")
			}
			if !errors.Is(err, ErrInvalid) {
				t.Fatalf("want ErrInvalid, got %v", err)
			}
		})
	}
}
