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
	if len(scenarios) != 3 {
		t.Fatalf("expected three scenarios, got %d", len(scenarios))
	}
	for _, scenario := range scenarios {
		if _, ok := scenario["rules"]; ok {
			t.Fatal("private rules leaked in public response")
		}
		if _, ok := scenario["opponentGoal"]; ok {
			t.Fatal("opponent goal leaked in public response")
		}
		if scenario["id"] == nil || scenario["title"] == nil || scenario["opponentMode"] == nil {
			t.Fatalf("missing public fields: %+v", scenario)
		}
		if scenario["id"] == "project-deadline" && scenario["opponentMode"] != negotiation.OpponentModeDifficult {
			t.Fatalf("difficult mode is not exposed: %+v", scenario)
		}
	}
}

func TestAchievementCatalog(t *testing.T) {
	service := negotiation.NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	router := NewHandler(service, nil, nil, slog.Default()).Router(http.NotFoundHandler())
	request := httptest.NewRequest(http.MethodGet, "/api/v1/achievements", nil)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status %d: %s", response.Code, response.Body.String())
	}
	var achievements []domain.Achievement
	if err := json.Unmarshal(response.Body.Bytes(), &achievements); err != nil {
		t.Fatal(err)
	}
	if len(achievements) != 9 || achievements[0].Code != negotiation.AchievementFirstRound {
		t.Fatalf("unexpected achievement catalog: %+v", achievements)
	}
}

func TestPlayerProfileAndDifficultyGate(t *testing.T) {
	service := negotiation.NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	if err := service.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	router := NewHandler(service, nil, nil, slog.Default()).Router(http.NotFoundHandler())

	createRequest := httptest.NewRequest(http.MethodPost, "/api/v1/players", bytes.NewBufferString(`{"displayName":"Мария"}`))
	createResponse := httptest.NewRecorder()
	router.ServeHTTP(createResponse, createRequest)
	if createResponse.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", createResponse.Code, createResponse.Body.String())
	}
	var player domain.PlayerProfile
	if err := json.Unmarshal(createResponse.Body.Bytes(), &player); err != nil {
		t.Fatal(err)
	}
	if player.ID == "" || player.DisplayName != "Мария" || player.UnlockedDifficulty != negotiation.DifficultyEasy {
		t.Fatalf("unexpected player: %+v", player)
	}

	lockedBody := []byte(`{"scenarioId":"salary-negotiation","playerId":"` + player.ID + `"}`)
	lockedRequest := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewReader(lockedBody))
	lockedResponse := httptest.NewRecorder()
	router.ServeHTTP(lockedResponse, lockedRequest)
	if lockedResponse.Code != http.StatusForbidden {
		t.Fatalf("locked status %d: %s", lockedResponse.Code, lockedResponse.Body.String())
	}

	easyBody := []byte(`{"scenarioId":"vendor-introduction","playerId":"` + player.ID + `"}`)
	easyRequest := httptest.NewRequest(http.MethodPost, "/api/v1/sessions", bytes.NewReader(easyBody))
	easyResponse := httptest.NewRecorder()
	router.ServeHTTP(easyResponse, easyRequest)
	if easyResponse.Code != http.StatusCreated {
		t.Fatalf("easy status %d: %s", easyResponse.Code, easyResponse.Body.String())
	}
	var session domain.Session
	if err := json.Unmarshal(easyResponse.Body.Bytes(), &session); err != nil {
		t.Fatal(err)
	}
	if session.PlayerID != player.ID {
		t.Fatalf("session is not linked to player: %+v", session)
	}

	getRequest := httptest.NewRequest(http.MethodGet, "/api/v1/players/"+player.ID, nil)
	getResponse := httptest.NewRecorder()
	router.ServeHTTP(getResponse, getRequest)
	if getResponse.Code != http.StatusOK {
		t.Fatalf("get status %d: %s", getResponse.Code, getResponse.Body.String())
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
	checkpointRequest := httptest.NewRequest(http.MethodGet, "/api/v1/sessions/"+sessionID+"/checkpoints", nil)
	checkpointResponse := httptest.NewRecorder()
	router.ServeHTTP(checkpointResponse, checkpointRequest)
	if checkpointResponse.Code != http.StatusOK {
		t.Fatalf("checkpoint status %d: %s", checkpointResponse.Code, checkpointResponse.Body.String())
	}
	var checkpoints []domain.TurnCheckpoint
	if err := json.Unmarshal(checkpointResponse.Body.Bytes(), &checkpoints); err != nil {
		t.Fatal(err)
	}
	if len(checkpoints) != 2 || checkpoints[0].Turn != 0 || checkpoints[1].Turn != 1 || !checkpoints[1].State.InterestsExplored {
		t.Fatalf("unexpected checkpoints: %+v", checkpoints)
	}
}

