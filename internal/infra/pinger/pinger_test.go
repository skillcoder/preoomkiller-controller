package pinger

import (
	"context"
	"errors"
	"log/slog"
	"strconv"
	"testing"
	"time"
)

func TestService_Register(t *testing.T) {
	t.Parallel()

	t.Run("register valid pinger", func(t *testing.T) {
		t.Parallel()

		logger := slog.Default()
		service := New(logger, 1*time.Second)
		pinger := &mockPinger{name: "test1"}

		err := service.Register(pinger)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("register nil pinger", func(t *testing.T) {
		t.Parallel()

		logger := slog.Default()
		service := New(logger, 1*time.Second)

		err := service.Register(nil)
		if err == nil {
			t.Fatal("expected error but got nil")
		}
	})

	t.Run("register duplicate pinger", func(t *testing.T) {
		t.Parallel()

		logger := slog.Default()
		service := New(logger, 1*time.Second)
		pinger1 := &mockPinger{name: "test3"}

		err := service.Register(pinger1)
		if err != nil {
			t.Fatalf("first registration failed: %v", err)
		}

		pinger2 := &mockPinger{name: "test3"}

		err = service.Register(pinger2)
		if err == nil {
			t.Fatal("expected error but got nil")
		}

		if !errors.Is(err, ErrPingerAlreadyRegistered) {
			t.Fatalf("expected error type %v, got %v", ErrPingerAlreadyRegistered, err)
		}
	})
}

func TestService_GetStats(t *testing.T) {
	t.Parallel()

	logger := slog.Default()
	service := New(logger, 1*time.Second)

	err := service.Register(&mockPinger{name: "test"})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	stats, err := service.GetStats("test")
	if err != nil {
		t.Fatalf("get stats failed: %v", err)
	}

	if stats == nil {
		t.Fatal("expected stats but got nil")
	}

	_, err = service.GetStats("nonexistent")
	if err == nil {
		t.Fatal("expected error for nonexistent pinger")
	}

	if !errors.Is(err, ErrPingerNotFound) {
		t.Fatalf("expected ErrPingerNotFound, got %v", err)
	}
}

func TestService_GetAllStats(t *testing.T) {
	t.Parallel()

	logger := slog.Default()
	service := New(logger, 1*time.Second)

	err := service.Register(&mockPinger{name: "pinger1"})
	if err != nil {
		t.Fatalf("register pinger1 failed: %v", err)
	}

	err = service.Register(&mockPinger{name: "pinger2"})
	if err != nil {
		t.Fatalf("register pinger2 failed: %v", err)
	}

	allStats := service.GetAllStats()
	if len(allStats) != 2 {
		t.Fatalf("expected 2 stats, got %d", len(allStats))
	}

	if allStats["pinger1"] == nil {
		t.Fatal("expected stats for pinger1")
	}

	if allStats["pinger2"] == nil {
		t.Fatal("expected stats for pinger2")
	}
}

func TestService_Start_Shutdown(t *testing.T) {
	t.Parallel()

	logger := slog.Default()
	service := New(logger, 100*time.Millisecond)

	err := service.Register(&mockPinger{name: "test"})
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = service.Start(ctx)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}

	// Wait for ready
	select {
	case <-service.Ready():
	case <-time.After(1 * time.Second):
		t.Fatal("service did not become ready")
	}

	// Let it run for a bit
	time.Sleep(250 * time.Millisecond)

	// Cancel the context to signal shutdown
	cancel()

	// Shutdown with reasonable timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	err = service.Shutdown(shutdownCtx)
	if err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
}

func TestLatencyBuffer(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		capacity int
		addCount int
		wantLen  int
	}{
		{
			name:     "add within capacity",
			capacity: 10,
			addCount: 5,
			wantLen:  5,
		},
		{
			name:     "add beyond capacity",
			capacity: 10,
			addCount: 15,
			wantLen:  10,
		},
		{
			name:     "add exactly capacity",
			capacity: 10,
			addCount: 10,
			wantLen:  10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			lb := NewLatencyBuffer(tt.capacity)

			for i := range tt.addCount {
				lb.Add(time.Duration(i) * time.Millisecond)
			}

			if lb.Len() != tt.wantLen {
				t.Fatalf("expected length %d, got %d", tt.wantLen, lb.Len())
			}

			all := lb.GetAll()
			if len(all) != tt.wantLen {
				t.Fatalf("expected GetAll length %d, got %d", tt.wantLen, len(all))
			}
		})
	}
}

