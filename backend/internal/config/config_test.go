package config

import (
	"testing"
	"time"
)

func TestLoadAdminSecuritySettings(t *testing.T) {
	t.Setenv("ADMIN_SESSION_TTL", "2h")
	t.Setenv("ADMIN_LOGIN_MAX_ATTEMPTS", "3")
	t.Setenv("ADMIN_LOGIN_WINDOW", "7m")
	t.Setenv("API_RATE_LIMIT", "45")
	t.Setenv("API_RATE_WINDOW", "30s")

	config := Load()
	if config.AdminSessionTTL != 2*time.Hour {
		t.Fatalf("AdminSessionTTL = %v", config.AdminSessionTTL)
	}
	if config.AdminLoginMaxAttempts != 3 {
		t.Fatalf("AdminLoginMaxAttempts = %d", config.AdminLoginMaxAttempts)
	}
	if config.AdminLoginWindow != 7*time.Minute {
		t.Fatalf("AdminLoginWindow = %v", config.AdminLoginWindow)
	}
	if config.APIRateLimit != 45 || config.APIRateWindow != 30*time.Second {
		t.Fatalf("unexpected API rate limit: %d per %s", config.APIRateLimit, config.APIRateWindow)
	}
}

func TestLoadAdminSecuritySettingsUsesSafeDefaults(t *testing.T) {
	t.Setenv("ADMIN_SESSION_TTL", "invalid")
	t.Setenv("ADMIN_LOGIN_MAX_ATTEMPTS", "0")
	t.Setenv("ADMIN_LOGIN_WINDOW", "-1m")
	t.Setenv("API_RATE_LIMIT", "0")
	t.Setenv("API_RATE_WINDOW", "invalid")

	config := Load()
	if config.AdminSessionTTL != 12*time.Hour || config.AdminLoginMaxAttempts != 5 || config.AdminLoginWindow != 15*time.Minute || config.APIRateLimit != 120 || config.APIRateWindow != time.Minute {
		t.Fatalf("unexpected defaults: %+v", config)
	}
}

func TestLoadLLMSettingsAndDefaults(t *testing.T) {
	t.Setenv("LLM_API_KEYS", "")
	t.Setenv("LLM_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "gemini-key")
	t.Setenv("LLM_MODEL", "")
	t.Setenv("LLM_TIMEOUT_SECONDS", "0")

	config := Load()
	if config.LLMAPIKey != "gemini-key" {
		t.Fatalf("LLMAPIKey was not loaded from GEMINI_API_KEY")
	}
	if len(config.LLMAPIKeys) != 1 || config.LLMAPIKeys[0] != "gemini-key" {
		t.Fatalf("LLMAPIKeys fallback = %v", config.LLMAPIKeys)
	}
	if config.LLMModel != "gemini-2.5-flash" || config.LLMTimeout != 15*time.Second {
		t.Fatalf("unexpected LLM defaults: model=%q timeout=%s", config.LLMModel, config.LLMTimeout)
	}
}

func TestLoadLLMKeyPoolTakesPriorityAndRemovesDuplicates(t *testing.T) {
	t.Setenv("LLM_API_KEYS", " key-1, key-2, key-1, ,key-3 ")
	t.Setenv("LLM_API_KEY", "legacy-key")

	config := Load()
	want := []string{"key-1", "key-2", "key-3"}
	if len(config.LLMAPIKeys) != len(want) {
		t.Fatalf("LLMAPIKeys = %v, want %v", config.LLMAPIKeys, want)
	}
	for index := range want {
		if config.LLMAPIKeys[index] != want[index] {
			t.Fatalf("LLMAPIKeys = %v, want %v", config.LLMAPIKeys, want)
		}
	}
}

func TestLoadOperationalSettings(t *testing.T) {
	t.Setenv("APP_ENV", " Production ")
	t.Setenv("LOG_LEVEL", " WARN ")
	t.Setenv("TRUST_PROXY_HEADERS", "true")

	config := Load()
	if config.AppEnv != "production" {
		t.Fatalf("AppEnv = %q", config.AppEnv)
	}
	if config.LogLevel != "warn" {
		t.Fatalf("LogLevel = %q", config.LogLevel)
	}
	if !config.TrustProxyHeaders {
		t.Fatal("TrustProxyHeaders = false, want true")
	}
}

func TestProductionConfigValidation(t *testing.T) {
	tests := []struct {
		name    string
		config  Config
		wantErr bool
	}{
		{
			name: "valid production config",
			config: Config{
				AppEnv:        "production",
				DatabaseURL:   "postgres://arena:arena@database/arena",
				AdminPassword: "strong-password",
			},
		},
		{
			name: "database is required",
			config: Config{
				AppEnv:        "production",
				AdminPassword: "strong-password",
			},
			wantErr: true,
		},
		{
			name: "default password is rejected",
			config: Config{
				AppEnv:        "production",
				DatabaseURL:   "postgres://arena:arena@database/arena",
				AdminPassword: "change-me",
			},
			wantErr: true,
		},
		{
			name: "short password is rejected",
			config: Config{
				AppEnv:        "production",
				DatabaseURL:   "postgres://arena:arena@database/arena",
				AdminPassword: "too-short",
			},
			wantErr: true,
		},
		{
			name: "development allows local defaults",
			config: Config{
				AppEnv:        "development",
				AdminPassword: "change-me",
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.config.Validate()
			if (err != nil) != test.wantErr {
				t.Fatalf("Validate() error = %v, wantErr %v", err, test.wantErr)
			}
		})
	}
}
