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
	"github.com/skillcoder/preoomkiller-controller/internal/infra/cronparser"
	"github.com/skillcoder/preoomkiller-controller/internal/infra/shutdown"
	"github.com/skillcoder/preoomkiller-controller/internal/logic/controller"
)

type App struct {
	logger        *slog.Logger
	signalHandler signalHandler
	appState      appstater
	controller    appServer
	httpServer    appServer
	metricsServer appServer
}

// New creates a new application instance with all dependencies wired.
func New(
	logger *slog.Logger,
	cfg *config.Config,
	appState appstater,
) (*App, error) {
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

	cronParser := cronparser.New()

	// Create logic service (inject repository adapter)
	controllerService := controller.New(
		logger,
		k8sRepo,
		cronParser,
		cfg.Interval,
		cfg.PodLabelSelector,
		cfg.AnnotationMemoryThresholdKey,
		cfg.AnnotationRestartScheduleKey,
		cfg.AnnotationTZKey,
		controller.PreoomkillerAnnotationRestartAtKey,
		cfg.RestartScheduleJitterMax,
		cfg.MinPodAgeBeforeEviction,
	)

	// Create HTTP server
	httpServer := httpserver.New(logger, appState, cfg.HTTPPort)

	// Create metrics server (separate port for Prometheus scraping)
	metricsServer := httpserver.NewMetricsServer(logger, cfg.MetricsPort)

	// Create signal handler
	signalHandler := shutdown.New(logger, appState)

	return &App{
		controller:    controllerService,
		signalHandler: signalHandler,
		appState:      appState,
		httpServer:    httpServer,
		metricsServer: metricsServer,
		logger:        logger,
	}, nil
}

// Run starts the application and blocks until context is cancelled.
func (a *App) Run(originCtx context.Context) error {
	if err := a.initialize(originCtx); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(originCtx)
	defer cancel()

	if err := a.startServices(ctx, cancel); err != nil {
		return err
	}

	if err := a.waitForReady(ctx); err != nil {
		return err
	}

	return a.runUntilShutdown(ctx)
}

// initialize checks termination file and sets starting state
func (a *App) initialize(ctx context.Context) error {
	if err := a.signalHandler.CheckTermination(ctx); err != nil {
		return fmt.Errorf("check termination: %w", err)
	}

	return nil
}

// startServices starts all services and registers them with app state
func (a *App) startServices(ctx context.Context, cancel func()) error {
	if err := a.appState.SetStarting(ctx); err != nil {
		return fmt.Errorf("set starting application state: %w", err)
	}

	go a.signalHandler.HandleSignals(ctx, cancel)

	if err := a.startHTTPServer(ctx); err != nil {
		return fmt.Errorf("start http server: %w", err)
	}

	if err := a.startMetricsServer(ctx); err != nil {
		return fmt.Errorf("start metrics server: %w", err)
	}

	if err := a.startController(ctx); err != nil {
		return fmt.Errorf("start controller: %w", err)
	}

	return nil
}

// startHTTPServer starts the HTTP server and registers it
func (a *App) startHTTPServer(ctx context.Context) error {
	if err := a.httpServer.Start(ctx); err != nil {
		return fmt.Errorf("start http server: %w", err)
	}

	if err := a.appState.RegisterShutdowner(a.httpServer); err != nil {
		return fmt.Errorf("register shutdowner: %w", err)
	}

	if err := a.appState.RegisterPinger(a.httpServer); err != nil {
		return fmt.Errorf("register pinger: %w", err)
	}

	return nil
}

// startMetricsServer starts the metrics server and registers it
func (a *App) startMetricsServer(ctx context.Context) error {
	if err := a.metricsServer.Start(ctx); err != nil {
		return fmt.Errorf("start metrics server: %w", err)
	}

	if err := a.appState.RegisterShutdowner(a.metricsServer); err != nil {
		return fmt.Errorf("register metrics shutdowner: %w", err)
	}

	if err := a.appState.RegisterPinger(a.metricsServer); err != nil {
		return fmt.Errorf("register metrics pinger: %w", err)
	}

	return nil
}

// startController starts the controller and registers it
func (a *App) startController(ctx context.Context) error {
	if err := a.controller.Start(ctx); err != nil {
		return fmt.Errorf("start controller: %w", err)
	}

	if err := a.appState.RegisterShutdowner(a.controller); err != nil {
		return fmt.Errorf("register shutdowner: %w", err)
	}

	if err := a.appState.RegisterPinger(a.controller); err != nil {
		return fmt.Errorf("register pinger: %w", err)
	}

	return nil
}

// waitForReady waits for all services to be ready
func (a *App) waitForReady(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return fmt.Errorf("context done")
	case <-allChannelsClose(ctx, a.logger, a.httpServer.Ready(), a.metricsServer.Ready(), a.controller.Ready()):
		// All are ready
	}

	if err := a.appState.SetRunning(ctx); err != nil {
		return fmt.Errorf("set running application state: %w", err)
	}

	a.logger.InfoContext(ctx, "starting controller")

	return nil
}

// runUntilShutdown waits for shutdown signal and performs shutdown
func (a *App) runUntilShutdown(ctx context.Context) error {
	<-ctx.Done()
	a.logger.InfoContext(ctx, "shutting down application by context")

	return a.Shutdown(ctx)
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
func (a *App) Shutdown(ctx context.Context) error {
	return a.appState.Shutdown(ctx)
}
