package app

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/skillcoder/preoomkiller-controller/internal/adapters/outbound/k8s"
	"github.com/skillcoder/preoomkiller-controller/internal/config"
	"github.com/skillcoder/preoomkiller-controller/internal/infra/logging"
	"github.com/skillcoder/preoomkiller-controller/internal/infra/shutdown"
	"github.com/skillcoder/preoomkiller-controller/internal/logic/controller"
)

type App struct {
	controller controller.UseCase
	shutdowner shutdown.Shutdowner
	logger     *slog.Logger
}

// New creates a new application instance with all dependencies wired.
func New(cfg *config.Config, signals <-chan os.Signal) (*App, error) {
	logger := logging.New(cfg.LogFormat, cfg.LogLevel)

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
	controllerService := controller.NewService(
		logger,
		k8sRepo,
		cfg.Interval,
	)

	// Create shutdown handler
	shutdownHandler := shutdown.New(logger, signals)

	return &App{
		controller: controllerService,
		shutdowner: shutdownHandler,
		logger:     logger,
	}, nil
}

// Run starts the application and blocks until context is cancelled.
func (a *App) Run(originCtx context.Context) error {
	err := a.shutdowner.CheckTermination(originCtx)
	if err != nil {
		return fmt.Errorf("check termination: %w", err)
	}

	ctx, cancel := context.WithCancel(originCtx)
	defer cancel()

	go a.shutdowner.HandleSignals(ctx, cancel)

	a.logger.InfoContext(ctx, "starting controller")

	// Run controller (blocks until context is cancelled)
	// Context cancellation is handled by signal listener in main()
	return a.controller.RunCommand(ctx)
}
