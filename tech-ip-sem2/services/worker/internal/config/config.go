package config

import (
	"os"
	"strconv"
)

type Config struct {
	RabbitMQURL       string
	MainQueue         string
	RetryQueue        string
	DLQ               string
	PrefetchCount     int
	MaxAttempts       int
	RetryDelaySeconds int
	LogLevel          string
}

func Load() *Config {
	return &Config{
		RabbitMQURL:       getEnv("RABBITMQ_URL", "amqp://guest:guest@localhost:5672/"),
		MainQueue:         getEnv("MAIN_QUEUE", "task_jobs"),
		RetryQueue:        getEnv("RETRY_QUEUE", "task_jobs_retry"),
		DLQ:               getEnv("DLQ", "task_jobs_dlq"),
		PrefetchCount:     getEnvAsInt("RABBITMQ_PREFETCH", 1),
		MaxAttempts:       getEnvAsInt("MAX_ATTEMPTS", 3),
		RetryDelaySeconds: getEnvAsInt("RETRY_DELAY_SECONDS", 10),
		LogLevel:          getEnv("LOG_LEVEL", "info"),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}
