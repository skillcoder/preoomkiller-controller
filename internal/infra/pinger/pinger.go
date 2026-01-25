package pinger

import (
	"context"
	"fmt"
	"log/slog"
	"maps"
	"sync"
	"sync/atomic"
	"time"

	"github.com/skillcoder/preoomkiller-controller/internal/infra/shutdown"
)

const (
	// defaultPingTimeout is the default timeout for ping operations
	defaultPingTimeout = 1 * time.Second
)

// Optional interface types for type assertions
type readyCriticalPinger interface {
	PingerReadyCritical() bool
}

type healthCriticalPinger interface {
	PingerCritical() bool
}

type timeoutPinger interface {
	PingerTimeout() time.Duration
}

// pingerInfo holds pinger instance and its configuration
type pingerInfo struct {
	// FIXME: replace pinger with name and pingFunc.
	pinger         Pinger
	readyCritical  bool
	healthCritical bool
	timeout        time.Duration
}

// Service manages health check pingers and tracks their statistics
type Service struct {
	logger     *slog.Logger
	interval   time.Duration
	pingers    map[string]*pingerInfo
	stats      map[string]*Stats
	mu         sync.RWMutex
	ready      chan struct{}
	inShutdown atomic.Bool
	doneCh     chan struct{}
	wg         sync.WaitGroup
}

// New creates a new pinger service with the specified interval
func New(
	logger *slog.Logger,
	interval time.Duration,
) *Service {
	return &Service{
		logger:   logger,
		interval: interval,
		pingers:  make(map[string]*pingerInfo),
		stats:    make(map[string]*Stats),
		ready:    make(chan struct{}),
		doneCh:   make(chan struct{}),
	}
}

var _ shutdown.Shutdowner = (*Service)(nil)

// Name returns the name of the pinger service component
func (s *Service) Name() string {
	return "pinger-service"
}

// Register registers a pinger with the given name
func (s *Service) Register(pinger Pinger) error {
	if pinger == nil {
		return fmt.Errorf("register pinger: pinger cannot be nil")
	}

	name := pinger.Name()

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.pingers[name]; exists {
		return fmt.Errorf("register pinger %s: %w", name, ErrPingerAlreadyRegistered)
	}

	// Detect optional interface methods
	readyCritical := true

	if rc, ok := pinger.(readyCriticalPinger); ok {
		readyCritical = rc.PingerReadyCritical()
	}

	healthCritical := true

	if hc, ok := pinger.(healthCriticalPinger); ok {
		healthCritical = hc.PingerCritical()
	}

	timeout := defaultPingTimeout

	if tp, ok := pinger.(timeoutPinger); ok {
		customTimeout := tp.PingerTimeout()
		if customTimeout > 0 {
			timeout = customTimeout
		}
	}

	info := &pingerInfo{
		pinger:         pinger,
		readyCritical:  readyCritical,
		healthCritical: healthCritical,
		timeout:        timeout,
	}

	s.pingers[name] = info
	s.stats[name] = NewPingerStats(name)

	logFields := []any{"name", name}

	if readyCritical {
		logFields = append(logFields, "readyCritical", true)
	}

	if healthCritical {
		logFields = append(logFields, "healthCritical", true)
	}

	if timeout != defaultPingTimeout {
		logFields = append(logFields, "timeout", timeout)
	}

	s.logger.Info("pinger registered", logFields...)

	return nil
}

// Start starts the pinger service in a goroutine
func (s *Service) Start(ctx context.Context) error {
	if s.inShutdown.Load() {
		s.logger.InfoContext(ctx, "pinger service is shutting down, skipping start")

		return nil
	}

	go s.run(ctx)

	return nil
}

// Ready returns a channel that is closed when the pinger service is ready
func (s *Service) Ready() <-chan struct{} {
	return s.ready
}

