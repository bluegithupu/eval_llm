package config

import (
	"os"
	"strconv"
	"time"
)

// Config holds application configuration
type Config struct {
	Server     ServerConfig
	Database   DatabaseConfig
	Redis      RedisConfig
	Kubernetes KubernetesConfig
	Evaluation EvaluationConfig
}

// ServerConfig holds server configuration
type ServerConfig struct {
	Port    int
	Timeout time.Duration
}

// DatabaseConfig holds database configuration
type DatabaseConfig struct {
	Host           string
	Port           int
	Name           string
	User           string
	Password       string
	MaxConnections int
}

// RedisConfig holds Redis configuration
type RedisConfig struct {
	Host string
	Port int
	TTL  time.Duration
}

// KubernetesConfig holds Kubernetes configuration
type KubernetesConfig struct {
	Namespace  string
	JobTimeout time.Duration
	JobRetries int
}

// EvaluationConfig holds evaluation configuration
type EvaluationConfig struct {
	ContainerImage string
	WorkDir        string
}

// Load reads configuration from environment variables
func Load() *Config {
	return &Config{
		Server: ServerConfig{
			Port:    getEnvInt("API_PORT", 3100),
			Timeout: getEnvDuration("API_TIMEOUT", 30*time.Second),
		},
		Database: DatabaseConfig{
			Host:           getEnv("DB_HOST", "localhost"),
			Port:           getEnvInt("DB_PORT", 3105),
			Name:           getEnv("DB_NAME", "evaluations"),
			User:           getEnv("DB_USER", "eval_user"),
			Password:       getEnv("DB_PASSWORD", "eval_pass"),
			MaxConnections: getEnvInt("DB_MAX_CONNECTIONS", 25),
		},
		Redis: RedisConfig{
			Host: getEnv("REDIS_HOST", "localhost"),
			Port: getEnvInt("REDIS_PORT", 3106),
			TTL:  getEnvDuration("REDIS_TTL", 24*time.Hour),
		},
		Kubernetes: KubernetesConfig{
			Namespace:  getEnv("K8S_NAMESPACE", "llm-eval"),
			JobTimeout: getEnvDuration("K8S_JOB_TIMEOUT", 2*time.Hour),
			JobRetries: getEnvInt("K8S_JOB_RETRIES", 3),
		},
		Evaluation: EvaluationConfig{
			ContainerImage: getEnv("EVAL_CONTAINER_IMAGE", "opencompass:latest"),
			WorkDir:        getEnv("EVAL_WORK_DIR", "/tmp/opencompass_runs"),
		},
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intVal, err := strconv.Atoi(value); err == nil {
			return intVal
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			return duration
		}
	}
	return defaultValue
}
