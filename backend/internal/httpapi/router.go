package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kulich00/negotiation-arena/backend/internal/config"
	"github.com/kulich00/negotiation-arena/backend/internal/domain"
	"github.com/kulich00/negotiation-arena/backend/internal/negotiation"
	"github.com/kulich00/negotiation-arena/backend/internal/repository"
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
	mux.HandleFunc("GET /api/v1/scenarios", h.listScenarios)
	mux.HandleFunc("POST /api/v1/sessions", h.startSession)
	mux.HandleFunc("GET /api/v1/sessions/{id}", h.getSession)
	mux.HandleFunc("GET /api/v1/sessions/{id}/messages", h.getMessages)
	mux.HandleFunc("POST /api/v1/sessions/{id}/messages", h.processMessage)
	mux.HandleFunc("POST /api/v1/sessions/{id}/finish", h.finishSession)
	mux.HandleFunc("GET /api/v1/sessions/{id}/result", h.getResult)
	mux.HandleFunc("POST /api/v1/admin/login", h.login)
	mux.HandleFunc("POST /api/v1/admin/scenarios", h.createScenario)
	mux.Handle("/", static)
	return h.recover(h.logRequests(mux))
}

func (h *Handler) listScenarios(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListScenarios(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load scenarios")
		return
	}
	writeJSON(w, http.StatusOK, items)
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
	session, err := h.service.StartSession(r.Context(), body.ScenarioID)
	if err != nil {
		writeStorageError(w, err, "scenario not found")
		return
	}
	writeJSON(w, http.StatusCreated, session)
}

func (h *Handler) getSession(w http.ResponseWriter, r *http.Request) {
	session, err := h.service.Session(r.Context(), r.PathValue("id"))
	if err != nil {
		writeStorageError(w, err, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *Handler) getMessages(w http.ResponseWriter, r *http.Request) {
	messages, err := h.service.Messages(r.Context(), r.PathValue("id"))
	if err != nil {
		writeStorageError(w, err, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, messages)
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
		writeStorageError(w, err, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) finishSession(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Finish(r.Context(), r.PathValue("id"))
	if err != nil {
		writeStorageError(w, err, "session not found")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (h *Handler) getResult(w http.ResponseWriter, r *http.Request) {
	result, err := h.service.Result(r.Context(), r.PathValue("id"))
	if err != nil {
		writeStorageError(w, err, "result not found")
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
	created, err := h.service.CreateScenario(r.Context(), scenario)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not save scenario")
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func writeStorageError(w http.ResponseWriter, err error, notFoundMessage string) {
	switch {
	case errors.Is(err, repository.ErrNotFound):
		writeError(w, http.StatusNotFound, notFoundMessage)
	case errors.Is(err, repository.ErrConflict):
		writeError(w, http.StatusConflict, "session changed; reload and try again")
	default:
		writeError(w, http.StatusInternalServerError, "storage unavailable")
	}
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
