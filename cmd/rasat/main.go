// Command rasat is the Rasat observability process.
package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/odurgut/rasat/internal/app"
)

func main() {
	a, err := app.InitApp(context.Background(), []app.Unit{
		{Name: "config", InitFn: app.InitConfig},
		{Name: "logger", InitFn: app.InitLogger},
		{Name: "store", InitFn: app.InitStore, CloseFn: app.CloseStore},
		{Name: "stream", InitFn: app.InitStream, CloseFn: app.CloseStream},
		{Name: "otlp", InitFn: app.InitOTLP, CloseFn: app.CloseOTLP},
		{Name: "query", InitFn: app.InitQuery},
		{Name: "logs", InitFn: app.InitLogs},
		{Name: "http", InitFn: app.InitHTTP, CloseFn: app.CloseHTTP},
	}, app.Options{})
	if err != nil {
		slog.Error("init", "err", err)
		os.Exit(1)
	}

	go a.WatchShutdown()
	if err := a.Wait(); err != nil {
		slog.Error("exit", "err", err)
		os.Exit(1)
	}
}
