package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/skillcoder/preoomkiller-controller/internal/config"
	"github.com/skillcoder/preoomkiller-controller/internal/logic/controller"
)

type loadCase struct {
	name    string
	giveEnv map[string]string
	wantErr bool
	wantCfg *config.Config
}

func assertConfigFields(t *testing.T, got, want *config.Config) {
	t.Helper()

	if want == nil {
		return
	}

	if want.HTTPPort != "" {
		require.Equal(t, want.HTTPPort, got.HTTPPort)
	}

	if want.Interval != 0 {
		require.Equal(t, want.Interval, got.Interval)
	}

	if want.PingerInterval != 0 {
		require.Equal(t, want.PingerInterval, got.PingerInterval)
	}

	if want.LogLevel != "" {
		require.Equal(t, want.LogLevel, got.LogLevel)
	}

	if want.LogFormat != "" {
		require.Equal(t, want.LogFormat, got.LogFormat)
	}

	if want.PodLabelSelector != "" {
		require.Equal(t, want.PodLabelSelector, got.PodLabelSelector)
	}

	if want.AnnotationMemoryThresholdKey != "" {
		require.Equal(t, want.AnnotationMemoryThresholdKey, got.AnnotationMemoryThresholdKey)
	}

	if want.AnnotationRestartScheduleKey != "" {
		require.Equal(t, want.AnnotationRestartScheduleKey, got.AnnotationRestartScheduleKey)
	}

	if want.AnnotationTZKey != "" {
		require.Equal(t, want.AnnotationTZKey, got.AnnotationTZKey)
	}

	if want.RestartScheduleJitterMax != 0 {
		require.Equal(t, want.RestartScheduleJitterMax, got.RestartScheduleJitterMax)
	}

	if want.MetricsPort != "" {
		require.Equal(t, want.MetricsPort, got.MetricsPort)
	}

	if want.MinPodAgeBeforeEviction != 0 {
		require.Equal(t, want.MinPodAgeBeforeEviction, got.MinPodAgeBeforeEviction)
	}
}

func TestLoad(t *testing.T) {
	tests := []loadCase{
		{
			name: "all defaults",
			giveEnv: map[string]string{
				"PREOOMKILLER_INTERVAL_SEC":        "300",
				"PREOOMKILLER_PINGER_INTERVAL_SEC": "10",
			},
			wantErr: false,
			wantCfg: &config.Config{
				LogLevel:                     "info",
				LogFormat:                    "json",
				HTTPPort:                     "8080",
				MetricsPort:                  "9090",
				PodLabelSelector:             controller.PreoomkillerPodLabelSelector,
				AnnotationMemoryThresholdKey: controller.PreoomkillerAnnotationMemoryThresholdKey,
				AnnotationRestartScheduleKey: controller.PreoomkillerAnnotationRestartScheduleKey,
				AnnotationTZKey:              controller.PreoomkillerAnnotationTZKey,
				RestartScheduleJitterMax:     30 * time.Second,
				MinPodAgeBeforeEviction:      1800 * time.Second,
				Interval:                     300 * time.Second,
				PingerInterval:               10 * time.Second,
			},
		},
		{
			name: "override PREOOMKILLER_HTTP_PORT and PREOOMKILLER_INTERVAL_SEC",
			giveEnv: map[string]string{
				"PREOOMKILLER_HTTP_PORT":    "9090",
				"PREOOMKILLER_INTERVAL_SEC": "60",
			},
			wantErr: false,
			wantCfg: &config.Config{
				HTTPPort:  "9090",
				Interval:  60 * time.Second,
				LogLevel:  "info",
				LogFormat: "json",
			},
		},
		{
			name: "override PREOOMKILLER_PINGER_INTERVAL_SEC",
			giveEnv: map[string]string{
				"PREOOMKILLER_PINGER_INTERVAL_SEC": "5",
			},
			wantErr: false,
			wantCfg: &config.Config{
				PingerInterval: 5 * time.Second,
			},
		},
		{
			name: "invalid PREOOMKILLER_INTERVAL_SEC",
			giveEnv: map[string]string{
				"PREOOMKILLER_INTERVAL_SEC": "x",
			},
			wantErr: true,
		},
		{
			name: "invalid PREOOMKILLER_PINGER_INTERVAL_SEC",
			giveEnv: map[string]string{
				"PREOOMKILLER_PINGER_INTERVAL_SEC": "not-a-number",
			},
			wantErr: true,
		},
		{
			name: "invalid PREOOMKILLER_RESTART_SCHEDULE_JITTER_MAX_SEC",
			giveEnv: map[string]string{
				"PREOOMKILLER_INTERVAL_SEC":                    "300",
				"PREOOMKILLER_PINGER_INTERVAL_SEC":             "10",
				"PREOOMKILLER_RESTART_SCHEDULE_JITTER_MAX_SEC": "x",
			},
			wantErr: true,
		},
		{
			name: "override PREOOMKILLER_METRICS_PORT and PREOOMKILLER_MIN_POD_AGE_BEFORE_EVICTION_SEC",
			giveEnv: map[string]string{
				"PREOOMKILLER_INTERVAL_SEC":                    "300",
				"PREOOMKILLER_PINGER_INTERVAL_SEC":             "10",
				"PREOOMKILLER_METRICS_PORT":                    "9091",
				"PREOOMKILLER_MIN_POD_AGE_BEFORE_EVICTION_SEC": "900",
			},
			wantErr: false,
			wantCfg: &config.Config{
				MetricsPort:             "9091",
				MinPodAgeBeforeEviction: 900 * time.Second,
			},
		},
		{
			name: "invalid PREOOMKILLER_MIN_POD_AGE_BEFORE_EVICTION_SEC",
			giveEnv: map[string]string{
				"PREOOMKILLER_INTERVAL_SEC":                    "300",
				"PREOOMKILLER_PINGER_INTERVAL_SEC":             "10",
				"PREOOMKILLER_MIN_POD_AGE_BEFORE_EVICTION_SEC": "x",
			},
			wantErr: true,
		},
		{
			name: "negative PREOOMKILLER_MIN_POD_AGE_BEFORE_EVICTION_SEC",
			giveEnv: map[string]string{
				"PREOOMKILLER_INTERVAL_SEC":                    "300",
				"PREOOMKILLER_PINGER_INTERVAL_SEC":             "10",
				"PREOOMKILLER_MIN_POD_AGE_BEFORE_EVICTION_SEC": "-1",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.giveEnv {
				t.Setenv(k, v)
			}

			got, err := config.Load()
			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			require.NotNil(t, got)

			assertConfigFields(t, got, tt.wantCfg)
		})
	}
}