// Shutdown gracefully shuts down the pinger service
func (s *Service) Shutdown(ctx context.Context) error {
	if !s.inShutdown.CompareAndSwap(false, true) {
		s.logger.ErrorContext(ctx, "pinger service is already shutting down, skipping shutdown")

		return nil
	}

	defer func() {
		s.logger.InfoContext(ctx, "pinger service shut downed")
	}()

	s.logger.InfoContext(ctx, "shutting down pinger service")

	// Wait for run goroutine to exit
	select {
	case <-ctx.Done():
		return fmt.Errorf("shutdown context done before pinger loop exited: %w", ctx.Err())
	case <-s.doneCh:
		s.logger.InfoContext(ctx, "pinger loop exited")
	}

	// Wait for any in-flight ping operations to complete
	s.wg.Wait()

	return nil
}

// GetStats returns statistics for a specific pinger
func (s *Service) GetStats(name string) (*Statistics, error) {
	s.mu.RLock()
	info, infoExists := s.pingers[name]
	stats, statsExists := s.stats[name]
	s.mu.RUnlock()

	if !infoExists || !statsExists {
		return nil, fmt.Errorf("get stats: %w: %s", ErrPingerNotFound, name)
	}

	return GetStatistics(stats, info), nil
}

// GetAllStats returns a deep copy of all pinger statistics
func (s *Service) GetAllStats() map[string]*Statistics {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make(map[string]*Statistics, len(s.stats))
	for name, stats := range s.stats {
		info, exists := s.pingers[name]
		if !exists {
			s.logger.Warn("get all stats: pinger not found", "name", name)

			continue
		}

		result[name] = GetStatistics(stats, info)
	}

	return result
}

// run is the main goroutine that runs pingers at intervals
func (s *Service) run(ctx context.Context) {
	defer close(s.doneCh)

	logger := s.logger.With("component", "pinger-run")

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	// Run first ping immediately
	s.runPingers(ctx, logger)

	// Close ready channel immediately - pingers start on first interval
	close(s.ready)

	for {
		// Check shutdown flag first
		if s.inShutdown.Load() {
			logger.InfoContext(ctx, "terminating pinger loop")

			return
		}

		select {
		case <-ticker.C:
			s.runPingers(ctx, logger)
		case <-ctx.Done():
			logger.InfoContext(ctx, "terminating pinger loop")

			return
		}
	}
}

// runPingers executes all registered pingers in parallel
func (s *Service) runPingers(ctx context.Context, logger *slog.Logger) {
	s.mu.RLock()
	pingers := make(map[string]*pingerInfo, len(s.pingers))
	maps.Copy(pingers, s.pingers)
	s.mu.RUnlock()

	if len(pingers) == 0 {
		return
	}

	var wg sync.WaitGroup

	for name, info := range pingers {
		// Check if context is cancelled before starting each pinger
		select {
		case <-ctx.Done():
			return
		default:
		}

		wg.Add(1)
		s.wg.Add(1)

		go func(n string, i *pingerInfo) {
			defer wg.Done()
			defer s.wg.Done()

			// Create context with per-pinger timeout
			pingCtx, cancel := context.WithTimeout(ctx, i.timeout)
			defer cancel()

			start := time.Now()
			err := i.pinger.Ping(pingCtx)
			latency := time.Since(start)

			s.updateStats(n, latency, err)

			if err != nil {
				logger.DebugContext(ctx, "pinger error",
					"name", n,
					"latency", latency,
					"reason", err,
				)
			} else {
				logger.DebugContext(ctx, "pinger success",
					"name", n,
					"latency", latency,
				)
			}
		}(name, info)
	}

	// Wait for all pingers with context cancellation support
	done := make(chan struct{})

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return
	case <-done:
	}
}

// updateStats updates statistics for a pinger in a thread-safe manner
func (s *Service) updateStats(name string, latency time.Duration, err error) {
	s.mu.RLock()
	stats, exists := s.stats[name]
	s.mu.RUnlock()

	if !exists {
		return
	}

	stats.mu.Lock()
	defer stats.mu.Unlock()

	now := time.Now()
	stats.LastRun = now

	if err != nil {
		stats.LastError = err
		stats.LastErrorSnapshot = &ErrorSnapshot{
			Timestamp: now,
			Latency:   latency,
			Error:     err,
		}
		stats.ErrorLatencies.Add(latency)
	} else {
		stats.LastError = nil
		stats.SuccessLatencies.Add(latency)
	}
}
