package httpapi

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kulich00/negotiation-arena/backend/internal/llm"
	arenametrics "github.com/kulich00/negotiation-arena/backend/internal/metrics"
	"github.com/kulich00/negotiation-arena/backend/internal/negotiation"
	"github.com/kulich00/negotiation-arena/backend/internal/repository"
)

func TestMetricsEndpointUsesStableRouteLabels(t *testing.T) {
	registry := arenametrics.NewRegistry()
	repo := repository.NewMemoryRepository()
	service := negotiation.NewService(repo, llm.NewMockProvider(), negotiation.WithMetrics(registry))
	if err := service.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	router := NewHandler(service, nil, nil, slog.Default(), WithMetrics(registry)).Router(http.NotFoundHandler())

	health := httptest.NewRecorder()
	router.ServeHTTP(health, httptest.NewRequest(http.MethodGet, "/health/live", nil))
	sessionID := startTestSession(t, router, "vendor-introduction")
	move := httptest.NewRecorder()
	router.ServeHTTP(move, httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sessionID+"/messages", strings.NewReader(`{"content":"Что важно?","intent":"ask_interest"}`)))
	if move.Code != http.StatusOK {
		t.Fatalf("move status %d: %s", move.Code, move.Body.String())
	}

	response := httptest.NewRecorder()
	router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, "/metrics", nil))
	body := response.Body.String()
	for _, expected := range []string{
		`route="/health/live"`,
		`route="/api/v1/sessions/{id}/messages"`,
		`arena_sessions_started_total{scenario="vendor-introduction"} 1`,
		`arena_negotiation_moves_total{intent="ask_interest",technique="harvard_interests",reply_source="local"} 1`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("metric %q missing from:\n%s", expected, body)
		}
	}
	if strings.Contains(body, sessionID) {
		t.Fatalf("session ID leaked into metric labels: %s", body)
	}
}

func TestPublicMutationRateLimit(t *testing.T) {
	repo := repository.NewMemoryRepository()
	service := negotiation.NewService(repo, llm.NewMockProvider())
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	router := NewHandler(service, nil, nil, logger, WithRateLimit(2, time.Minute)).Router(http.NotFoundHandler())

	for attempt := 1; attempt <= 3; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/players", bytes.NewBufferString(`{}`))
		request.RemoteAddr = "192.0.2.10:1234"
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)

		if response.Header().Get("X-Request-ID") == "" {
			t.Fatal("response has no request id")
		}
		if attempt < 3 && response.Code != http.StatusCreated {
			t.Fatalf("attempt %d status = %d: %s", attempt, response.Code, response.Body.String())
		}
		if attempt == 3 {
			if response.Code != http.StatusTooManyRequests {
				t.Fatalf("rate-limited status = %d: %s", response.Code, response.Body.String())
			}
			if response.Header().Get("Retry-After") == "" || response.Header().Get("X-RateLimit-Remaining") != "0" {
				t.Fatalf("missing rate-limit headers: %v", response.Header())
			}
		}
	}

	healthRequest := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	healthRequest.RemoteAddr = "192.0.2.10:1234"
	healthResponse := httptest.NewRecorder()
	router.ServeHTTP(healthResponse, healthRequest)
	if healthResponse.Code != http.StatusOK {
		t.Fatalf("health endpoint was rate-limited: %d", healthResponse.Code)
	}
}

func TestRecoveredPanicIsLoggedAsInternalServerError(t *testing.T) {
	var logs bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logs, nil))
	router := NewHandler(nil, nil, nil, logger).Router(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("test panic")
	}))
	request := httptest.NewRequest(http.MethodGet, "/unexpected", nil)
	response := httptest.NewRecorder()

	router.ServeHTTP(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusInternalServerError)
	}
	if !strings.Contains(logs.String(), `"status":500`) {
		t.Fatalf("request log has no 500 status: %s", logs.String())
	}
}

func TestFixedWindowLimiterResets(t *testing.T) {
	limiter := newFixedWindowLimiter(1, time.Minute)
	now := time.Date(2026, time.September, 28, 12, 0, 0, 0, time.UTC)
	if allowed, _, _ := limiter.allow("client", now); !allowed {
		t.Fatal("first request was rejected")
	}
	if allowed, _, _ := limiter.allow("client", now.Add(time.Second)); allowed {
		t.Fatal("request over the limit was allowed")
	}
	if allowed, remaining, _ := limiter.allow("client", now.Add(time.Minute)); !allowed || remaining != 0 {
		t.Fatalf("new window was not opened: allowed=%v remaining=%d", allowed, remaining)
	}
}

func TestRateLimiterUsesForwardedClientAddress(t *testing.T) {
	request, err := http.NewRequestWithContext(context.Background(), http.MethodPost, "http://example.test/api/v1/players", nil)
	if err != nil {
		t.Fatal(err)
	}
	request.RemoteAddr = "10.0.0.1:1234"
	request.Header.Set("X-Forwarded-For", "203.0.113.8, 10.0.0.1")
	if got := clientAddress(request, true); got != "203.0.113.8" {
		t.Fatalf("clientAddress = %q", got)
	}
	if got := clientAddress(request, false); got != "10.0.0.1" {
		t.Fatalf("untrusted forwarded clientAddress = %q", got)
	}
}
