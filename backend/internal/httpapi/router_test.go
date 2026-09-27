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

func TestSecurityHeaders(t *testing.T) {
	service := negotiation.NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	router := NewHandler(service, nil, nil, slog.Default()).Router(http.NotFoundHandler())
	request := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}
	expected := map[string]string{
		"Content-Security-Policy":    "default-src 'self'; connect-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'",
		"Cross-Origin-Opener-Policy": "same-origin",
		"Permissions-Policy":         "camera=(), microphone=(), geolocation=()",
		"Referrer-Policy":            "no-referrer",
		"X-Content-Type-Options":     "nosniff",
		"X-Frame-Options":            "DENY",
	}
	for header, value := range expected {
		if response.Header().Get(header) != value {
			t.Errorf("%s = %q, want %q", header, response.Header().Get(header), value)
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
	if turn.Analysis.Technique != "harvard_interests" || turn.Analysis.TrustDelta != 2 {
		t.Fatalf("structured move analysis is missing: %+v", turn.Analysis)
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

func TestAdminScenarioManagement(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryRepository()
	service := negotiation.NewService(repo, llm.NewMockProvider())
	authService, token := newTestAdminAuth(t, repo)
	router := NewHandler(service, authService, nil, slog.Default()).Router(http.NotFoundHandler())

	adminRequest := func(method, path string, value any) *httptest.ResponseRecorder {
		var body []byte
		if value != nil {
			var err error
			body, err = json.Marshal(value)
			if err != nil {
				t.Fatal(err)
			}
		}
		request := httptest.NewRequest(method, path, bytes.NewReader(body))
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}

	scenario := validHTTPScenario("managed-scenario")
	if response := adminRequest(http.MethodPost, "/api/v1/admin/scenarios", scenario); response.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", response.Code, response.Body.String())
	}
	if response := adminRequest(http.MethodPost, "/api/v1/admin/scenarios", scenario); response.Code != http.StatusConflict {
		t.Fatalf("duplicate status %d: %s", response.Code, response.Body.String())
	}

	listResponse := adminRequest(http.MethodGet, "/api/v1/admin/scenarios", nil)
	if listResponse.Code != http.StatusOK {
		t.Fatalf("list status %d: %s", listResponse.Code, listResponse.Body.String())
	}
	var scenarios []domain.Scenario
	if err := json.Unmarshal(listResponse.Body.Bytes(), &scenarios); err != nil {
		t.Fatal(err)
	}
	if len(scenarios) != 1 || scenarios[0].OpponentGoal == "" || scenarios[0].Rules.MaxTurns == 0 {
		t.Fatalf("admin list omitted private fields: %+v", scenarios)
	}

	scenario.ID = ""
	scenario.Title = "Updated through API"
	updateResponse := adminRequest(http.MethodPut, "/api/v1/admin/scenarios/managed-scenario", scenario)
	if updateResponse.Code != http.StatusOK {
		t.Fatalf("update status %d: %s", updateResponse.Code, updateResponse.Body.String())
	}
	var updated domain.Scenario
	if err := json.Unmarshal(updateResponse.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.ID != "managed-scenario" || updated.Title != "Updated through API" {
		t.Fatalf("unexpected update response: %+v", updated)
	}

	if _, err := service.StartSession(ctx, updated.ID); err != nil {
		t.Fatal(err)
	}
	if response := adminRequest(http.MethodPut, "/api/v1/admin/scenarios/managed-scenario", scenario); response.Code != http.StatusConflict {
		t.Fatalf("active update status %d: %s", response.Code, response.Body.String())
	}
	if response := adminRequest(http.MethodDelete, "/api/v1/admin/scenarios/managed-scenario", nil); response.Code != http.StatusConflict {
		t.Fatalf("used delete status %d: %s", response.Code, response.Body.String())
	}

	deletable := validHTTPScenario("deletable-scenario")
	if response := adminRequest(http.MethodPost, "/api/v1/admin/scenarios", deletable); response.Code != http.StatusCreated {
		t.Fatalf("second create status %d: %s", response.Code, response.Body.String())
	}
	if response := adminRequest(http.MethodDelete, "/api/v1/admin/scenarios/deletable-scenario", nil); response.Code != http.StatusNoContent {
		t.Fatalf("delete status %d: %s", response.Code, response.Body.String())
	}

	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/admin/scenarios", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized list status %d: %s", unauthorized.Code, unauthorized.Body.String())
	}
}

func TestAdminLoginAndLogout(t *testing.T) {
	repo := repository.NewMemoryRepository()
	service := negotiation.NewService(repo, llm.NewMockProvider())
	authService := newAdminAuthService(repo, time.Hour)
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

func TestAdminLoginRateLimit(t *testing.T) {
	repo := repository.NewMemoryRepository()
	service := negotiation.NewService(repo, llm.NewMockProvider())
	authService := adminauth.NewService(repo, adminauth.Config{
		SessionTTL:       time.Hour,
		MaxLoginAttempts: 1,
		LoginWindow:      time.Minute,
	})
	if err := authService.Bootstrap(context.Background(), "admin@example.com", "correct-password"); err != nil {
		t.Fatal(err)
	}
	router := NewHandler(service, authService, nil, slog.Default()).Router(http.NotFoundHandler())

	request := func(password string) *httptest.ResponseRecorder {
		body := strings.NewReader(`{"email":"admin@example.com","password":"` + password + `"}`)
		httpRequest := httptest.NewRequest(http.MethodPost, "/api/v1/admin/login", body)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httpRequest)
		return response
	}
	if response := request("wrong-password"); response.Code != http.StatusUnauthorized {
		t.Fatalf("first login status %d: %s", response.Code, response.Body.String())
	}
	response := request("correct-password")
	if response.Code != http.StatusTooManyRequests {
		t.Fatalf("limited login status %d: %s", response.Code, response.Body.String())
	}
	if response.Header().Get("Retry-After") == "" || response.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("missing rate-limit headers: %+v", response.Header())
	}
}

func newTestAdminAuth(t *testing.T, repo *repository.MemoryRepository) (*adminauth.Service, string) {
	t.Helper()
	service := newAdminAuthService(repo, time.Hour)
	if err := service.Bootstrap(context.Background(), "admin@example.com", "correct-password"); err != nil {
		t.Fatal(err)
	}
	session, err := service.Login(context.Background(), "admin@example.com", "correct-password")
	if err != nil {
		t.Fatal(err)
	}
	return service, session.Token
}

func newAdminAuthService(repo repository.AdminRepository, sessionTTL time.Duration) *adminauth.Service {
	return adminauth.NewService(repo, adminauth.Config{
		SessionTTL:       sessionTTL,
		MaxLoginAttempts: 5,
		LoginWindow:      15 * time.Minute,
	})
}

func validHTTPScenario(id string) domain.Scenario {
	return domain.Scenario{
		ID:             id,
		Title:          "Project negotiation",
		Sphere:         "IT",
		Topic:          "Delivery date",
		Difficulty:     "medium",
		OpponentRole:   "Customer",
		OpponentTone:   "Demanding",
		PlayerGoal:     "Agree on a realistic date",
		OpponentGoal:   "Reduce delivery risk",
		InitialMessage: "Why should the deadline change?",
	}
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
