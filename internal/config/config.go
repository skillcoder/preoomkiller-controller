package config

import (
	"fmt"
	"os"
	"strconv"
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

	pingerIntervalSecondsStr := getEnvOrDefault(envKeyPingerIntervalSec, "10")

	pingerIntervalSeconds, err := strconv.Atoi(pingerIntervalSecondsStr)
	if err != nil {
		return nil, fmt.Errorf("parse pinger interval: %w", err)
	}

	cfg.PingerInterval = time.Duration(pingerIntervalSeconds) * time.Second

	intervalSecondsStr := getEnvOrDefault(envKeyIntervalSec, "300")

	intervalSeconds, err := strconv.Atoi(intervalSecondsStr)
	if err != nil {
		return nil, fmt.Errorf("parse interval: %w", err)
	}

	cfg.Interval = time.Duration(intervalSeconds) * time.Second

	jitterSecondsStr := getEnvOrDefault(envKeyRestartScheduleJitterMaxSec, "30")

	jitterSeconds, err := strconv.Atoi(jitterSecondsStr)
	if err != nil {
		return nil, fmt.Errorf("parse restart schedule jitter: %w", err)
	}

	cfg.RestartScheduleJitterMax = time.Duration(jitterSeconds) * time.Second

	minPodAge, err := parseMinPodAgeBeforeEvictionSec()
	if err != nil {
		return nil, err
	}

	cfg.MinPodAgeBeforeEviction = minPodAge

	return cfg, nil
}

func parseMinPodAgeBeforeEvictionSec() (time.Duration, error) {
	minPodAgeSecondsStr := getEnvOrDefault(envKeyMinPodAgeBeforeEvictionSec, "1800")

	minPodAgeSeconds, err := strconv.Atoi(minPodAgeSecondsStr)
	if err != nil {
		return 0, fmt.Errorf("parse min pod age before eviction: %w", err)
	}

	if minPodAgeSeconds < 0 {
		return 0, fmt.Errorf(
			"%s must be non-negative, got %d",
			envKeyMinPodAgeBeforeEvictionSec,
			minPodAgeSeconds,
		)
	}

	return time.Duration(minPodAgeSeconds) * time.Second, nil
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
