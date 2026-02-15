package cronparser_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/skillcoder/preoomkiller-controller/internal/infra/cronparser"
)

func TestParser_NextAfter(t *testing.T) {
	t.Parallel()

	p := cronparser.New()

	t.Run("standard spec returns next occurrence", func(t *testing.T) {
		t.Parallel()

		after := time.Date(2026, 2, 15, 7, 0, 0, 0, time.UTC)
		next, err := p.NextAfter("40 7 * * *", "", after)
		require.NoError(t, err)
		require.True(t, next.After(after))
		require.Equal(t, 7, next.Hour())
		require.Equal(t, 40, next.Minute())
	})

	t.Run("with tz uses timezone", func(t *testing.T) {
		t.Parallel()

		after := time.Date(2026, 2, 15, 12, 0, 0, 0, time.UTC)
		next, err := p.NextAfter("0 8 * * *", "America/New_York", after)
		require.NoError(t, err)
		require.True(t, next.After(after))
	})

	t.Run("inline CRON_TZ ignores tz param", func(t *testing.T) {
		t.Parallel()

		after := time.Date(2026, 2, 15, 12, 0, 0, 0, time.UTC)
		next, err := p.NextAfter("CRON_TZ=UTC 0 14 * * *", "America/New_York", after)
		require.NoError(t, err)
		require.True(t, next.After(after))
		require.Equal(t, 14, next.Hour())
	})

	t.Run("malformed spec returns error", func(t *testing.T) {
		t.Parallel()

		_, err := p.NextAfter("invalid", "", time.Now())
		require.Error(t, err)
	})
}
