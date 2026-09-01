// Package logging constructs the process slog logger.
package logging

import (
	"fmt"
	"io"
	"log/slog"
)

// New returns a slog logger. format is json or text; level is a slog level name.
func New(w io.Writer, format, level string) (*slog.Logger, error) {
	if w == nil {
		return nil, fmt.Errorf("nil writer")
	}
	var lvl slog.Level
	if err := lvl.UnmarshalText([]byte(level)); err != nil {
		return nil, fmt.Errorf("log level %q: %w", level, err)
	}
	opts := &slog.HandlerOptions{
		Level:     lvl,
		AddSource: lvl <= slog.LevelDebug,
	}
	var h slog.Handler
	switch format {
	case "text":
		h = slog.NewTextHandler(w, opts)
	case "json":
		h = slog.NewJSONHandler(w, opts)
	default:
		return nil, fmt.Errorf("log format %q", format)
	}
	return slog.New(h), nil
}
