package app

import (
	"context"
	"os"
	"time"

	"github.com/skillcoder/preoomkiller-controller/internal/infra/appstate"
	"github.com/skillcoder/preoomkiller-controller/internal/infra/pinger"
	"github.com/skillcoder/preoomkiller-controller/internal/infra/shutdown"
)

// appstater defines the interface for application state management
type appstater interface {
	appStateRegistrar
	appStateQuerier
	appStateLifecycle
	appStateShutdowner
}

// appStateRegistrar handles registration of components
type appStateRegistrar interface {
	RegisterPinger(pinger pinger.Pinger) error
	RegisterShutdowner(shutdowner shutdown.Shutdowner) error
}

// appStateQuerier provides query methods for application state
type appStateQuerier interface {
	GetAllStats() map[string]*pinger.Statistics
	Quit() <-chan os.Signal
	GetStartTime() time.Time
	GetState() appstate.State
	GetUptime() time.Duration
	IsHealthy() bool
	IsReady() bool
}

// appStateLifecycle handles state transitions
type appStateLifecycle interface {
	SetStarting(ctx context.Context) error
	SetRunning(ctx context.Context) error
	SetTerminating(ctx context.Context) error
}

// appStateShutdowner handles graceful shutdown
type appStateShutdowner interface {
	Shutdown(ctx context.Context) error
}

type signalHandler interface {
	HandleSignals(ctx context.Context, cancel func())
	CheckTermination(ctx context.Context) error
}

type appServer interface {
	pinger.Pinger
	Start(ctx context.Context) error
	Ready() <-chan struct{}
	shutdown.Shutdowner
}