func TestCalculatePercentile(t *testing.T) {
	t.Parallel()

	latencies := []time.Duration{
		10 * time.Millisecond,
		20 * time.Millisecond,
		30 * time.Millisecond,
		40 * time.Millisecond,
		50 * time.Millisecond,
		60 * time.Millisecond,
		70 * time.Millisecond,
		80 * time.Millisecond,
		90 * time.Millisecond,
		100 * time.Millisecond,
	}

	tests := []struct {
		name       string
		percentile float64
		want       time.Duration
	}{
		{
			name:       "p0",
			percentile: 0,
			want:       10 * time.Millisecond,
		},
		{
			name:       "p50",
			percentile: 50,
			want:       50 * time.Millisecond,
		},
		{
			name:       "p80",
			percentile: 80,
			want:       80 * time.Millisecond,
		},
		{
			name:       "p90",
			percentile: 90,
			want:       90 * time.Millisecond,
		},
		{
			name:       "p99",
			percentile: 99,
			want:       100 * time.Millisecond,
		},
		{
			name:       "p100",
			percentile: 100,
			want:       100 * time.Millisecond,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := CalculatePercentile(latencies, tt.percentile)
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestCalculateMedian(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		latencies []time.Duration
		want      time.Duration
	}{
		{
			name:      "odd count",
			latencies: []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond, 40 * time.Millisecond, 50 * time.Millisecond},
			want:      30 * time.Millisecond,
		},
		{
			name:      "even count",
			latencies: []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond, 40 * time.Millisecond},
			want:      25 * time.Millisecond,
		},
		{
			name:      "single value",
			latencies: []time.Duration{42 * time.Millisecond},
			want:      42 * time.Millisecond,
		},
		{
			name:      "empty",
			latencies: []time.Duration{},
			want:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := CalculateMedian(tt.latencies)
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestCalculateAverage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		latencies []time.Duration
		want      time.Duration
	}{
		{
			name:      "multiple values",
			latencies: []time.Duration{10 * time.Millisecond, 20 * time.Millisecond, 30 * time.Millisecond, 40 * time.Millisecond, 50 * time.Millisecond},
			want:      30 * time.Millisecond,
		},
		{
			name:      "single value",
			latencies: []time.Duration{42 * time.Millisecond},
			want:      42 * time.Millisecond,
		},
		{
			name:      "empty",
			latencies: []time.Duration{},
			want:      0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := CalculateAverage(tt.latencies)
			if got != tt.want {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestService_StatisticsTracking(t *testing.T) {
	t.Parallel()

	logger := slog.Default()
	service := New(logger, 100*time.Millisecond)

	successPinger := &mockPinger{name: "success", shouldError: false}
	errPinger := &mockPinger{name: "error", shouldError: true}

	err := service.Register(successPinger)
	if err != nil {
		t.Fatalf("register success pinger failed: %v", err)
	}

	err = service.Register(errPinger)
	if err != nil {
		t.Fatalf("register error pinger failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = service.Start(ctx)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}

	// Wait for ready
	select {
	case <-service.Ready():
	case <-time.After(1 * time.Second):
		t.Fatal("service did not become ready")
	}

	// Let it run for a bit to collect stats
	time.Sleep(350 * time.Millisecond)

	// Check success pinger stats
	successStats, err := service.GetStats("success")
	if err != nil {
		t.Fatalf("get success stats failed: %v", err)
	}

	if successStats.SuccessCount == 0 {
		t.Fatal("expected success count > 0")
	}

	// Check error pinger stats
	errorStats, err := service.GetStats("error")
	if err != nil {
		t.Fatalf("get error stats failed: %v", err)
	}

	if errorStats.ErrorCount == 0 {
		t.Fatal("expected error count > 0")
	}

	if errorStats.LastError == nil {
		t.Fatal("expected last error to be set")
	}

	if errorStats.LastErrorSnapshot == nil {
		t.Fatal("expected last error snapshot to be set")
	}

	// Cancel the context to signal shutdown
	cancel()

	// Shutdown with reasonable timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	err = service.Shutdown(shutdownCtx)
	if err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
}

func TestService_ParallelExecution(t *testing.T) {
	t.Parallel()

	logger := slog.Default()
	service := New(logger, 200*time.Millisecond)

	// Register multiple pingers
	for i := range 5 {
		err := service.Register(&mockPinger{name: "pinger" + strconv.Itoa(i)})
		if err != nil {
			t.Fatalf("register pinger%d failed: %v", i, err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err := service.Start(ctx)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}

	// Wait for ready
	select {
	case <-service.Ready():
	case <-time.After(1 * time.Second):
		t.Fatal("service did not become ready")
	}

	// Let it run for a bit
	time.Sleep(450 * time.Millisecond)

	// All pingers should have stats
	allStats := service.GetAllStats()
	if len(allStats) != 5 {
		t.Fatalf("expected 5 stats, got %d", len(allStats))
	}

	// Cancel the context to signal shutdown
	cancel()

	// Shutdown with reasonable timeout
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	err = service.Shutdown(shutdownCtx)
	if err != nil {
		t.Fatalf("shutdown failed: %v", err)
	}
}

func TestService_IsReady_IsHealthy(t *testing.T) {
	t.Parallel()

	logger := slog.Default()

	tests := []struct {
		name          string
		pinger        Pinger
		shouldError   bool
		wantIsReady   bool
		wantIsHealthy bool
	}{
		{
			name:          "normal pinger with error",
			pinger:        &mockPinger{name: "error", shouldError: true},
			shouldError:   true,
			wantIsReady:   false, // Default critical: ready only if no error
			wantIsHealthy: false, // Default critical: healthy only if no error
		},
		{
			name:          "normal pinger without error",
			pinger:        &mockPinger{name: "success", shouldError: false},
			shouldError:   false,
			wantIsReady:   true, // Default critical: ready when no error
			wantIsHealthy: true, // Default critical: healthy when no error
		},
		{
			name:          "non-critical pinger with error",
			pinger:        &criticalMockPinger{name: "non-critical-error", readyCritical: false, healthCritical: false, shouldError: true},
			shouldError:   true,
			wantIsReady:   true, // Non-critical: always ready regardless of error
			wantIsHealthy: true, // Non-critical: always healthy regardless of error
		},
		{
			name:          "non-critical pinger without error",
			pinger:        &criticalMockPinger{name: "non-critical-success", readyCritical: false, healthCritical: false, shouldError: false},
			shouldError:   false,
			wantIsReady:   true,
			wantIsHealthy: true,
		},
		{
			name:          "ready critical pinger with error",
			pinger:        &criticalMockPinger{name: "ready-critical-error", readyCritical: true, healthCritical: false, shouldError: true},
			shouldError:   true,
			wantIsReady:   false, // Critical: ready only if no error
			wantIsHealthy: true,  // Non-critical for health: always healthy
		},
		{
			name:          "health critical pinger with error",
			pinger:        &criticalMockPinger{name: "health-critical-error", readyCritical: false, healthCritical: true, shouldError: true},
			shouldError:   true,
			wantIsReady:   true,  // Non-critical for ready: always ready
			wantIsHealthy: false, // Critical: healthy only if no error
		},
		{
			name:          "both critical pinger with error",
			pinger:        &criticalMockPinger{name: "both-critical-error", readyCritical: true, healthCritical: true, shouldError: true},
			shouldError:   true,
			wantIsReady:   false, // Critical: ready only if no error
			wantIsHealthy: false, // Critical: healthy only if no error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			service := New(logger, 100*time.Millisecond)

			err := service.Register(tt.pinger)
			if err != nil {
				t.Fatalf("register failed: %v", err)
			}

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			err = service.Start(ctx)
			if err != nil {
				t.Fatalf("start failed: %v", err)
			}

			// Wait for ready
			select {
			case <-service.Ready():
			case <-time.After(1 * time.Second):
				t.Fatal("service did not become ready")
			}

			// Let it run to collect stats
			time.Sleep(250 * time.Millisecond)

			stats, err := service.GetStats(tt.pinger.Name())
			if err != nil {
				t.Fatalf("get stats failed: %v", err)
			}

			if stats.IsReady != tt.wantIsReady {
				t.Errorf("IsReady: expected %v, got %v", tt.wantIsReady, stats.IsReady)
			}

			if stats.IsHealthy != tt.wantIsHealthy {
				t.Errorf("IsHealthy: expected %v, got %v", tt.wantIsHealthy, stats.IsHealthy)
			}

			cancel()

			shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer shutdownCancel()

			_ = service.Shutdown(shutdownCtx)
		})
	}
}

func TestService_PingerTimeout(t *testing.T) {
	t.Parallel()

	logger := slog.Default()
	service := New(logger, 100*time.Millisecond)

	// Test pinger with custom timeout
	customTimeoutPinger := &timeoutMockPinger{
		name:    "custom-timeout",
		timeout: 50 * time.Millisecond,
		delay:   30 * time.Millisecond, // Should complete within timeout
	}

	err := service.Register(customTimeoutPinger)
	if err != nil {
		t.Fatalf("register failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = service.Start(ctx)
	if err != nil {
		t.Fatalf("start failed: %v", err)
	}

	// Wait for ready
	select {
	case <-service.Ready():
	case <-time.After(1 * time.Second):
		t.Fatal("service did not become ready")
	}

	// Let it run
	time.Sleep(250 * time.Millisecond)

	stats, err := service.GetStats("custom-timeout")
	if err != nil {
		t.Fatalf("get stats failed: %v", err)
	}

	// Should have successful pings
	if stats.SuccessCount == 0 {
		t.Error("expected success count > 0")
	}

	// Test pinger that times out
	timeoutPinger := &timeoutMockPinger{
		name:    "timeout",
		timeout: 20 * time.Millisecond,
		delay:   50 * time.Millisecond, // Should timeout
	}

	err = service.Register(timeoutPinger)
	if err != nil {
		t.Fatalf("register timeout pinger failed: %v", err)
	}

	// Let it run
	time.Sleep(250 * time.Millisecond)

	timeoutStats, err := service.GetStats("timeout")
	if err != nil {
		t.Fatalf("get timeout stats failed: %v", err)
	}

	// Should have timeout errors
	if timeoutStats.ErrorCount == 0 {
		t.Error("expected error count > 0 for timeout pinger")
	}

	if timeoutStats.LastError == nil {
		t.Error("expected last error to be set for timeout pinger")
	}

	cancel()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	_ = service.Shutdown(shutdownCtx)
}

// mockPinger is a test implementation of Pinger
type mockPinger struct {
	shouldError bool
	delay       time.Duration
	name        string
}

func (m *mockPinger) Name() string {
	if m.name != "" {
		return m.name
	}

	return "mock-pinger"
}

func (m *mockPinger) Ping(ctx context.Context) error {
	if m.delay > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(m.delay):
		}
	}

	if m.shouldError {
		return errors.New("mock pinger error")
	}

	return nil
}

// criticalMockPinger is a test implementation with critical flags
type criticalMockPinger struct {
	name           string
	readyCritical  bool
	healthCritical bool
	shouldError    bool
	delay          time.Duration
}

func (m *criticalMockPinger) Name() string {
	return m.name
}

func (m *criticalMockPinger) Ping(ctx context.Context) error {
	if m.delay > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(m.delay):
		}
	}

	if m.shouldError {
		return errors.New("critical mock pinger error")
	}

	return nil
}

func (m *criticalMockPinger) PingerReadyCritical() bool {
	return m.readyCritical
}

func (m *criticalMockPinger) PingerCritical() bool {
	return m.healthCritical
}

// timeoutMockPinger is a test implementation with custom timeout
type timeoutMockPinger struct {
	name    string
	timeout time.Duration
	delay   time.Duration
}

func (m *timeoutMockPinger) Name() string {
	return m.name
}

func (m *timeoutMockPinger) Ping(ctx context.Context) error {
	if m.delay > 0 {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(m.delay):
		}
	}

	return nil
}

func (m *timeoutMockPinger) PingerTimeout() time.Duration {
	return m.timeout
}
