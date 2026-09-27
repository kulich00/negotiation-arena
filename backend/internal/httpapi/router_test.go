package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/kulich00/negotiation-arena/backend/internal/adminauth"
	"github.com/kulich00/negotiation-arena/backend/internal/domain"
	"github.com/kulich00/negotiation-arena/backend/internal/llm"
	"github.com/kulich00/negotiation-arena/backend/internal/negotiation"
	"github.com/kulich00/negotiation-arena/backend/internal/repository"
)

func TestScenarioListOmitsPrivateRules(t *testing.T) {
	service := negotiation.NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	if err := service.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	handler := NewHandler(service, nil, nil, slog.Default())
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

func TestProcessStructuredMove(t *testing.T) {
	service := negotiation.NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	if err := service.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	router := NewHandler(service, nil, nil, slog.Default()).Router(http.NotFoundHandler())
	sessionID := startTestSession(t, router, "salary-negotiation")

	body := []byte(`{"content":"Что для вас важно?","intent":"ask_interest"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sessionID+"/messages", bytes.NewReader(body))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}

	var turn negotiation.TurnResult
	if err := json.Unmarshal(response.Body.Bytes(), &turn); err != nil {
		t.Fatal(err)
	}
	if turn.Session.TrustScore != 52 || !turn.Session.State.InterestsExplored {
		t.Fatalf("structured move was not evaluated: %+v", turn.Session)
	}
	if turn.Reply != "Для меня важно снизить риски и понять взаимную выгоду. Какие варианты вы предлагаете?" {
		t.Fatalf("unexpected deterministic reply: %q", turn.Reply)
	}
}

func TestProcessMoveValidationError(t *testing.T) {
	service := negotiation.NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	if err := service.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	router := NewHandler(service, nil, nil, slog.Default()).Router(http.NotFoundHandler())
	sessionID := startTestSession(t, router, "salary-negotiation")

	body := []byte(`{"content":"Предлагаю 20%","intent":"propose","proposal":{"kind":"raise_percent","value":20}}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sessionID+"/messages", bytes.NewReader(body))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}
	var apiError map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &apiError); err != nil {
		t.Fatal(err)
	}
	if apiError["error"] != "invalid move" {
		t.Fatalf("private validation details leaked: %s", response.Body.String())
	}
}

func TestProcessLegacyMessageRemainsSupported(t *testing.T) {
	service := negotiation.NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	if err := service.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	router := NewHandler(service, nil, nil, slog.Default()).Router(http.NotFoundHandler())
	sessionID := startTestSession(t, router, "salary-negotiation")

	request := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sessionID+"/messages", bytes.NewReader([]byte(`{"content":"Какие условия возможны?"}`)))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}
}

