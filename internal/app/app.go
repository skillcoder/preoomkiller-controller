package app

import (
	"context"
	"fmt"
	"log/slog"
	"math"
	"sync/atomic"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/skillcoder/preoomkiller-controller/internal/adapters/outbound/k8s"
	"github.com/skillcoder/preoomkiller-controller/internal/config"
	"github.com/skillcoder/preoomkiller-controller/internal/httpserver"
	"github.com/skillcoder/preoomkiller-controller/internal/infra/shutdown"
	"github.com/skillcoder/preoomkiller-controller/internal/logic/controller"
)

const defaultShutdownersCount = 10

type App struct {
	logger        *slog.Logger
	signalHandler signalHandler
	appState      appstater
	shutdowners   []shutdown.Shutdowner
	controller    appServer
	httpServer    appServer
}

// New creates a new application instance with all dependencies wired.
func New(
	logger *slog.Logger,
	cfg *config.Config,
	appState appstater,
) (*App, error) {
	shutdowners := make([]shutdown.Shutdowner, 0, defaultShutdownersCount)
	// Create K8s config
	kubeConfig, err := clientcmd.BuildConfigFromFlags(
		cfg.KubeMaster,
		cfg.KubeConfig,
	)
	if err != nil {
		return nil, fmt.Errorf("build k8s config: %w", err)
	}

	// Create K8s clientset
	clientset, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("create clientset: %w", err)
	}

	// Create metrics clientset
	metricsClientset, err := metricsv.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("create metrics clientset: %w", err)
	}

	// Create secondary adapter (K8s adapter)
	k8sRepo := k8s.New(logger, clientset, metricsClientset)

	// Create logic service (inject repository adapter)
	controllerService := controller.New(
		logger,
		k8sRepo,
		cfg.Interval,
	)

	// Create HTTP server
	httpServer := httpserver.New(logger, appState, cfg.HTTPPort)

	// Create signal handler
	signalHandler := shutdown.New(logger, appState)

	return &App{
		controller:    controllerService,
		signalHandler: signalHandler,
		appState:      appState,
		httpServer:    httpServer,
		shutdowners:   shutdowners,
		logger:        logger,
	}, nil
}

// Run starts the application and blocks until context is cancelled.
func (a *App) Run(originCtx context.Context) error {
	// Check termination file
	err := a.signalHandler.CheckTermination(originCtx)
	if err != nil {
		return fmt.Errorf("check termination: %w", err)
	}

	ctx, cancel := context.WithCancel(originCtx)
	defer cancel()

	// Set starting state
	if err = a.appState.SetStarting(ctx); err != nil {
		return fmt.Errorf("set starting application state: %w", err)
	}

	// Start signal handler
	go a.signalHandler.HandleSignals(ctx, cancel)

	// Start HTTP server
	if err = a.httpServer.Start(ctx); err != nil {
		return fmt.Errorf("start http server: %w", err)
	}

	a.shutdowners = append(a.shutdowners, a.httpServer)

	// Start controller in goroutine
	err = a.controller.Start(ctx)
	if err != nil {
		return fmt.Errorf("start controller: %w", err)
	}

	a.shutdowners = append(a.shutdowners, a.controller)

	// Wait for both httpServer and controller to be ready
	select {
	case <-ctx.Done():
		return fmt.Errorf("context done")
	case <-allChannelsClose(ctx, a.logger, a.httpServer.Ready(), a.controller.Ready()):
		// Both are ready
	}

	// Set running state
	if err := a.appState.SetRunning(ctx); err != nil {
		return fmt.Errorf("set running application state: %w", err)
	}

	a.logger.InfoContext(ctx, "starting controller")

	// Wait for shutdown signal, context cancellation, or controller error
	select {
	case <-a.appState.Quit():
		cancel()
		a.logger.InfoContext(ctx, "shutting down application by signal")

		return a.Shutdown(ctx)
	case <-ctx.Done():
		a.logger.InfoContext(ctx, "shutting down application by context")

		return a.Shutdown(ctx)
	}
}

// allChannelsClose waits for all provided channels to close/signal and returns
// a channel that closes when all input channels have signaled.
func allChannelsClose(ctx context.Context, logger *slog.Logger, cs ...<-chan struct{}) <-chan struct{} {
	count := len(cs)
	out := make(chan struct{})

	if count == 0 {
		close(out)

		return out
	}

	var readyCount atomic.Int32

	if count > math.MaxInt32 || count < 0 {
		// This should never happen in practice, but handle overflow case
		close(out)

		logger.ErrorContext(ctx, "allChannelsClose: len(cs) > math.MaxInt32 or < 0",
			"len", len(cs),
		)

		return out
	}

	targetCount := int32(count)

	// Wait for each channel in a separate goroutine
	for _, c := range cs {
		go func(ch <-chan struct{}) {
			<-ch

			if readyCount.Add(1) == targetCount {
				close(out)
			}
		}(c)
	}

	return out
}

// Shutdown gracefully shuts down the application
func (a *App) Shutdown(originCtx context.Context) error {
	return shutdown.GracefulShutdown(originCtx, a.logger, a.appState, a.shutdowners)
}
