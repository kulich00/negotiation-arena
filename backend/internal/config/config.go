package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	HTTPAddr        string
	DatabaseURL     string
	AdminEmail      string
	AdminPassword   string
	AdminSessionTTL time.Duration
	LLMProvider     string
	LLMAPIKey       string
	LLMModel        string
	LLMTimeout      time.Duration
}

func Load() Config {
	seconds, _ := strconv.Atoi(getenv("LLM_TIMEOUT_SECONDS", "15"))
	adminSessionTTL, err := time.ParseDuration(getenv("ADMIN_SESSION_TTL", "12h"))
	if err != nil || adminSessionTTL <= 0 {
		adminSessionTTL = 12 * time.Hour
	}
	return Config{
		HTTPAddr:        getenv("HTTP_ADDR", ":8080"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		AdminEmail:      getenv("ADMIN_EMAIL", "admin@example.com"),
		AdminPassword:   getenv("ADMIN_PASSWORD", "change-me"),
		AdminSessionTTL: adminSessionTTL,
		LLMProvider:     getenv("LLM_PROVIDER", "mock"),
		LLMAPIKey:       os.Getenv("LLM_API_KEY"),
		LLMModel:        os.Getenv("LLM_MODEL"),
		LLMTimeout:      time.Duration(seconds) * time.Second,
	}
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
