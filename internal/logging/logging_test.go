package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
)

func TestNewJSON(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	log, err := New(&buf, "json", "info")
	if err != nil {
		t.Fatal(err)
	}
	log.Info("hello", "k", "v")
	out := buf.String()
	if !strings.Contains(out, "hello") {
		t.Fatalf("unexpected log: %s", out)
	}
}

func TestNewRejects(t *testing.T) {
	t.Parallel()
	if _, err := New(nil, "json", "info"); err == nil {
		t.Fatal("expected nil writer error")
	}
	var buf bytes.Buffer
	if _, err := New(&buf, "xml", "info"); err == nil {
		t.Fatal("expected format error")
	}
}

func TestDebugAddsSource(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	log, err := New(&buf, "text", "debug")
	if err != nil {
		t.Fatal(err)
	}
	log.Log(context.Background(), slog.LevelDebug, "x")
	if buf.Len() == 0 {
		t.Fatal("expected output")
	}
}
