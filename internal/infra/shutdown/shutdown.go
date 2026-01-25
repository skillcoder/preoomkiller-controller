package shutdown

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const (
	defaultShutdownTimeout = 5 * time.Second
)

// Notify returns a channel that will receive SIGTERM and SIGINT signals.
// This should be called as the first thing in main() before any other initialization.
func Notify() <-chan os.Signal {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)

	return signals
}

type Handler struct {
	logger *slog.Logger
	quiter quiter
}

// New creates a new shutdown handler.
func New(logger *slog.Logger, quiter quiter) *Handler {
	return &Handler{
		logger: logger,
		quiter: quiter,
	}
}

// HandleSignals listens for SIGTERM and SIGINT signals and cancels the context when received.
func (h *Handler) HandleSignals(ctx context.Context, cancel func()) {
	select {
	case <-ctx.Done():
		h.logger.InfoContext(ctx, "terminating signal handler due to context done")

		return
	case <-h.quiter.Quit():
	}

	h.logger.InfoContext(ctx, "received termination signal, terminating")

	cancel()
}

func (h *Handler) CheckTermination(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("termination context done before startup: %w", ctx.Err())
	default:
	}

	terminationFile := "/mnt/signal/terminating"
	if _, err := os.Stat(terminationFile); err == nil {
		return fmt.Errorf("termination file found: %w", err)
	}

	return nil
}

// CheckTerminationFile checks if the termination file exists
func CheckTerminationFile(ctx context.Context, logger *slog.Logger, terminationFile string) bool {
	_, err := os.Stat(terminationFile)
	if err != nil {
		if !os.IsNotExist(err) {
			logger.ErrorContext(ctx, "error checking termination file",
				"error", err,
				"path", terminationFile,
			)

			return false
		}

		return false
	}

	logger.InfoContext(ctx, "termination file found", "path", terminationFile)

	return true
}

// GracefulShutdown performs graceful shutdown of the components with timeout.
func GracefulShutdown(
	originCtx context.Context,
	logger *slog.Logger,
	shutdowners []Shutdowner,
) error {
	// Use context.WithoutCancel to ensure shutdown continues even if originCtx is cancelled
	ctx, cancel := context.WithTimeout(context.WithoutCancel(originCtx), defaultShutdownTimeout)
	defer cancel()

	componentsShutdownErrors := make(chan error, len(shutdowners))

	// Shutdown components in reverse order to ensure dependencies are met
	for i := len(shutdowners) - 1; i >= 0; i-- {
		start := time.Now()
		shutdowner := shutdowners[i]

		if err := shutdowner.Shutdown(ctx); err != nil {
			logger.ErrorContext(ctx, "component shutdown failed",
				"component", shutdowner.Name(),
				"duration", time.Since(start),
				"reason", err,
			)

			// collect errors from components
			componentsShutdownErrors <- err

			continue
		}

		logger.InfoContext(ctx, "component shutdown completed",
			"component", shutdowner.Name(),
			"duration", time.Since(start),
		)
	}

	close(componentsShutdownErrors)

	var errs error
	// collect errors from components
	for err := range componentsShutdownErrors {
		errs = errors.Join(errs, err)
	}

	return errs
}
