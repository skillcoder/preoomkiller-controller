package appstate

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/skillcoder/preoomkiller-controller/internal/infra/pinger"
	"github.com/skillcoder/preoomkiller-controller/internal/infra/shutdown"
)

// State represents the application state
type State string

const (
	// StateInit is the initial state when the application is created
	StateInit State = "init"

	// StateStarting is the state when the application is starting up
	StateStarting State = "starting"

	// StateRunning is the state when the application is running normally
	StateRunning State = "running"

	// StateTerminating is the state when the application is shutting down
	StateTerminating State = "terminating"

	// StateTerminated is the final state when the application has terminated
	StateTerminated State = "terminated"
)

const defaultShutdownersCount = 10

// AppState manages the application state with thread-safe operations
type AppState struct {
	mu                  sync.RWMutex
	logger              *slog.Logger
	startedAt           time.Time
	readyAt             *time.Time
	terminatingAt       *time.Time
	state               State
	quit                <-chan os.Signal
	terminationFilePath string
	pinger              pingerServer
	shutdowners         []shutdown.Shutdowner
}

// New creates a new AppState with the given start time
func New(
	logger *slog.Logger,
	appStart time.Time,
	terminationFilePath string,
	quit <-chan os.Signal,
	pinger pingerServer,
) *AppState {
	return &AppState{
		logger:              logger,
		startedAt:           appStart,
		state:               StateInit,
		quit:                quit,
		terminationFilePath: terminationFilePath,
		pinger:              pinger,
		shutdowners:         make([]shutdown.Shutdowner, 0, defaultShutdownersCount),
	}
}

func (s *AppState) RegisterPinger(pinger pinger.Pinger) error {
	return s.pinger.Register(pinger)
}

func (s *AppState) RegisterShutdowner(shutdowner shutdown.Shutdowner) error {
	s.shutdowners = append(s.shutdowners, shutdowner)

	return nil
}

func (s *AppState) GetAllStats() map[string]*pinger.Statistics {
	return s.pinger.GetAllStats()
}

// SetStarting transitions the state from Init to Starting
func (s *AppState) SetStarting(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state != StateInit {
		return fmt.Errorf("set starting: %w", ErrInvalidStateTransition)
	}

	return s.setState(StateStarting)
}

// SetRunning transitions the state from Starting to Running
func (s *AppState) SetRunning(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	defer func() {
		// Check termination file after initialization
		if shutdown.CheckTerminationFile(ctx, s.logger, s.terminationFilePath) {
			pid := os.Getpid()
			s.logger.InfoContext(ctx, "termination file found after initialization, sending SIGTERM",
				"pid", pid,
			)

			killErr := syscall.Kill(pid, syscall.SIGTERM)
			if killErr != nil {
				s.logger.ErrorContext(ctx, "failed to send SIGTERM",
					"error", killErr,
					"pid", pid,
				)
			}
		}
	}()

	if s.state != StateStarting {
		return fmt.Errorf("set running: %w", ErrInvalidStateTransition)
	}

	now := time.Now()
	s.readyAt = &now

	return s.setState(StateRunning)
}

// SetTerminating transitions the state to Terminating
func (s *AppState) SetTerminating(_ context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == StateTerminated {
		return fmt.Errorf("set terminating: %w", ErrAlreadyTerminated)
	}

	now := time.Now()
	s.terminatingAt = &now

	return s.setState(StateTerminating)
}

// setState is an internal method to set the state
func (s *AppState) setState(newState State) error {
	if s.state == StateTerminated {
		return fmt.Errorf("set state: %w", ErrAlreadyTerminated)
	}

	s.state = newState

	return nil
}

// GetState returns the current application state
func (s *AppState) GetState() State {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.state
}

// GetStartTime returns the time when the application started
func (s *AppState) GetStartTime() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.startedAt
}

// GetUptime returns the duration since the application started
func (s *AppState) GetUptime() time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return time.Since(s.startedAt)
}

// IsHealthy returns true if the application is in a healthy state (running)
func (s *AppState) IsHealthy() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.state == StateRunning
}

// IsReady returns true if the application is ready to serve requests (running and readyAt is set)
func (s *AppState) IsReady() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.state == StateRunning && s.readyAt != nil
}

// Quit returns the channel that will receive the signal when shutdown is requested
func (s *AppState) Quit() <-chan os.Signal {
	return s.quit
}

// Shutdown transitions the application to the terminated state
func (s *AppState) Shutdown(ctx context.Context) error {
	if err := s.SetTerminating(ctx); err != nil {
		return fmt.Errorf("set terminating application state: %w", err)
	}

	err := shutdown.GracefulShutdown(ctx, s.logger, s.shutdowners)
	if err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.state == StateTerminated {
		return nil
	}

	s.state = StateTerminated

	return nil
}
