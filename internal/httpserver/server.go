package httpserver

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"sync/atomic"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/skillcoder/preoomkiller-controller/internal/infra/appstate"
	"github.com/skillcoder/preoomkiller-controller/internal/infra/shutdown"
)

type Server struct {
	logger     *slog.Logger
	appState   appstater
	port       string
	server     *http.Server
	ready      chan struct{}
	inShutdown atomic.Bool
}

// New creates a new HTTP server instance
func New(logger *slog.Logger, appState appstater, port string) *Server {
	if port == "" {
		port = defaultPort
	}

	return &Server{
		logger:   logger,
		appState: appState,
		port:     port,
		ready:    make(chan struct{}),
	}
}

var _ shutdown.Shutdowner = (*Server)(nil)

// Name returns the name of the server component
func (s *Server) Name() string {
	return "http-server"
}

func (s *Server) Ping(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-s.ready:
		return nil
	default:
		return fmt.Errorf("http server is not ready")
	}
}

// Start starts the HTTP server in a goroutine
func (s *Server) Start(ctx context.Context) error {
	if s.inShutdown.Load() {
		s.logger.InfoContext(ctx, "http server is shutting down, skipping start")

		return nil
	}

	router := chi.NewRouter()

	// Add middleware
	router.Use(middleware.RequestID)
	router.Use(middleware.RealIP)
	router.Use(middleware.Logger)
	router.Use(middleware.Recoverer)

	// Register health endpoints
	router.Get("/-/healthz", appstate.HandleHealthz(s.logger, s.appState))
	router.Get("/-/readyz", appstate.HandleReadyz(s.logger, s.appState))
	router.Get("/-/status", appstate.HandleStatus(s.logger, s.appState))

	addr := ":" + s.port
	s.server = &http.Server{
		Addr:              addr,
		Handler:           router,
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
		return fmt.Errorf("listen tcp: %w", err)
	}

	s.logger.InfoContext(ctx, "http server listening", "addr", listener.Addr().String())

	go func() {
		close(s.ready)

		if err := s.server.Serve(listener); err != nil && err != http.ErrServerClosed {
			s.logger.ErrorContext(ctx, "http server error", "error", err)
		}
	}()

	return nil
}

// Ready returns a channel that is closed when the HTTP server is ready to serve requests
func (s *Server) Ready() <-chan struct{} {
	return s.ready
}

// Shutdown gracefully shuts down the HTTP server
//
//nolint:dupl // mirrors MetricsServer.Shutdown for same lifecycle
func (s *Server) Shutdown(ctx context.Context) error {
	if !s.inShutdown.CompareAndSwap(false, true) {
		s.logger.ErrorContext(ctx, "http server is already shutting down, skipping shutdown")

		return nil // Already shutting down
	}

	defer func() {
		s.logger.InfoContext(ctx, "http server shut downed")
	}()

	s.logger.InfoContext(ctx, "shutting down http server")

	if s.server == nil {
		return nil
	}

	if err := s.server.Shutdown(ctx); err != nil {
		s.logger.ErrorContext(ctx, "error shutting down http server", "error", err)

		return fmt.Errorf("http server shutdown: %w", err)
	}

	s.logger.InfoContext(ctx, "http server closed properly")

	return nil
}
