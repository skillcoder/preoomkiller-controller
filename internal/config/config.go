package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	KubeConfig string
	KubeMaster string
	Interval   time.Duration
	LogLevel   string
	LogFormat  string
}

func Load() (*Config, error) {
	cfg := &Config{
		KubeConfig: os.Getenv("KUBECONFIG"),
		KubeMaster: os.Getenv("KUBERNETES_MASTER"),
		LogLevel:   getEnvOrDefault("LOG_LEVEL", "info"),
		LogFormat:  getEnvOrDefault("LOG_FORMAT", "json"),
	}

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
