package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/skillcoder/preoomkiller-controller/internal/app"
	"github.com/skillcoder/preoomkiller-controller/internal/config"
	"github.com/skillcoder/preoomkiller-controller/internal/infra/shutdown"
)

func main() {
	// Start listening for signals immediately as first thing, before any other initialization
	signals := shutdown.Notify()
	ctx := context.Background()

	err := run(ctx, signals)
	if err != nil {
		slog.ErrorContext(ctx, "failed to run", "reason", err)
		// Give the logger some time to flush
		time.Sleep(1 * time.Second)
		os.Exit(1)
	}

	slog.InfoContext(ctx, "bye")
}

func run(ctx context.Context, signals <-chan os.Signal) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	application, err := app.New(cfg, signals)
	if err != nil {
		return fmt.Errorf("new application: %w", err)
	}

	return application.Run(ctx)
}