func TestProcessMoveValidationError(t *testing.T) {
	service := negotiation.NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	if err := service.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	router := NewHandler(service, nil, nil, slog.Default()).Router(http.NotFoundHandler())
	sessionID := startTestSession(t, router, "salary-negotiation")

	body := []byte(`{"content":"Предлагаю 31%","intent":"propose","proposal":{"kind":"raise_percent","value":31}}`)
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

func TestAbandonSessionEndpointIsIdempotent(t *testing.T) {
	service := negotiation.NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	if err := service.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	router := NewHandler(service, nil, nil, slog.Default()).Router(http.NotFoundHandler())
	sessionID := startTestSession(t, router, "salary-negotiation")

	for attempt := 0; attempt < 2; attempt++ {
		request := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+sessionID+"/abandon", nil)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		if response.Code != http.StatusOK {
			t.Fatalf("attempt %d status %d: %s", attempt+1, response.Code, response.Body.String())
		}
		var result domain.Result
		if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
			t.Fatal(err)
		}
		if result.OutcomeCode != "abandoned" || result.SessionID != sessionID {
			t.Fatalf("unexpected abandoned result: %+v", result)
		}
	}
}

func TestForkSessionEndpoint(t *testing.T) {
	service := negotiation.NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	if err := service.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	router := NewHandler(service, nil, nil, slog.Default()).Router(http.NotFoundHandler())
	parentID := startTestSession(t, router, "salary-negotiation")
	if _, err := service.ProcessMove(context.Background(), parentID, negotiation.PlayerMove{Content: "Что для вас важно?", Intent: negotiation.IntentAskInterest}); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+parentID+"/fork", strings.NewReader(`{"turn":0}`))
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusCreated {
		t.Fatalf("fork status %d: %s", response.Code, response.Body.String())
	}
	var child domain.Session
	if err := json.Unmarshal(response.Body.Bytes(), &child); err != nil {
		t.Fatal(err)
	}
	if child.ParentSessionID != parentID || child.ForkedFromTurn == nil || *child.ForkedFromTurn != 0 || child.Turn != 0 || child.TrustScore != 50 {
		t.Fatalf("unexpected forked session: %+v", child)
	}
	messages, err := service.Messages(context.Background(), child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 1 {
		t.Fatalf("unexpected forked messages: %+v", messages)
	}

	invalidRequest := httptest.NewRequest(http.MethodPost, "/api/v1/sessions/"+parentID+"/fork", strings.NewReader(`{"turn":7}`))
	invalidResponse := httptest.NewRecorder()
	router.ServeHTTP(invalidResponse, invalidRequest)
	if invalidResponse.Code != http.StatusBadRequest {
		t.Fatalf("invalid fork status %d: %s", invalidResponse.Code, invalidResponse.Body.String())
	}
}

