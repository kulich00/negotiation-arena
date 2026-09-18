package httpapi

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kulich00/negotiation-arena/backend/internal/config"
	"github.com/kulich00/negotiation-arena/backend/internal/domain"
	"github.com/kulich00/negotiation-arena/backend/internal/negotiation"
)

type Handler struct {
	service *negotiation.Service
	db      *pgxpool.Pool
	cfg     config.Config
	logger  *slog.Logger
}

func NewHandler(service *negotiation.Service, db *pgxpool.Pool, cfg config.Config, logger *slog.Logger) *Handler {
	return &Handler{service: service, db: db, cfg: cfg, logger: logger}
}

func (h *Handler) Router(static http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/live", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /health/ready", h.ready)
	mux.HandleFunc("GET /api/v1/scenarios", func(w http.ResponseWriter, _ *http.Request) { writeJSON(w, http.StatusOK, h.service.ListScenarios()) })
	mux.HandleFunc("POST /api/v1/sessions", h.startSession)
	mux.HandleFunc("GET /api/v1/sessions/{id}", h.getSession)
	mux.HandleFunc("POST /api/v1/sessions/{id}/messages", h.processMessage)
	mux.HandleFunc("POST /api/v1/sessions/{id}/finish", h.finishSession)
	mux.HandleFunc("GET /api/v1/sessions/{id}/result", h.getResult)
	mux.HandleFunc("POST /api/v1/admin/login", h.login)
	mux.HandleFunc("POST /api/v1/admin/scenarios", h.createScenario)
	mux.Handle("/", static)
	return h.recover(h.logRequests(mux))
}

func (h *Handler) ready(w http.ResponseWriter, r *http.Request) {
	if h.db != nil {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := h.db.Ping(ctx); err != nil {
			writeError(w, http.StatusServiceUnavailable, "database unavailable")
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
}

func (h *Handler) startSession(w http.ResponseWriter, r *http.Request) {
	var body struct {
		ScenarioID string `json:"scenarioId"`
	}
	if !decode(w, r, &body) {
		return
	}
	session, err := h.service.StartSession(body.ScenarioID)
	if err != nil {
		writeError(w, http.StatusNotFound, "scenario not found")
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

func (h *Handler) getSession(w http.ResponseWriter, r *http.Request) {
	session, err := h.service.Session(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *Handler) processMessage(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Content string `json:"content"`
	}
	if !decode(w, r, &body) {
		return
	}
	if strings.TrimSpace(body.Content) == "" {
		writeError(w, http.StatusBadRequest, "content is required")
		return
	}
	result, err := h.service.ProcessMessage(r.Context(), r.PathValue("id"), body.Content)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) finishSession(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Finish(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) getResult(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Result(r.PathValue("id"))
	if err != nil {
		writeError(w, http.StatusNotFound, "result not found")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var body struct{ Email, Password string }
	if !decode(w, r, &body) {
		return
	}
	if body.Email != h.cfg.AdminEmail || body.Password != h.cfg.AdminPassword {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"token": adminToken(h.cfg.SecretKey)})
}

func (h *Handler) createScenario(w http.ResponseWriter, r *http.Request) {
	token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
	if !validToken(token, h.cfg.SecretKey) {
		writeError(w, http.StatusUnauthorized, "invalid token")
		return
	}
	var scenario domain.Scenario
	if !decode(w, r, &scenario) {
		return
	}
	if scenario.Title == "" || scenario.InitialMessage == "" {
		writeError(w, http.StatusBadRequest, "title and initialMessage are required")
		return
	}
	writeJSON(w, http.StatusCreated, h.service.CreateScenario(scenario))
}

func decode(w http.ResponseWriter, r *http.Request, target any) bool {
	defer r.Body.Close()
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

func (h *Handler) logRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		h.logger.Info("request", "method", r.Method, "path", r.URL.Path, "duration", time.Since(start))
	})
}

func (h *Handler) recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if value := recover(); value != nil {
				h.logger.Error("panic recovered", "value", value)
				writeError(w, http.StatusInternalServerError, "internal error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}
