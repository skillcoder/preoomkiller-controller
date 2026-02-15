package shutdown_test

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/skillcoder/preoomkiller-controller/internal/infra/shutdown"
	"github.com/skillcoder/preoomkiller-controller/internal/infra/shutdown/mocks"
)

func TestCheckTerminationFile(t *testing.T) {
	t.Parallel()

	logger := slog.Default()

	t.Run("file missing returns false", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "nonexistent")

		got := shutdown.CheckTerminationFile(t.Context(), logger, path)
		require.False(t, got)
	})

	t.Run("file exists returns true", func(t *testing.T) {
		t.Parallel()

		dir := t.TempDir()
		path := filepath.Join(dir, "terminating")
		require.NoError(t, os.WriteFile(path, nil, 0o600))

		got := shutdown.CheckTerminationFile(t.Context(), logger, path)
		require.True(t, got)
	})
}

func TestGracefulShutdown(t *testing.T) {
	t.Parallel()

	logger := slog.Default()

	t.Run("empty list returns nil", func(t *testing.T) {
		t.Parallel()

		err := shutdown.GracefulShutdown(t.Context(), logger, nil)
		require.NoError(t, err)
	})

	t.Run("one shutdowner success returns nil", func(t *testing.T) {
		t.Parallel()

		m := mocks.NewMockShutdowner(t)
		m.EXPECT().Name().Return("test").Once()
		m.EXPECT().Shutdown(mock.Anything).Return(nil).Once()

		err := shutdown.GracefulShutdown(t.Context(), logger, []shutdown.Shutdowner{m})
		require.NoError(t, err)
	})

	t.Run("one shutdowner error returns error", func(t *testing.T) {
		t.Parallel()

		m := mocks.NewMockShutdowner(t)
		m.EXPECT().Name().Return("test").Once()
		m.EXPECT().Shutdown(mock.Anything).Return(context.DeadlineExceeded).Once()

		err := shutdown.GracefulShutdown(t.Context(), logger, []shutdown.Shutdowner{m})
		require.Error(t, err)
		require.ErrorIs(t, err, context.DeadlineExceeded)
	})

	t.Run("multiple shutdowners called in reverse order", func(t *testing.T) {
		t.Parallel()

		first := mocks.NewMockShutdowner(t)
		first.EXPECT().Name().Return("first").Once()
		first.EXPECT().Shutdown(mock.Anything).Return(nil).Once()

		second := mocks.NewMockShutdowner(t)
		second.EXPECT().Name().Return("second").Once()
		second.EXPECT().Shutdown(mock.Anything).Return(nil).Once()

		err := shutdown.GracefulShutdown(t.Context(), logger, []shutdown.Shutdowner{first, second})
		require.NoError(t, err)
	})
}
