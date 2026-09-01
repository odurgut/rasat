package app

import (
	"context"
	"io"
	"strings"
	"testing"
)

func TestDefaultUnitsOrder(t *testing.T) {
	t.Parallel()
	names := make([]string, 0, 8)
	for _, u := range DefaultUnits() {
		names = append(names, u.Name)
	}
	got := strings.Join(names, ",")
	want := "config,logger,store,stream,otlp,query,logs,http"
	if got != want {
		t.Fatalf("units %s want %s", got, want)
	}
	var streamI, otlpI int
	for i, n := range names {
		switch n {
		case "stream":
			streamI = i
		case "otlp":
			otlpI = i
		}
	}
	if streamI >= otlpI {
		t.Fatal("stream must init before otlp")
	}
}

func TestInitStream(t *testing.T) {
	t.Parallel()
	a, err := InitApp(context.Background(), []Unit{UnitConfig, UnitLogger, UnitStream}, Options{
		Stdout: io.Discard,
		Getenv: func(string) string { return "" },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close(context.Background())
	if a.Stream == nil {
		t.Fatal("nil hub")
	}
	if a.LogStream == nil {
		t.Fatal("nil log hub")
	}
}

func TestInitLogs(t *testing.T) {
	t.Parallel()
	a, err := InitApp(context.Background(), []Unit{UnitConfig, UnitLogger, UnitLogs}, Options{
		Stdout: io.Discard,
		Getenv: func(string) string { return "" },
	})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close(context.Background())
	if a.Logs == nil {
		t.Fatal("nil logs handler")
	}
}
