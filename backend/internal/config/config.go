package config

import (
	"errors"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv                string
	HTTPAddr              string
	DatabaseURL           string
	AdminEmail            string
	AdminPassword         string
	AdminSessionTTL       time.Duration
	AdminLoginMaxAttempts int
	AdminLoginWindow      time.Duration
	APIRateLimit          int
	APIRateWindow         time.Duration
	TrustProxyHeaders     bool
	LLMProvider           string
	LLMAPIKey             string
	LLMAPIKeys            []string
	LLMModel              string
	LLMTimeout            time.Duration
	LogLevel              string
}

func Load() Config {
	seconds, err := strconv.Atoi(getenv("LLM_TIMEOUT_SECONDS", "15"))
	if err != nil || seconds <= 0 {
		seconds = 15
	}
	adminSessionTTL, err := time.ParseDuration(getenv("ADMIN_SESSION_TTL", "12h"))
	if err != nil || adminSessionTTL <= 0 {
		adminSessionTTL = 12 * time.Hour
	}
	adminLoginMaxAttempts, err := strconv.Atoi(getenv("ADMIN_LOGIN_MAX_ATTEMPTS", "5"))
	if err != nil || adminLoginMaxAttempts < 1 {
		adminLoginMaxAttempts = 5
	}
	adminLoginWindow, err := time.ParseDuration(getenv("ADMIN_LOGIN_WINDOW", "15m"))
	if err != nil || adminLoginWindow <= 0 {
		adminLoginWindow = 15 * time.Minute
	}
	apiRateLimit, err := strconv.Atoi(getenv("API_RATE_LIMIT", "120"))
	if err != nil || apiRateLimit < 1 {
		apiRateLimit = 120
	}
	apiRateWindow, err := time.ParseDuration(getenv("API_RATE_WINDOW", "1m"))
	if err != nil || apiRateWindow <= 0 {
		apiRateWindow = time.Minute
	}
	trustProxyHeaders, err := strconv.ParseBool(getenv("TRUST_PROXY_HEADERS", "false"))
	if err != nil {
		trustProxyHeaders = false
	}
	singleLLMKey := getenv("LLM_API_KEY", getenv("GEMINI_API_KEY", os.Getenv("GOOGLE_API_KEY")))
	llmKeys := parseAPIKeys(os.Getenv("LLM_API_KEYS"))
	if len(llmKeys) == 0 && strings.TrimSpace(singleLLMKey) != "" {
		llmKeys = []string{strings.TrimSpace(singleLLMKey)}
	}
	return Config{
		AppEnv:                strings.ToLower(strings.TrimSpace(getenv("APP_ENV", "development"))),
		HTTPAddr:              getenv("HTTP_ADDR", ":8080"),
		DatabaseURL:           os.Getenv("DATABASE_URL"),
		AdminEmail:            getenv("ADMIN_EMAIL", "admin@example.com"),
		AdminPassword:         getenv("ADMIN_PASSWORD", "change-me"),
		AdminSessionTTL:       adminSessionTTL,
		AdminLoginMaxAttempts: adminLoginMaxAttempts,
		AdminLoginWindow:      adminLoginWindow,
		APIRateLimit:          apiRateLimit,
		APIRateWindow:         apiRateWindow,
		TrustProxyHeaders:     trustProxyHeaders,
		LLMProvider:           getenv("LLM_PROVIDER", "mock"),
		LLMAPIKey:             singleLLMKey,
		LLMAPIKeys:            llmKeys,
		LLMModel:              getenv("LLM_MODEL", "gemini-2.5-flash"),
		LLMTimeout:            time.Duration(seconds) * time.Second,
		LogLevel:              strings.ToLower(strings.TrimSpace(getenv("LOG_LEVEL", "info"))),
	}
}

func (config Config) Validate() error {
	if config.AppEnv != "production" {
		return nil
	}
	if strings.TrimSpace(config.DatabaseURL) == "" {
		return errors.New("DATABASE_URL is required in production")
	}
	if config.AdminPassword == "change-me" || len(config.AdminPassword) < 12 {
		return errors.New("ADMIN_PASSWORD must contain at least 12 characters and must not use the default in production")
	}
	return nil
}

func parseAPIKeys(value string) []string {
	seen := make(map[string]bool)
	keys := make([]string, 0, 8)
	for _, item := range strings.Split(value, ",") {
		key := strings.TrimSpace(item)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		keys = append(keys, key)
	}
	return keys
}

func getenv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
