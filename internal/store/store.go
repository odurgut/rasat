// Package store is the ClickHouse client and schema.
//
// HTTP handlers in httpapi must not import this package; they depend on
// injected interfaces. Ingest writes through TraceWriter. Logs write through
// LogWriter. Search, get, the service catalog, operations, the service map,
// derived metrics, and error causes go through query.Searcher (Store implements it).
package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/ClickHouse/clickhouse-go/v2"
	"github.com/odurgut/rasat/internal/config"
)

// ErrNotReady is returned when ClickHouse does not answer a ping in time.
var ErrNotReady = errors.New("clickhouse not ready")

// ErrNotFound is returned when a trace id has no spans in the requested window.
var ErrNotFound = errors.New("not found")

// ErrTraceTooLarge is returned when a trace exceeds the span or child row cap.
var ErrTraceTooLarge = errors.New("trace too large")

type chConn interface {
	Ping(context.Context) error
	Exec(ctx context.Context, query string, args ...any) error
	Select(ctx context.Context, dest any, query string, args ...any) error
	Close() error
}

// rowBatch is the subset of clickhouse-go Batch used for ingest.
type rowBatch interface {
	Append(v ...any) error
	Send() error
	Abort() error
}

// batchFactory prepares native protocol INSERT batches.
type batchFactory interface {
	PrepareBatch(ctx context.Context, query string) (rowBatch, error)
}

type nativeBatches struct {
	conn clickhouse.Conn
}

func (n nativeBatches) PrepareBatch(ctx context.Context, query string) (rowBatch, error) {
	return n.conn.PrepareBatch(ctx, query)
}

// Store is a bounded ClickHouse connection pool.
type Store struct {
	conn           chConn
	batches        batchFactory
	log            *slog.Logger
	database       string
	pingTimeout    time.Duration
	migrateTimeout time.Duration
	insertTimeout  time.Duration
	queryTimeout   time.Duration
}

// New opens a pool. It does not ping; readiness is /ready. Call Migrate after New.
func New(cfg config.Config, log *slog.Logger) (*Store, error) {
	if log == nil {
		return nil, errors.New("nil logger")
	}
	if cfg.ClickHouseAddr == "" {
		return nil, fmt.Errorf("%w: clickhouse addr is empty", config.ErrInvalid)
	}

	conn, err := clickhouse.Open(&clickhouse.Options{
		Addr: []string{cfg.ClickHouseAddr},
		Auth: clickhouse.Auth{
			// Always land on default so CREATE DATABASE rasat works on first boot.
			// All DDL/DML uses fully-qualified names in cfg.ClickHouseDatabase.
			Database: "default",
			Username: cfg.ClickHouseUser,
			Password: cfg.ClickHousePassword,
		},
		DialTimeout:     cfg.ClickHouseDialTimeout,
		MaxOpenConns:    16,
		MaxIdleConns:    8,
		ConnMaxLifetime: time.Hour,
		Compression: &clickhouse.Compression{
			Method: clickhouse.CompressionLZ4,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("clickhouse open: %w", err)
	}

	log.Info("clickhouse configured",
		"addr", cfg.ClickHouseAddr,
		"database", cfg.ClickHouseDatabase,
		"user", cfg.ClickHouseUser,
	)
	return &Store{
		conn:           conn,
		batches:        nativeBatches{conn: conn},
		log:            log,
		database:       cfg.ClickHouseDatabase,
		pingTimeout:    cfg.ClickHousePingTimeout,
		migrateTimeout: cfg.ClickHouseMigrateTimeout,
		insertTimeout:  cfg.ClickHouseInsertTimeout,
		queryTimeout:   cfg.ClickHouseQueryTimeout,
	}, nil
}

// Ready pings ClickHouse with a timeout derived from ctx and config.
func (s *Store) Ready(ctx context.Context) error {
	if s == nil || s.conn == nil {
		return ErrNotReady
	}
	ctx, cancel := context.WithTimeout(ctx, s.pingTimeout)
	defer cancel()
	if err := s.conn.Ping(ctx); err != nil {
		return fmt.Errorf("%w: %w", ErrNotReady, err)
	}
	return nil
}

// Migrate applies idempotent DDL (CREATE DATABASE / TABLE / VIEW IF NOT EXISTS).
func (s *Store) Migrate(ctx context.Context) error {
	if s == nil || s.conn == nil {
		return ErrNotReady
	}
	stmts, err := Statements(s.database)
	if err != nil {
		return err
	}
	timeout := s.migrateTimeout
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	for i, q := range stmts {
		if err := s.conn.Exec(ctx, q); err != nil {
			return fmt.Errorf("migrate statement %d: %w", i, err)
		}
	}
	if s.log != nil {
		s.log.Info("clickhouse schema ready", "database", s.database, "statements", len(stmts))
	}
	return nil
}

// Close releases pool connections.
func (s *Store) Close() error {
	if s == nil || s.conn == nil {
		return nil
	}
	if err := s.conn.Close(); err != nil {
		return fmt.Errorf("clickhouse close: %w", err)
	}
	return nil
}
