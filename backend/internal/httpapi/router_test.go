package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kulich00/negotiation-arena/backend/internal/config"
	"github.com/kulich00/negotiation-arena/backend/internal/llm"
	"github.com/kulich00/negotiation-arena/backend/internal/negotiation"
	"github.com/kulich00/negotiation-arena/backend/internal/repository"
)

func TestScenarioListOmitsPrivateRules(t *testing.T) {
	service := negotiation.NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	if err := service.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(service, nil, config.Config{}, slog.Default())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/scenarios", nil)
	response := httptest.NewRecorder()
	handler.Router(http.NotFoundHandler()).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}
	var scenarios []map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &scenarios); err != nil {
		t.Fatal(err)
	}
	if len(scenarios) != 2 {
		t.Fatalf("expected two scenarios, got %d", len(scenarios))
	}
	for _, scenario := range scenarios {
		if _, ok := scenario["rules"]; ok {
			t.Fatal("private rules leaked in public response")
		}
		if _, ok := scenario["opponentGoal"]; ok {
			t.Fatal("opponent goal leaked in public response")
		}
		if scenario["id"] == nil || scenario["title"] == nil {
			t.Fatalf("missing public fields: %+v", scenario)
		}
	}
}
