package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestRegistryExportsPrometheusMetrics(t *testing.T) {
	registry := NewRegistry()
	registry.ObserveHTTP("POST", "/api/v1/sessions/{id}/messages", 200, 12*time.Millisecond)
	registry.ObserveLLM("reply", "fallback")
	registry.ObserveMove("ask_interest", "harvard_interests", "local", 2, 0, 0)
	registry.ObserveSessionStarted("vendor-introduction")
	registry.ObserveSessionCompleted("finished", "mutual_gain")

	recorder := httptest.NewRecorder()
	registry.ServeHTTP(recorder, httptest.NewRequest("GET", "/metrics", nil))
	body := recorder.Body.String()
	for _, expected := range []string{
		`arena_http_requests_total{method="POST",route="/api/v1/sessions/{id}/messages",status="200"} 1`,
		`arena_llm_operations_total{operation="reply",result="fallback"} 1`,
		`arena_negotiation_moves_total{intent="ask_interest",technique="harvard_interests",reply_source="local"} 1`,
		`arena_score_change_points_total{attribute="trust",direction="increase"} 2`,
		`arena_sessions_started_total{scenario="vendor-introduction"} 1`,
		`arena_sessions_completed_total{status="finished",outcome="mutual_gain"} 1`,
	} {
		if !strings.Contains(body, expected) {
			t.Fatalf("metric %q is missing from:\n%s", expected, body)
		}
	}
}

func TestRegistryIsSafeForConcurrentUpdates(t *testing.T) {
	registry := NewRegistry()
	done := make(chan struct{})
	for worker := 0; worker < 8; worker++ {
		go func() {
			for index := 0; index < 100; index++ {
				registry.ObserveLLM("reply", "fallback")
			}
			done <- struct{}{}
		}()
	}
	for worker := 0; worker < 8; worker++ {
		<-done
	}
	recorder := httptest.NewRecorder()
	registry.ServeHTTP(recorder, httptest.NewRequest("GET", "/metrics", nil))
	if !strings.Contains(recorder.Body.String(), `arena_llm_operations_total{operation="reply",result="fallback"} 800`) {
		t.Fatalf("concurrent updates were lost:\n%s", recorder.Body.String())
	}
}
