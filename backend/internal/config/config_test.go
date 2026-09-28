package config

import (
	"testing"
	"time"
)

func TestLoadAdminSecuritySettings(t *testing.T) {
	t.Setenv("ADMIN_SESSION_TTL", "2h")
	t.Setenv("ADMIN_LOGIN_MAX_ATTEMPTS", "3")
	t.Setenv("ADMIN_LOGIN_WINDOW", "7m")

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
}

func TestLoadAdminSecuritySettingsUsesSafeDefaults(t *testing.T) {
	t.Setenv("ADMIN_SESSION_TTL", "invalid")
	t.Setenv("ADMIN_LOGIN_MAX_ATTEMPTS", "0")
	t.Setenv("ADMIN_LOGIN_WINDOW", "-1m")

	config := Load()
	if config.AdminSessionTTL != 12*time.Hour || config.AdminLoginMaxAttempts != 5 || config.AdminLoginWindow != 15*time.Minute {
		t.Fatalf("unexpected defaults: %+v", config)
	}
}

func TestLoadLLMSettingsAndDefaults(t *testing.T) {
	t.Setenv("LLM_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "gemini-key")
	t.Setenv("LLM_MODEL", "")
	t.Setenv("LLM_TIMEOUT_SECONDS", "0")

	config := Load()
	if config.LLMAPIKey != "gemini-key" {
		t.Fatalf("LLMAPIKey was not loaded from GEMINI_API_KEY")
	}
	if config.LLMModel != "gemini-2.5-flash" || config.LLMTimeout != 15*time.Second {
		t.Fatalf("unexpected LLM defaults: model=%q timeout=%s", config.LLMModel, config.LLMTimeout)
	}
}
