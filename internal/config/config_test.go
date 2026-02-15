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
}

func TestLoad(t *testing.T) {
	tests := []loadCase{
		{
			name: "all defaults",
			giveEnv: map[string]string{
				"INTERVAL":        "300",
				"PINGER_INTERVAL": "10",
			},
			wantErr: false,
			wantCfg: &config.Config{
				LogLevel:                     "info",
				LogFormat:                    "json",
				HTTPPort:                     "8080",
				PodLabelSelector:             controller.PreoomkillerPodLabelSelector,
				AnnotationMemoryThresholdKey: controller.PreoomkillerAnnotationMemoryThresholdKey,
				AnnotationRestartScheduleKey: controller.PreoomkillerAnnotationRestartScheduleKey,
				AnnotationTZKey:              controller.PreoomkillerAnnotationTZKey,
				RestartScheduleJitterMax:     30 * time.Second,
				Interval:                     300 * time.Second,
				PingerInterval:               10 * time.Second,
			},
		},
		{
			name: "override HTTP_PORT and INTERVAL",
			giveEnv: map[string]string{
				"HTTP_PORT": "9090",
				"INTERVAL":  "60",
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
			name: "override PINGER_INTERVAL",
			giveEnv: map[string]string{
				"PINGER_INTERVAL": "5",
			},
			wantErr: false,
			wantCfg: &config.Config{
				PingerInterval: 5 * time.Second,
			},
		},
		{
			name: "invalid INTERVAL",
			giveEnv: map[string]string{
				"INTERVAL": "x",
			},
			wantErr: true,
		},
		{
			name: "invalid PINGER_INTERVAL",
			giveEnv: map[string]string{
				"PINGER_INTERVAL": "not-a-number",
			},
			wantErr: true,
		},
		{
			name: "invalid PREOOMKILLER_RESTART_SCHEDULE_JITTER_MAX",
			giveEnv: map[string]string{
				"INTERVAL":        "300",
				"PINGER_INTERVAL": "10",
				"PREOOMKILLER_RESTART_SCHEDULE_JITTER_MAX": "x",
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
