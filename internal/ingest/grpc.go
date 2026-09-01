package ingest

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"runtime/debug"
	"time"

	"go.opentelemetry.io/collector/pdata/ptrace/ptraceotlp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/keepalive"
	"google.golang.org/grpc/status"
)

const maxSendBytes = 1 << 20

// GRPC is the OTLP/gRPC traces receiver. It writes the same schema as HTTP.
type GRPC struct {
	ptraceotlp.UnimplementedGRPCServer
	h   *Handler
	srv *grpc.Server
}

// NewGRPC builds a TraceService server. maxRecv is the max request size in bytes.
func NewGRPC(h *Handler, maxRecv int) (*GRPC, error) {
	if h == nil {
		return nil, errors.New("nil handler")
	}
	if maxRecv < 1 {
		maxRecv = 16 << 20
	}
	g := &GRPC{h: h}
	g.srv = grpc.NewServer(
		grpc.ChainUnaryInterceptor(recoverUnary(h.log)),
		grpc.MaxRecvMsgSize(maxRecv),
		grpc.MaxSendMsgSize(maxSendBytes),
		grpc.KeepaliveParams(keepalive.ServerParameters{
			MaxConnectionIdle: 15 * time.Minute,
			Time:              2 * time.Minute,
			Timeout:           20 * time.Second,
		}),
		grpc.KeepaliveEnforcementPolicy(keepalive.EnforcementPolicy{
			MinTime:             5 * time.Second,
			PermitWithoutStream: true,
		}),
	)
	ptraceotlp.RegisterGRPCServer(g.srv, g)
	return g, nil
}

// Export implements ptraceotlp.GRPCServer.
func (g *GRPC) Export(ctx context.Context, req ptraceotlp.ExportRequest) (ptraceotlp.ExportResponse, error) {
	if err := g.h.ExportTraces(ctx, req.Traces()); err != nil {
		g.h.log.Error("otlp grpc", "err", err)
		return ptraceotlp.ExportResponse{}, grpcStatus(err)
	}
	return ptraceotlp.NewExportResponse(), nil
}

// Serve accepts gRPC connections on ln until Shutdown or Stop.
func (g *GRPC) Serve(ln net.Listener) error {
	if g == nil || g.srv == nil {
		return errors.New("nil grpc server")
	}
	err := g.srv.Serve(ln)
	if err != nil && !errors.Is(err, grpc.ErrServerStopped) {
		return fmt.Errorf("grpc serve: %w", err)
	}
	return nil
}

// Shutdown drains in-flight RPCs, then stops. Stop is used if ctx expires.
func (g *GRPC) Shutdown(ctx context.Context) error {
	if g == nil || g.srv == nil {
		return nil
	}
	done := make(chan struct{})
	go func() {
		g.srv.GracefulStop()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		g.srv.Stop()
		<-done
		return ctx.Err()
	}
}

// Stop immediately closes the server (init failure before Serve).
func (g *GRPC) Stop() {
	if g == nil || g.srv == nil {
		return
	}
	g.srv.Stop()
}

func grpcStatus(err error) error {
	switch {
	case errors.Is(err, ErrTooManySpans):
		return status.Error(codes.ResourceExhausted, "too many spans")
	case errors.Is(err, errWrite):
		return status.Error(codes.Unavailable, "storage unavailable")
	default:
		return status.Error(codes.Internal, "internal error")
	}
}

func recoverUnary(log *slog.Logger) grpc.UnaryServerInterceptor {
	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			log.Error("grpc panic",
				"err", rec,
				"method", info.FullMethod,
				"stack", string(debug.Stack()),
			)
			resp = nil
			err = status.Error(codes.Internal, "internal error")
		}()
		return handler(ctx, req)
	}
}
