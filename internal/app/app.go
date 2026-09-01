// Package app wires process dependencies.
//
// cmd/rasat lists Units (InitFn / CloseFn). InitApp runs them in order and
// keeps CloseFns for reverse shutdown. Wait blocks on servers; WatchShutdown
// stops them on SIGINT/SIGTERM. No package-level mutable state.
package app

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/odurgut/rasat/internal/httpapi"
	"github.com/odurgut/rasat/internal/query"
	"github.com/odurgut/rasat/internal/store"
)

const fallbackShutdown = 15 * time.Second

// Options control how InitApp binds and logs. Tests inject Listener and Ready.
type Options struct {
	Getenv        func(string) string
	Stdout        io.Writer
	Listener      net.Listener
	GRPCListener  net.Listener
	Ready         httpapi.ReadyChecker
	ReadyCloser   io.Closer
	TraceWriter   store.TraceWriter
	LogWriter     store.LogWriter
	TraceSearcher query.Searcher
	Units         []Unit
}

// Run inits the default or provided units, stops when ctx is done, then Wait.
func Run(ctx context.Context, opt Options) error {
	units := opt.Units
	if len(units) == 0 {
		units = DefaultUnits()
	}
	a, err := InitApp(ctx, units, opt)
	if err != nil {
		return err
	}
	go func() {
		<-ctx.Done()
		a.Stop()
	}()
	return a.Wait()
}

// WatchShutdown stops the process on SIGINT/SIGTERM. Call from a goroutine
// before Wait, as in cmd/rasat.
func (a *App) WatchShutdown() {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	<-ctx.Done()
	if a.Log != nil {
		a.Log.Info("shutdown signal")
	}
	a.Stop()
}

// Stop unblocks Wait so Close can run. Safe to call more than once.
func (a *App) Stop() {
	if a == nil {
		return
	}
	a.stopOnce.Do(func() {
		close(a.stopCh)
	})
}

// Wait blocks until Stop or a server error, then closes units in reverse.
func (a *App) Wait() error {
	if a == nil {
		return nil
	}
	a.waitOnce.Do(func() {
		select {
		case err := <-a.serveErr:
			a.serveTaken = true
			a.waitServeErr = err
		case <-a.stopCh:
		}
		stopUnitsBounded(context.Background(), a)
		a.waitServeErr = drainServeErr(a, a.waitServeErr)
		if a.Log != nil {
			a.Log.Info("stopped")
		}
	})
	if a.waitServeErr != nil {
		return fmt.Errorf("serve: %w", a.waitServeErr)
	}
	return nil
}

func stopUnitsBounded(parent context.Context, a *App) {
	d := fallbackShutdown
	if a != nil && a.Cfg.ShutdownTimeout > 0 {
		d = a.Cfg.ShutdownTimeout
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(parent), d)
	defer cancel()
	a.Close(ctx)
}

func drainServeErr(a *App, already error) error {
	n := a.serveCount
	if a.serveTaken {
		n--
	}
	if n < 0 {
		n = 0
	}
	err := already
	for i := 0; i < n; i++ {
		se := <-a.serveErr
		if se != nil && err == nil {
			err = se
		}
	}
	return err
}

func getenvOr(opt Options) func(string) string {
	if opt.Getenv != nil {
		return opt.Getenv
	}
	return os.Getenv
}

func stdoutOr(opt Options) io.Writer {
	if opt.Stdout != nil {
		return opt.Stdout
	}
	return os.Stdout
}
