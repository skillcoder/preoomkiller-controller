package httpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/skillcoder/preoomkiller-controller/internal/infra/shutdown"
)

const defaultMetricsPort = "9090"

// MetricsServer serves Prometheus metrics on a dedicated port.
type MetricsServer struct {
	logger     *slog.Logger
	port       string
	server     *http.Server
	ready      chan struct{}
	inShutdown atomic.Bool
}

// NewMetricsServer creates a new metrics server that serves GET /metrics on the given port.
func NewMetricsServer(logger *slog.Logger, port string) *MetricsServer {
	if port == "" {
		port = defaultMetricsPort
	}

	return &MetricsServer{
		logger: logger,
		port:   port,
		ready:  make(chan struct{}),
	}
}

var _ shutdown.Shutdowner = (*MetricsServer)(nil)

// Name returns the name of the metrics server component.
func (s *MetricsServer) Name() string {
	return "metrics-server"
}

// Ping returns nil when the server is ready to serve.
func (s *MetricsServer) Ping(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.ready:
		return nil
	default:
		return fmt.Errorf("metrics server is not ready")
	}
}

// Start starts the metrics HTTP server in a goroutine.
func (s *MetricsServer) Start(ctx context.Context) error {
	if s.inShutdown.Load() {
		s.logger.InfoContext(ctx, "metrics server is shutting down, skipping start")

		return nil
	}

	mux := http.NewServeMux()
	mux.Handle("/metrics", promhttp.Handler())

	addr := ":" + s.port
	s.server = &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadTimeout:       readTimeout,
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}

	lc := &net.ListenConfig{
		KeepAliveConfig: net.KeepAliveConfig{
			Enable: true,
		},
	}

	listener, err := lc.Listen(ctx, "tcp", addr)
	if err != nil {
		return fmt.Errorf("listen metrics tcp: %w", err)
	}

	s.logger.InfoContext(ctx, "metrics server listening", "addr", listener.Addr().String())

	go func() {
		close(s.ready)

		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			s.logger.ErrorContext(ctx, "metrics server error", "error", err)
		}
	}()

	return nil
}

// Ready returns a channel that is closed when the metrics server is ready.
func (s *MetricsServer) Ready() <-chan struct{} {
	return s.ready
}

// Shutdown gracefully shuts down the metrics server.
//
//nolint:dupl // mirrors Server.Shutdown for same lifecycle; dedup would abstract over *http.Server
func (s *MetricsServer) Shutdown(ctx context.Context) error {
	if !s.inShutdown.CompareAndSwap(false, true) {
		s.logger.ErrorContext(ctx, "metrics server is already shutting down, skipping shutdown")

		return nil
	}

	defer func() {
		s.logger.InfoContext(ctx, "metrics server shut downed")
	}()

	s.logger.InfoContext(ctx, "shutting down metrics server")

	if s.server == nil {
		return nil
	}

	if err := s.server.Shutdown(ctx); err != nil {
		s.logger.ErrorContext(ctx, "error shutting down metrics server", "error", err)

		return fmt.Errorf("metrics server shutdown: %w", err)
	}

	s.logger.InfoContext(ctx, "metrics server closed properly")

	return nil
}
