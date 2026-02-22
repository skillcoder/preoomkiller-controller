package config

import (
	"fmt"
	"os"
	"time"

	"github.com/skillcoder/preoomkiller-controller/internal/logic/controller"
)

type Config struct {
	KubeConfig                   string
	KubeMaster                   string
	Interval                     time.Duration
	PingerInterval               time.Duration
	LogLevel                     string
	LogFormat                    string
	HTTPPort                     string
	MetricsPort                  string
	PodLabelSelector             string
	AnnotationMemoryThresholdKey string
	AnnotationRestartScheduleKey string
	AnnotationTZKey              string
	RestartScheduleJitterMax     time.Duration
	MinPodAgeBeforeEviction      time.Duration
}

func Load() (*Config, error) {
	cfg := &Config{
		KubeConfig:       getEnvWithFallback(envKeyKubeConfig, envKeyKubeConfigFallback),
		KubeMaster:       getEnvWithFallback(envKeyKubeMaster, envKeyKubeMasterFallback),
		LogLevel:         getEnvOrDefault(envKeyLogLevel, "info"),
		LogFormat:        getEnvOrDefault(envKeyLogFormat, "json"),
		HTTPPort:         getEnvOrDefault(envKeyHTTPPort, "8080"),
		MetricsPort:      getEnvOrDefault(envKeyMetricsPort, "9090"),
		PodLabelSelector: getEnvOrDefault(envKeyPodLabelSelector, controller.PreoomkillerPodLabelSelector),
		AnnotationMemoryThresholdKey: getEnvOrDefault(
			envKeyAnnotationMemoryThreshold,
			controller.PreoomkillerAnnotationMemoryThresholdKey,
		),
		AnnotationRestartScheduleKey: getEnvOrDefault(
			envKeyAnnotationRestartSchedule,
			controller.PreoomkillerAnnotationRestartScheduleKey,
		),
		AnnotationTZKey: getEnvOrDefault(
			envKeyAnnotationTZ,
			controller.PreoomkillerAnnotationTZKey,
		),
	}

	var err error

	cfg.PingerInterval, err = parseDurationEnv(envKeyPingerInterval, "10s", envMinPingerInterval)
	if err != nil {
		return nil, fmt.Errorf("parse duration env: %s: %w", envKeyPingerInterval, err)
	}

	cfg.Interval, err = parseDurationEnv(envKeyInterval, "300s", envMinInterval)
	if err != nil {
		return nil, fmt.Errorf("parse duration env: %s: %w", envKeyInterval, err)
	}

	cfg.RestartScheduleJitterMax, err = parseDurationEnv(envKeyRestartScheduleJitterMax, "30s", envMinRestartScheduleJitterMax)
	if err != nil {
		return nil, fmt.Errorf("parse duration env: %s: %w", envKeyRestartScheduleJitterMax, err)
	}

	cfg.MinPodAgeBeforeEviction, err = parseDurationEnv(envKeyMinPodAgeBeforeEviction, "30m", envMinMinPodAgeBeforeEviction)
	if err != nil {
		return nil, fmt.Errorf("parse duration env: %s: %w", envKeyMinPodAgeBeforeEviction, err)
	}

	return cfg, nil
}

func parseDurationEnv(key, defaultVal string, minDuration time.Duration) (time.Duration, error) {
	s := getEnvOrDefault(key, defaultVal)

	d, err := time.ParseDuration(s)
	if err != nil {
		return 0, fmt.Errorf("parse duration: %w", err)
	}

	if d < minDuration {
		return 0, fmt.Errorf("value must be at least %s, got %s", minDuration.String(), d.String())
	}

	return d, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	return value
}

func getEnvWithFallback(primaryKey, fallbackKey string) string {
	if v := os.Getenv(primaryKey); v != "" {
		return v
	}

	return os.Getenv(fallbackKey)
}
