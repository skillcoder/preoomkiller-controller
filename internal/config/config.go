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
	PodLabelSelector             string
	AnnotationMemoryThresholdKey string
}

func Load() (*Config, error) {
	cfg := &Config{
		KubeConfig:       os.Getenv("KUBECONFIG"),
		KubeMaster:       os.Getenv("KUBERNETES_MASTER"),
		LogLevel:         getEnvOrDefault("LOG_LEVEL", "info"),
		LogFormat:        getEnvOrDefault("LOG_FORMAT", "json"),
		HTTPPort:         getEnvOrDefault("HTTP_PORT", "8080"),
		PodLabelSelector: getEnvOrDefault("PREOOMKILLER_POD_LABEL_SELECTOR", controller.PreoomkillerPodLabelSelector),
		AnnotationMemoryThresholdKey: getEnvOrDefault(
			"PREOOMKILLER_ANNOTATION_MEMORY_THRESHOLD",
			controller.PreoomkillerAnnotationMemoryThresholdKey,
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

	return cfg, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	return value
}
