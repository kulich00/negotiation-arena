package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/kulich00/negotiation-arena/backend/internal/adminauth"
	"github.com/kulich00/negotiation-arena/backend/internal/domain"
	"github.com/kulich00/negotiation-arena/backend/internal/negotiation"
	"github.com/kulich00/negotiation-arena/backend/internal/repository"
)

type Handler struct {
	service *negotiation.Service
	auth    *adminauth.Service
	db      *pgxpool.Pool
	logger  *slog.Logger
}

func NewHandler(service *negotiation.Service, auth *adminauth.Service, db *pgxpool.Pool, logger *slog.Logger) *Handler {
	return &Handler{service: service, auth: auth, db: db, logger: logger}
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
	mux.HandleFunc("POST /api/v1/admin/logout", h.logout)
	mux.HandleFunc("GET /api/v1/admin/scenarios", h.listAdminScenarios)
	mux.HandleFunc("POST /api/v1/admin/scenarios", h.createScenario)
	mux.HandleFunc("PUT /api/v1/admin/scenarios/{id}", h.updateScenario)
	mux.HandleFunc("DELETE /api/v1/admin/scenarios/{id}", h.deleteScenario)
	mux.Handle("/", static)
	return h.recover(h.securityHeaders(h.logRequests(mux)))
}

func (h *Handler) listScenarios(w http.ResponseWriter, r *http.Request) {
	items, err := h.service.ListScenarios(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load scenarios")
		return
	}
	public := make([]publicScenario, 0, len(items))
	for _, scenario := range items {
		public = append(public, publicScenario{
			ID: scenario.ID, Title: scenario.Title, Sphere: scenario.Sphere,
			Topic: scenario.Topic, Difficulty: scenario.Difficulty,
			OpponentRole: scenario.OpponentRole, OpponentTone: scenario.OpponentTone,
			PlayerGoal: scenario.PlayerGoal, InitialMessage: scenario.InitialMessage,
		})
	}
	writeJSON(w, http.StatusOK, public)
}

// publicScenario omits the opponent's objective and private rules.
type publicScenario struct {
	ID             string `json:"id"`
	Title          string `json:"title"`
	Sphere         string `json:"sphere"`
	Topic          string `json:"topic"`
	Difficulty     string `json:"difficulty"`
	OpponentRole   string `json:"opponentRole"`
	OpponentTone   string `json:"opponentTone"`
	PlayerGoal     string `json:"playerGoal"`
	InitialMessage string `json:"initialMessage"`
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
	var move negotiation.PlayerMove
	if !decode(w, r, &move) {
		return
	}

	var (
		result negotiation.TurnResult
		err    error
	)
	if move.Intent == "" && move.Proposal == nil {
		result, err = h.service.ProcessMessage(r.Context(), r.PathValue("id"), move.Content)
	} else {
		result, err = h.service.ProcessMove(r.Context(), r.PathValue("id"), move)
	}
	if err != nil {
		if errors.Is(err, negotiation.ErrInvalidMove) {
			writeError(w, http.StatusBadRequest, "invalid move")
			return
		}
		if errors.Is(err, negotiation.ErrTurnLimitReached) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
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
	w.Header().Set("Cache-Control", "no-store")
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !decode(w, r, &body) {
		return
	}
	session, err := h.auth.Login(r.Context(), body.Email, body.Password)
	var rateLimitError *adminauth.RateLimitError
	if errors.As(err, &rateLimitError) {
		retryAfter := int((rateLimitError.RetryAfter + time.Second - 1) / time.Second)
		if retryAfter < 1 {
			retryAfter = 1
		}
		w.Header().Set("Retry-After", fmt.Sprintf("%d", retryAfter))
		writeError(w, http.StatusTooManyRequests, "too many login attempts")
		return
	}
	if errors.Is(err, adminauth.ErrInvalidCredentials) {
		writeError(w, http.StatusUnauthorized, "invalid credentials")
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, "authentication unavailable")
		return
	}
	writeJSON(w, http.StatusOK, session)
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if err := h.auth.Logout(r.Context(), bearerToken(r.Header.Get("Authorization"))); err != nil {
		writeError(w, http.StatusInternalServerError, "authentication unavailable")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) createScenario(w http.ResponseWriter, r *http.Request) {
	if !h.authorizeAdmin(w, r) {
		return
	}
	var scenario domain.Scenario
	if !decode(w, r, &scenario) {
		return
	}
	created, err := h.service.CreateScenario(r.Context(), scenario)
	if err != nil {
		switch {
		case errors.Is(err, negotiation.ErrInvalidScenario):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, repository.ErrConflict):
			writeError(w, http.StatusConflict, "scenario already exists")
		default:
			writeError(w, http.StatusInternalServerError, "could not save scenario")
		}
		return
	}
	writeJSON(w, http.StatusCreated, created)
}

func (h *Handler) listAdminScenarios(w http.ResponseWriter, r *http.Request) {
	if !h.authorizeAdmin(w, r) {
		return
	}
	scenarios, err := h.service.ListScenarios(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "could not load scenarios")
		return
	}
	writeJSON(w, http.StatusOK, scenarios)
}

func (h *Handler) updateScenario(w http.ResponseWriter, r *http.Request) {
	if !h.authorizeAdmin(w, r) {
		return
	}
	var scenario domain.Scenario
	if !decode(w, r, &scenario) {
		return
	}
	updated, err := h.service.UpdateScenario(r.Context(), r.PathValue("id"), scenario)
	if err != nil {
		switch {
		case errors.Is(err, negotiation.ErrInvalidScenario):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, repository.ErrNotFound):
			writeError(w, http.StatusNotFound, "scenario not found")
		case errors.Is(err, repository.ErrConflict):
			writeError(w, http.StatusConflict, "scenario has active sessions")
		default:
			writeError(w, http.StatusInternalServerError, "could not update scenario")
		}
		return
	}
	writeJSON(w, http.StatusOK, updated)
}

func (h *Handler) deleteScenario(w http.ResponseWriter, r *http.Request) {
	if !h.authorizeAdmin(w, r) {
		return
	}
	err := h.service.DeleteScenario(r.Context(), r.PathValue("id"))
	if err != nil {
		switch {
		case errors.Is(err, negotiation.ErrInvalidScenario):
			writeError(w, http.StatusBadRequest, err.Error())
		case errors.Is(err, repository.ErrNotFound):
			writeError(w, http.StatusNotFound, "scenario not found")
		case errors.Is(err, repository.ErrConflict):
			writeError(w, http.StatusConflict, "scenario has negotiation sessions")
		default:
			writeError(w, http.StatusInternalServerError, "could not delete scenario")
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) authorizeAdmin(w http.ResponseWriter, r *http.Request) bool {
	w.Header().Set("Cache-Control", "no-store")
	if h.auth == nil {
		writeError(w, http.StatusInternalServerError, "authentication unavailable")
		return false
	}
	token := bearerToken(r.Header.Get("Authorization"))
	if err := h.auth.Authenticate(r.Context(), token); errors.Is(err, adminauth.ErrInvalidToken) {
		writeError(w, http.StatusUnauthorized, "invalid token")
		return false
	} else if err != nil {
		writeError(w, http.StatusInternalServerError, "authentication unavailable")
		return false
	}
	return true
}

func bearerToken(header string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(header, prefix))
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
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON")
		return false
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
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

func (h *Handler) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; img-src 'self' data:; style-src 'self' 'unsafe-inline'; script-src 'self'; base-uri 'self'; frame-ancestors 'none'; form-action 'self'")
		w.Header().Set("Cross-Origin-Opener-Policy", "same-origin")
		w.Header().Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
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