func TestJSONBodyMustContainOneKnownObject(t *testing.T) {
	service := negotiation.NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	if err := service.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	router := NewHandler(service, nil, nil, slog.Default()).Router(http.NotFoundHandler())
	sessionID := startTestSession(t, router, "salary-negotiation")

	tests := []struct {
		name string
		body string
	}{
		{name: "unknown field", body: `{"content":"Hello","unexpected":true}`},
		{name: "second object", body: `{"content":"Hello"} {"content":"Again"}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sessionID+"/messages", strings.NewReader(test.body))
			response := httptest.NewRecorder()
			router.ServeHTTP(response, request)
			if response.Code != http.StatusBadRequest {
				t.Fatalf("status %d: %s", response.Code, response.Body.String())
			}
			var apiError map[string]string
			if err := json.Unmarshal(response.Body.Bytes(), &apiError); err != nil {
				t.Fatal(err)
			}
			if apiError["error"] != "invalid JSON" {
				t.Fatalf("unexpected error: %s", response.Body.String())
			}
		})
	}
}

func TestCreateScenarioRejectsInvalidPayload(t *testing.T) {
	repo := repository.NewMemoryRepository()
	service := negotiation.NewService(repo, llm.NewMockProvider())
	authService, token := newTestAdminAuth(t, repo)
	router := NewHandler(service, authService, nil, slog.Default()).Router(http.NotFoundHandler())

	body := []byte(`{"title":"Incomplete scenario"}`)
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/scenarios", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}
	var apiError map[string]string
	if err := json.Unmarshal(response.Body.Bytes(), &apiError); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(apiError["error"], "invalid scenario:") {
		t.Fatalf("unexpected validation error: %s", response.Body.String())
	}
	scenarios, err := service.ListScenarios(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(scenarios) != 0 {
		t.Fatalf("invalid scenario was saved: %+v", scenarios)
	}
}

func TestCreateScenarioAppliesDefaults(t *testing.T) {
	repo := repository.NewMemoryRepository()
	service := negotiation.NewService(repo, llm.NewMockProvider())
	authService, token := newTestAdminAuth(t, repo)
	router := NewHandler(service, authService, nil, slog.Default()).Router(http.NotFoundHandler())
	scenario := domain.Scenario{
		Title:          "  Project deadline  ",
		Sphere:         "IT",
		Topic:          "Delivery date",
		Difficulty:     " MEDIUM ",
		OpponentRole:   "Customer",
		OpponentTone:   "Demanding",
		PlayerGoal:     "Agree on a realistic date",
		OpponentGoal:   "Reduce delivery risk",
		InitialMessage: "Why should the deadline change?",
	}
	body, err := json.Marshal(scenario)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/admin/scenarios", bytes.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}
	var created domain.Scenario
	if err := json.Unmarshal(response.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.Title != "Project deadline" || created.Difficulty != "medium" {
		t.Fatalf("scenario was not normalized: %+v", created)
	}
	if !strings.HasPrefix(created.ID, "project-deadline-") {
		t.Fatalf("unexpected generated id: %q", created.ID)
	}
	defaults := domain.DefaultScenarioRules()
	if created.Rules.MaxTurns != defaults.MaxTurns ||
		created.Rules.MinimumTrustForAgreement != defaults.MinimumTrustForAgreement ||
		created.Rules.Proposal.Kind != defaults.Proposal.Kind ||
		len(created.Rules.Proposal.AlternativeIDs) != 0 {
		t.Fatalf("default rules were not applied: %+v", created.Rules)
	}
}

func TestAdminLoginAndLogout(t *testing.T) {
	repo := repository.NewMemoryRepository()
	service := negotiation.NewService(repo, llm.NewMockProvider())
	authService := adminauth.NewService(repo, time.Hour)
	if err := authService.Bootstrap(context.Background(), "admin@example.com", "correct-password"); err != nil {
		t.Fatal(err)
	}
	router := NewHandler(service, authService, nil, slog.Default()).Router(http.NotFoundHandler())

	loginRequest := httptest.NewRequest(http.MethodPost, "/api/v1/admin/login", strings.NewReader(`{"email":"ADMIN@example.com","password":"correct-password"}`))
	loginResponse := httptest.NewRecorder()
	router.ServeHTTP(loginResponse, loginRequest)
	if loginResponse.Code != http.StatusOK {
		t.Fatalf("login status %d: %s", loginResponse.Code, loginResponse.Body.String())
	}
	var session adminauth.Session
	if err := json.Unmarshal(loginResponse.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	if session.Token == "" || session.ExpiresAt.IsZero() {
		t.Fatalf("invalid login response: %+v", session)
	}

	logoutRequest := httptest.NewRequest(http.MethodPost, "/api/v1/admin/logout", nil)
	logoutRequest.Header.Set("Authorization", "Bearer "+session.Token)
	logoutResponse := httptest.NewRecorder()
	router.ServeHTTP(logoutResponse, logoutRequest)
	if logoutResponse.Code != http.StatusNoContent {
		t.Fatalf("logout status %d: %s", logoutResponse.Code, logoutResponse.Body.String())
	}
	if err := authService.Authenticate(context.Background(), session.Token); !errors.Is(err, adminauth.ErrInvalidToken) {
		t.Fatalf("expected token revocation, got %v", err)
	}
}

func newTestAdminAuth(t *testing.T, repo *repository.MemoryRepository) (*adminauth.Service, string) {
	t.Helper()
	service := adminauth.NewService(repo, time.Hour)
	if err := service.Bootstrap(context.Background(), "admin@example.com", "correct-password"); err != nil {
		t.Fatal(err)
	}
	session, err := service.Login(context.Background(), "admin@example.com", "correct-password")
	if err != nil {
		t.Fatal(err)
	}
	return service, session.Token
}

func startTestSession(t *testing.T, handler http.Handler, scenarioID string) string {
	t.Helper()
	body, err := json.Marshal(map[string]string{"scenarioId": scenarioID})
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewReader(body))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("start session status %d: %s", response.Code, response.Body.String())
	}
	var session struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	return session.ID
}
