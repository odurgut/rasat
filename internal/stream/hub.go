package stream

import (
	"errors"
	"log/slog"
	"sync"
	"time"

	"github.com/odurgut/rasat/internal/store"
)

// ErrTooManyClients is returned when Subscribe would exceed MaxClients.
var ErrTooManyClients = errors.New("too many stream clients")

// ErrClosed is returned when Subscribe is called after Close.
var ErrClosed = errors.New("stream hub closed")

const (
	defaultBuffer       = 64
	defaultMaxClients   = 64
	defaultWriteTimeout = 10 * time.Second
)

// HubConfig bounds the in-process fan-out. Publish never waits on clients.
type HubConfig struct {
	Buffer       int
	MaxClients   int
	WriteTimeout time.Duration
}

// Hub fans events to WebSocket subscribers.
// A full per-client buffer drops that client; ingest is not delayed.
type Hub[T any] struct {
	log          *slog.Logger
	buffer       int
	maxClients   int
	writeTimeout time.Duration

	mu     sync.Mutex
	subs   map[*Sub[T]]struct{}
	closed bool
}

// TraceHub is the live trace list stream.
type TraceHub = Hub[store.TraceListRow]

// LogHub is the live log stream.
type LogHub = Hub[store.LogRow]

// Sub is one subscriber. Close unsubscribes; C closes when dropped or hub stops.
type Sub[T any] struct {
	ch        chan T
	hub       *Hub[T]
	closeOnce sync.Once
}

// NewHub returns a fan-out hub. log is required.
func NewHub[T any](log *slog.Logger, cfg HubConfig) (*Hub[T], error) {
	if log == nil {
		return nil, errors.New("nil logger")
	}
	if cfg.Buffer <= 0 {
		cfg.Buffer = defaultBuffer
	}
	if cfg.MaxClients <= 0 {
		cfg.MaxClients = defaultMaxClients
	}
	if cfg.WriteTimeout <= 0 {
		cfg.WriteTimeout = defaultWriteTimeout
	}
	return &Hub[T]{
		log:          log,
		buffer:       cfg.Buffer,
		maxClients:   cfg.MaxClients,
		writeTimeout: cfg.WriteTimeout,
		subs:         make(map[*Sub[T]]struct{}),
	}, nil
}

// Subscribe registers a buffered client. The caller must Close the Sub.
func (h *Hub[T]) Subscribe() (*Sub[T], error) {
	if h == nil {
		return nil, ErrClosed
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, ErrClosed
	}
	if len(h.subs) >= h.maxClients {
		return nil, ErrTooManyClients
	}
	s := &Sub[T]{
		ch:  make(chan T, h.buffer),
		hub: h,
	}
	h.subs[s] = struct{}{}
	return s, nil
}

// C is the event stream. It is closed when the subscriber is dropped or closed.
func (s *Sub[T]) C() <-chan T {
	if s == nil {
		return nil
	}
	return s.ch
}

// Close unsubscribes. Safe to call more than once.
func (s *Sub[T]) Close() {
	if s == nil || s.hub == nil {
		return
	}
	s.hub.drop(s)
}

// Publish sends one event. Slow clients are dropped; this never blocks.
func (h *Hub[T]) Publish(row T) {
	if h == nil {
		return
	}
	h.mu.Lock()
	if h.closed {
		h.mu.Unlock()
		return
	}
	full := make([]*Sub[T], 0)
	for s := range h.subs {
		select {
		case s.ch <- row:
		default:
			full = append(full, s)
		}
	}
	h.mu.Unlock()
	for _, s := range full {
		h.log.Warn("stream drop slow client")
		h.drop(s)
	}
}

// Close unsubscribes every client. Safe to call more than once.
func (h *Hub[T]) Close() {
	if h == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for s := range h.subs {
		s.closeOnce.Do(func() { close(s.ch) })
	}
	h.subs = nil
}

func (h *Hub[T]) drop(s *Sub[T]) {
	if h == nil || s == nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subs == nil {
		return
	}
	if _, ok := h.subs[s]; !ok {
		return
	}
	delete(h.subs, s)
	s.closeOnce.Do(func() { close(s.ch) })
}
