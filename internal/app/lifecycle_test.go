package app

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestUnitsRollbackClosesInReverse(t *testing.T) {
	t.Parallel()
	var closed []string
	units := []Unit{
		{
			Name:   "a",
			InitFn: func(context.Context, *App) error { return nil },
			CloseFn: func(_ context.Context, _ *App) error {
				closed = append(closed, "a")
				return nil
			},
		},
		{
			Name:   "b",
			InitFn: func(context.Context, *App) error { return nil },
			CloseFn: func(_ context.Context, _ *App) error {
				closed = append(closed, "b")
				return nil
			},
		},
		{
			Name: "c",
			InitFn: func(context.Context, *App) error {
				return errors.New("boom")
			},
			CloseFn: func(_ context.Context, _ *App) error {
				closed = append(closed, "c")
				return nil
			},
		},
	}
	_, err := InitApp(context.Background(), units, Options{
		Stdout: io.Discard,
		Getenv: func(string) string { return "" },
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if got := strings.Join(closed, ","); got != "b,a" {
		t.Fatalf("close order: got %s want b,a", got)
	}
}
