package appstate_test

import (
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/skillcoder/preoomkiller-controller/internal/infra/appstate"
	"github.com/skillcoder/preoomkiller-controller/internal/infra/pinger"
)

func TestAppState_StateTransitions(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	t.Run("init to starting", func(t *testing.T) {
		ctx := t.Context()
		pingerService := pinger.New(logger, 1*time.Second)
		s := appstate.New(logger, time.Now(), "/mnt/signal/terminating", quit, pingerService)
		require.NoError(t, s.SetStarting(ctx))
		require.Equal(t, appstate.StateStarting, s.GetState())
	})

	t.Run("starting to running", func(t *testing.T) {
		ctx := t.Context()
		pingerService := pinger.New(logger, 1*time.Second)
		s := appstate.New(logger, time.Now(), "/mnt/signal/terminating", quit, pingerService)
		require.NoError(t, s.SetStarting(ctx))
		require.NoError(t, s.SetRunning(ctx))
		require.Equal(t, appstate.StateRunning, s.GetState())
	})

	t.Run("running to terminating", func(t *testing.T) {
		ctx := t.Context()
		pingerService := pinger.New(logger, 1*time.Second)
		s := appstate.New(logger, time.Now(), "/mnt/signal/terminating", quit, pingerService)
		require.NoError(t, s.SetStarting(ctx))
		require.NoError(t, s.SetRunning(ctx))
		require.NoError(t, s.SetTerminating(ctx))
		require.Equal(t, appstate.StateTerminating, s.GetState())
	})

	t.Run("invalid: init to running", func(t *testing.T) {
		ctx := t.Context()
		pingerService := pinger.New(logger, 1*time.Second)
		s := appstate.New(logger, time.Now(), "/mnt/signal/terminating", quit, pingerService)
		err := s.SetRunning(ctx)
		require.Error(t, err)
		require.Equal(t, appstate.StateInit, s.GetState())
	})

	t.Run("invalid: terminated cannot change", func(t *testing.T) {
		ctx := t.Context()
		logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
		quit := make(chan os.Signal, 1)
		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		pingerService := pinger.New(logger, 1*time.Second)
		s := appstate.New(logger, time.Now(), "/mnt/signal/terminating", quit, pingerService)
		require.NoError(t, s.SetStarting(ctx))
		require.NoError(t, s.SetRunning(ctx))
		require.NoError(t, s.SetTerminating(ctx))
		require.NoError(t, s.Shutdown(ctx))
		require.Equal(t, appstate.StateTerminated, s.GetState())

		err := s.SetStarting(ctx)
		require.Error(t, err)
		require.Equal(t, appstate.StateTerminated, s.GetState())
	})
}

func TestAppState_QueryMethods(t *testing.T) {
	ctx := t.Context()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	startTime := time.Now()
	pingerService := pinger.New(logger, 1*time.Second)
	s := appstate.New(logger, startTime, "/mnt/signal/terminating", quit, pingerService)

	require.Equal(t, appstate.StateInit, s.GetState())
	require.Equal(t, startTime, s.GetStartTime())
	require.False(t, s.IsHealthy())
	require.False(t, s.IsReady())

	require.NoError(t, s.SetStarting(ctx))
	require.False(t, s.IsReady())

	require.NoError(t, s.SetRunning(ctx))
	require.True(t, s.IsHealthy())
	require.True(t, s.IsReady())
}

func TestAppState_GetUptime(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	startTime := time.Now()
	pingerService := pinger.New(logger, 1*time.Second)
	s := appstate.New(logger, startTime, "/mnt/signal/terminating", quit, pingerService)

	// Small delay to ensure uptime is non-zero
	time.Sleep(10 * time.Millisecond)

	uptime := s.GetUptime()
	require.Greater(t, uptime, time.Duration(0))
	require.Less(t, uptime, 100*time.Millisecond) // Should be close to our sleep time
}

func TestAppState_Shutdown(t *testing.T) {
	ctx := t.Context()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	pingerService := pinger.New(logger, 1*time.Second)
	s := appstate.New(logger, time.Now(), "/mnt/signal/terminating", quit, pingerService)

	require.NoError(t, s.SetStarting(ctx))
	require.NoError(t, s.SetRunning(ctx))
	require.NoError(t, s.SetTerminating(ctx))

	require.NoError(t, s.Shutdown(ctx))
	require.Equal(t, appstate.StateTerminated, s.GetState())

	// Shutdown again should be idempotent
	require.NoError(t, s.Shutdown(ctx))
	require.Equal(t, appstate.StateTerminated, s.GetState())
}
