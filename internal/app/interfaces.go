package app

import (
	"context"
	"os"
	"time"

	"github.com/skillcoder/preoomkiller-controller/internal/infra/appstate"
	"github.com/skillcoder/preoomkiller-controller/internal/infra/shutdown"
)

// appstater defines the interface for application state management
type appstater interface {
	Quit() <-chan os.Signal
	SetStarting(ctx context.Context) error
	SetRunning(ctx context.Context) error
	SetTerminating(ctx context.Context) error
	GetStartTime() time.Time
	GetState() appstate.State
	GetUptime() time.Duration
	IsHealthy() bool
	IsReady() bool
	Shutdown(ctx context.Context) error
}

type signalHandler interface {
	HandleSignals(ctx context.Context, cancel func())
	CheckTermination(ctx context.Context) error
}

type appServer interface {
	Start(ctx context.Context) error
	Ready() <-chan struct{}
	shutdown.Shutdowner
}
