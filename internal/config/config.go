package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	KubeConfig     string
	KubeMaster     string
	Interval       time.Duration
	PingerInterval time.Duration
	LogLevel       string
	LogFormat      string
	HTTPPort       string
}

func Load() (*Config, error) {
	cfg := &Config{
		KubeConfig: os.Getenv("KUBECONFIG"),
		KubeMaster: os.Getenv("KUBERNETES_MASTER"),
		LogLevel:   getEnvOrDefault("LOG_LEVEL", "info"),
		LogFormat:  getEnvOrDefault("LOG_FORMAT", "json"),
		HTTPPort:   getEnvOrDefault("HTTP_PORT", "8080"),
	}

	pingerIntervalStr := getEnvOrDefault("PINGER_INTERVAL", "10")

	pingerInterval, err := strconv.Atoi(pingerIntervalStr)
	if err != nil {
		return nil, fmt.Errorf("parse pinger interval: %w", err)
	}

	cfg.PingerInterval = time.Duration(pingerInterval) * time.Second

	intervalStr := getEnvOrDefault("INTERVAL", "300")

	interval, err := strconv.Atoi(intervalStr)
	if err != nil {
		return nil, fmt.Errorf("parse interval: %w", err)
	}

	cfg.Interval = time.Duration(interval) * time.Second

	return cfg, nil
}

func getEnvOrDefault(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}

	return value
}
