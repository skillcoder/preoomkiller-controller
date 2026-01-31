package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"
)

const (
	envLeakRateMiPerMin = "LEAK_RATE_MI_PER_MIN"
	defaultLeakRate     = 10
	addr                = ":8080"
	logInterval         = 10 * time.Second
	shutdownTimeout     = 5 * time.Second
	readTimeout         = 2 * time.Second
	readHeaderTimeout   = 2 * time.Second
	writeTimeout        = 3 * time.Second
	idleTimeout         = 10 * time.Second
	maxHeaderBytes      = 1 << 12 // 4kb
)

func main() {
	ctx := context.Background()

	if err := run(ctx); err != nil {
		slog.Error("run failed", "reason", err)
		time.Sleep(1 * time.Second)
		os.Exit(1)
	}

	slog.Info("bye")
}

func run(runCtx context.Context) error {
	ctx, stop := signal.NotifyContext(runCtx, syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	rate, err := parseLeakRate()
	if err != nil {
		return fmt.Errorf("parse leak rate: %w", err)
	}

	bytesPerSecond := int64((float64(rate) * 1024 * 1024) / 60)

	slog.Info("starting memory-leaker", "rate_mi_per_min", rate, "bytes_per_second", bytesPerSecond)

	srv := newServer()
	go runServer(srv)
	runLeakLoop(ctx, bytesPerSecond)

	slog.Info("shutting down")
	if err := shutdownServer(ctx, srv); err != nil {
		return fmt.Errorf("shutdown server: %w", err)
	}

	slog.Info("shutdown complete")

	return nil
}

func newServer() *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })

	return &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadTimeout:       readTimeout,
		ReadHeaderTimeout: readHeaderTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
		MaxHeaderBytes:    maxHeaderBytes,
	}
}

func runServer(srv *http.Server) {
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("http server failed", "reason", err)
	}

	slog.Info("http server exited")
}

func runLeakLoop(ctx context.Context, bytesPerSecond int64) {
	var totalAllocated int64
	var memory [][]byte

	ticker := time.NewTicker(time.Second)
	logTicker := time.NewTicker(logInterval)

	defer ticker.Stop()
	defer logTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Info("shutdown signal received, draining")
			return
		case <-ticker.C:
			chunk := make([]byte, bytesPerSecond)

			for i := range bytesPerSecond {
				chunk[i] = byte(i % 256)
			}

			memory = append(memory, chunk)
			totalAllocated += bytesPerSecond
		case <-logTicker.C:
			mib := float64(totalAllocated) / (1024 * 1024)
			slog.Info("memory leak progress", "allocated_mib", mib, "total_bytes", totalAllocated)
		}
	}
}

func shutdownServer(ctx context.Context, srv *http.Server) error {
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), shutdownTimeout)
	defer cancel()

	slog.Info("shutting down http server")

	if err := srv.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown http server: %w", err)
	}

	slog.Info("http server shut downed")

	return nil
}

func parseLeakRate() (float64, error) {
	s := os.Getenv(envLeakRateMiPerMin)
	if s == "" {
		return defaultLeakRate, nil
	}

	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", envLeakRateMiPerMin, err)
	}

	if v <= 0 {
		return 0, fmt.Errorf("%s must be positive, got %v", envLeakRateMiPerMin, v)
	}

	return v, nil
}
