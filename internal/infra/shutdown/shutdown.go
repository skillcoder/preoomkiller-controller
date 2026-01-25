package shutdown

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
)

// Notify returns a channel that will receive SIGTERM and SIGINT signals.
// This should be called as the first thing in main() before any other initialization.
func Notify() <-chan os.Signal {
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGTERM, syscall.SIGINT)

	return signals
}

type shutdowner struct {
	logger  *slog.Logger
	signals <-chan os.Signal
}

// New creates a new shutdown handler.
func New(logger *slog.Logger, signals <-chan os.Signal) Shutdowner {
	return &shutdowner{
		logger:  logger,
		signals: signals,
	}
}

var _ Shutdowner = (*shutdowner)(nil)

// HandleSignals listens for SIGTERM and SIGINT signals and cancels the context when received.
func (s *shutdowner) HandleSignals(ctx context.Context, cancel func()) {
	select {
	case <-ctx.Done():
		s.logger.InfoContext(ctx, "terminating signal handler due to context done")

		return
	case <-s.signals:
	}

	s.logger.InfoContext(ctx, "received termination signal, terminating")

	cancel()
}

func (s *shutdowner) CheckTermination(ctx context.Context) error {
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
