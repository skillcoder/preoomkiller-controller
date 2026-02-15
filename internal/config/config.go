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
		KubeConfig:       os.Getenv("KUBECONFIG"),
		KubeMaster:       os.Getenv("KUBERNETES_MASTER"),
		LogLevel:         getEnvOrDefault("LOG_LEVEL", "info"),
		LogFormat:        getEnvOrDefault("LOG_FORMAT", "json"),
		HTTPPort:         getEnvOrDefault("HTTP_PORT", "8080"),
		MetricsPort:      getEnvOrDefault("METRICS_PORT", "9090"),
		PodLabelSelector: getEnvOrDefault("PREOOMKILLER_POD_LABEL_SELECTOR", controller.PreoomkillerPodLabelSelector),
		AnnotationMemoryThresholdKey: getEnvOrDefault(
			"PREOOMKILLER_ANNOTATION_MEMORY_THRESHOLD",
			controller.PreoomkillerAnnotationMemoryThresholdKey,
		),
		AnnotationRestartScheduleKey: getEnvOrDefault(
			"PREOOMKILLER_ANNOTATION_RESTART_SCHEDULE",
			controller.PreoomkillerAnnotationRestartScheduleKey,
		),
		AnnotationTZKey: getEnvOrDefault(
			"PREOOMKILLER_ANNOTATION_TZ",
			controller.PreoomkillerAnnotationTZKey,
		),
	}

	pingerIntervalSecondsStr := getEnvOrDefault("PINGER_INTERVAL", "10")

	pingerIntervalSeconds, err := strconv.Atoi(pingerIntervalSecondsStr)
	if err != nil {
		return nil, fmt.Errorf("parse pinger interval: %w", err)
	}

	cfg.PingerInterval = time.Duration(pingerIntervalSeconds) * time.Second

	intervalSecondsStr := getEnvOrDefault("INTERVAL", "300")

	intervalSeconds, err := strconv.Atoi(intervalSecondsStr)
	if err != nil {
		return nil, fmt.Errorf("parse interval: %w", err)
	}

	cfg.Interval = time.Duration(intervalSeconds) * time.Second

	jitterSecondsStr := getEnvOrDefault("PREOOMKILLER_RESTART_SCHEDULE_JITTER_MAX", "30")

	jitterSeconds, err := strconv.Atoi(jitterSecondsStr)
	if err != nil {
		return nil, fmt.Errorf("parse restart schedule jitter: %w", err)
	}

	cfg.RestartScheduleJitterMax = time.Duration(jitterSeconds) * time.Second

	minPodAge, err := parseMinPodAgeBeforeEviction()
	if err != nil {
		return nil, err
	}

	cfg.MinPodAgeBeforeEviction = minPodAge

	return cfg, nil
}

func parseMinPodAgeBeforeEviction() (time.Duration, error) {
	minPodAgeMinutesStr := getEnvOrDefault("PREOOMKILLER_MIN_POD_AGE_BEFORE_EVICTION", "30")

	minPodAgeMinutes, err := strconv.Atoi(minPodAgeMinutesStr)
	if err != nil {
		return 0, fmt.Errorf("parse min pod age before eviction: %w", err)
	}

	if minPodAgeMinutes < 0 {
		return 0, fmt.Errorf(
			"PREOOMKILLER_MIN_POD_AGE_BEFORE_EVICTION must be non-negative, got %d",
			minPodAgeMinutes,
		)
	}

	return time.Duration(minPodAgeMinutes) * time.Minute, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	return value
}
