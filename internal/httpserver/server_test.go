package httpserver_test

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/skillcoder/preoomkiller-controller/internal/httpserver"
	"github.com/skillcoder/preoomkiller-controller/internal/infra/appstate"
	"github.com/skillcoder/preoomkiller-controller/internal/infra/pinger"
)

func TestNew(t *testing.T) {
	t.Parallel()

	logger := slog.Default()

	quit := make(chan os.Signal, 1)

	quit <- syscall.SIGTERM

	close(quit)

	pingerSvc := pinger.New(logger, time.Second)
	appState := appstate.New(logger, time.Now(), "", quit, pingerSvc)

	t.Run("empty port uses default", func(t *testing.T) {
		t.Parallel()

		srv := httpserver.New(logger, appState, "")
		require.NotNil(t, srv)
	})

	t.Run("non-empty port is used", func(t *testing.T) {
		t.Parallel()

		srv := httpserver.New(logger, appState, "9090")
		require.NotNil(t, srv)
	})
}

func TestServer_Name(t *testing.T) {
	t.Parallel()

	logger := slog.Default()
	quit := make(chan os.Signal, 1)
	pingerSvc := pinger.New(logger, time.Second)
	appState := appstate.New(logger, time.Now(), "", quit, pingerSvc)
	srv := httpserver.New(logger, appState, "")

	require.Equal(t, "http-server", srv.Name())
}

func TestServer_Ping(t *testing.T) {
	t.Parallel()

	logger := slog.Default()

	t.Run("before ready returns error", func(t *testing.T) {
		t.Parallel()

		quit := make(chan os.Signal, 1)
		pingerSvc := pinger.New(logger, time.Second)
		appState := appstate.New(logger, time.Now(), "", quit, pingerSvc)
		srv := httpserver.New(logger, appState, "")

		err := srv.Ping(t.Context())
		require.Error(t, err)
	})

	t.Run("after ready returns nil", func(t *testing.T) {
		t.Parallel()

		quit := make(chan os.Signal, 1)

		signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

		pingerSvc := pinger.New(logger, time.Second)
		appState := appstate.New(logger, time.Now(), "", quit, pingerSvc)
		require.NoError(t, appState.SetStarting(t.Context()))
		require.NoError(t, appState.SetRunning(t.Context()))

		srv := httpserver.New(logger, appState, "0")

		ctx, cancel := context.WithTimeout(t.Context(), 2*time.Second)

		defer cancel()

		require.NoError(t, srv.Start(ctx))

		select {
		case <-srv.Ready():
		case <-time.After(1 * time.Second):
			t.Fatal("server did not become ready")
		}

		require.NoError(t, srv.Ping(t.Context()))

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer shutdownCancel()

		_ = srv.Shutdown(shutdownCtx)
	})
}