func TestAdminSessionHistoryAndStatistics(t *testing.T) {
	repo := repository.NewMemoryRepository()
	service := negotiation.NewService(repo, llm.NewMockProvider())
	if err := service.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	authService, token := newTestAdminAuth(t, repo)
	router := NewHandler(service, authService, nil, slog.Default()).Router(http.NotFoundHandler())
	activeSessionID := startTestSession(t, router, "salary-negotiation")
	abandonedSessionID := startTestSession(t, router, "salary-negotiation")
	if _, err := service.Abandon(context.Background(), abandonedSessionID); err != nil {
		t.Fatal(err)
	}

	adminGet := func(path string) *httptest.ResponseRecorder {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		request.Header.Set("Authorization", "Bearer "+token)
		response := httptest.NewRecorder()
		router.ServeHTTP(response, request)
		return response
	}

	pageResponse := adminGet("/api/v1/admin/sessions?status=abandoned&scenarioId=salary-negotiation&limit=1&offset=0")
	if pageResponse.Code != http.StatusOK {
		t.Fatalf("history status %d: %s", pageResponse.Code, pageResponse.Body.String())
	}
	var page domain.SessionPage
	if err := json.Unmarshal(pageResponse.Body.Bytes(), &page); err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != abandonedSessionID || page.Items[0].FinishedAt == nil {
		t.Fatalf("unexpected history page: %+v", page)
	}

	detailResponse := adminGet("/api/v1/admin/sessions/" + abandonedSessionID)
	if detailResponse.Code != http.StatusOK {
		t.Fatalf("detail status %d: %s", detailResponse.Code, detailResponse.Body.String())
	}
	var detail domain.SessionDetail
	if err := json.Unmarshal(detailResponse.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Session.ID != abandonedSessionID || detail.Result == nil || detail.Result.OutcomeCode != "abandoned" || len(detail.Messages) != 1 || len(detail.Checkpoints) != 1 || detail.Scenario.Rules.MaxTurns == 0 {
		t.Fatalf("unexpected session detail: %+v", detail)
	}

	statisticsResponse := adminGet("/api/v1/admin/session-statistics?scenarioId=salary-negotiation")
	if statisticsResponse.Code != http.StatusOK {
		t.Fatalf("statistics status %d: %s", statisticsResponse.Code, statisticsResponse.Body.String())
	}
	var statistics domain.SessionStatistics
	if err := json.Unmarshal(statisticsResponse.Body.Bytes(), &statistics); err != nil {
		t.Fatal(err)
	}
	if statistics.Total != 2 || statistics.Active != 1 || statistics.Abandoned != 1 || len(statistics.Outcomes) != 1 {
		t.Fatalf("unexpected session statistics: %+v", statistics)
	}

	activeDetailResponse := adminGet("/api/v1/admin/sessions/" + activeSessionID)
	if activeDetailResponse.Code != http.StatusOK {
		t.Fatalf("active detail status %d: %s", activeDetailResponse.Code, activeDetailResponse.Body.String())
	}
	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/sessions", nil)
	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, request)
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized history status %d: %s", unauthorized.Code, unauthorized.Body.String())
	}
}

func TestAdminCorpusExport(t *testing.T) {
	repo := repository.NewMemoryRepository()
	service := negotiation.NewService(repo, llm.NewMockProvider())
	if err := service.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	authService, token := newTestAdminAuth(t, repo)
	router := NewHandler(service, authService, nil, slog.Default()).Router(http.NotFoundHandler())
	sessionID := startTestSession(t, router, "vendor-introduction")
	if _, err := service.ProcessMove(context.Background(), sessionID, negotiation.PlayerMove{
		Content: "Что для вас важно?", Intent: negotiation.IntentAskInterest,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Abandon(context.Background(), sessionID); err != nil {
		t.Fatal(err)
	}

	request := httptest.NewRequest(http.MethodGet, "/api/v1/admin/corpus?status=abandoned&format=jsonl", nil)
	request.Header.Set("Authorization", "Bearer "+token)
	response := httptest.NewRecorder()
	router.ServeHTTP(response, request)
	if response.Code != http.StatusOK || response.Header().Get("Content-Type") != "application/x-ndjson; charset=utf-8" || response.Header().Get("X-Corpus-Schema-Version") != domain.CorpusSchemaVersion {
		t.Fatalf("corpus response: status=%d headers=%v body=%s", response.Code, response.Header(), response.Body.String())
	}
	lines := strings.Split(strings.TrimSpace(response.Body.String()), "\n")
	if len(lines) != 1 {
		t.Fatalf("expected one JSONL record, got %d", len(lines))
	}
	var dialogue domain.CorpusDialogue
	if err := json.Unmarshal([]byte(lines[0]), &dialogue); err != nil {
		t.Fatal(err)
	}
	if dialogue.DialogueID != sessionID || len(dialogue.Turns) != 1 || dialogue.Turns[0].Analysis.ReplySource != "local" {
		t.Fatalf("unexpected exported dialogue: %+v", dialogue)
	}
	encoded := response.Body.String()
	if strings.Contains(encoded, `"playerId"`) {
		t.Fatalf("corpus leaked player identifier: %s", encoded)
	}

	unauthorized := httptest.NewRecorder()
	router.ServeHTTP(unauthorized, httptest.NewRequest(http.MethodGet, "/api/v1/admin/corpus", nil))
	if unauthorized.Code != http.StatusUnauthorized {
		t.Fatalf("unauthorized corpus status %d", unauthorized.Code)
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
