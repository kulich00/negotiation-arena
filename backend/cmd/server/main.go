package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/kulich00/negotiation-arena/backend/internal/adminauth"
	"github.com/kulich00/negotiation-arena/backend/internal/config"
	"github.com/kulich00/negotiation-arena/backend/internal/database"
	"github.com/kulich00/negotiation-arena/backend/internal/httpapi"
	"github.com/kulich00/negotiation-arena/backend/internal/llm"
	arenametrics "github.com/kulich00/negotiation-arena/backend/internal/metrics"
	"github.com/kulich00/negotiation-arena/backend/internal/negotiation"
	"github.com/kulich00/negotiation-arena/backend/internal/repository"
	"github.com/kulich00/negotiation-arena/backend/internal/webapp"
)

func main() {
	cfg := config.Load()
	logger := newLogger(cfg.LogLevel)
	if err := cfg.Validate(); err != nil {
		logger.Error("configuration validation failed", "error", err)
		os.Exit(1)
	}
	ctx, cancelStartup := context.WithTimeout(context.Background(), 30*time.Second)

	db, err := database.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	if db != nil {
		defer db.Close()
	}

	var repo repository.AppRepository = repository.NewMemoryRepository()
	if db != nil {
		if err := database.Migrate(ctx, cfg.DatabaseURL); err != nil {
			logger.Error("database migration failed", "error", err)
			os.Exit(1)
		}
		repo = repository.NewPostgresRepository(db)
	}
	metricRegistry := arenametrics.NewRegistry()
	replyGenerator, err := configureReplyGenerator(cfg, logger, metricRegistry)
	if err != nil {
		logger.Error("LLM configuration failed", "error", err)
		os.Exit(1)
	}
	serviceOptions := []negotiation.ServiceOption{negotiation.WithReplyGenerator(replyGenerator), negotiation.WithMetrics(metricRegistry)}
	if interpreter, ok := replyGenerator.(llm.MoveInterpreter); ok {
		serviceOptions = append(serviceOptions, negotiation.WithMoveInterpreter(interpreter))
	}
	service := negotiation.NewService(repo, llm.NewMockProvider(), serviceOptions...)
	if err := service.SeedDefaults(ctx); err != nil {
		logger.Error("scenario initialization failed", "error", err)
		os.Exit(1)
	}
	authService := adminauth.NewService(repo, adminauth.Config{
		SessionTTL:       cfg.AdminSessionTTL,
		MaxLoginAttempts: cfg.AdminLoginMaxAttempts,
		LoginWindow:      cfg.AdminLoginWindow,
	})
	if err := authService.Bootstrap(ctx, cfg.AdminEmail, cfg.AdminPassword); err != nil {
		logger.Error("admin initialization failed", "error", err)
		os.Exit(1)
	}
	cancelStartup()

	handler := httpapi.NewHandler(
		service, authService, db, logger,
		httpapi.WithRateLimit(cfg.APIRateLimit, cfg.APIRateWindow),
		httpapi.WithTrustedProxyHeaders(cfg.TrustProxyHeaders),
		httpapi.WithMetrics(metricRegistry),
	)
	server := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler.Router(webapp.Handler()),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("server started", "address", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server stopped unexpectedly", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
	}
}

func newLogger(configuredLevel string) *slog.Logger {
	level := slog.LevelInfo
	switch strings.ToLower(strings.TrimSpace(configuredLevel)) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

func configureReplyGenerator(cfg config.Config, logger *slog.Logger, observer llm.Observer) (llm.ReplyGenerator, error) {
	passthrough := llm.NewPassthroughReplyGenerator()
	switch strings.ToLower(strings.TrimSpace(cfg.LLMProvider)) {
	case "", "mock":
		return passthrough, nil
	case "gemini":
		generator, err := llm.NewGeminiGenerator(llm.GeminiConfig{
			APIKeys: cfg.LLMAPIKeys,
			Model:   cfg.LLMModel,
			Client:  &http.Client{Timeout: cfg.LLMTimeout},
		})
		if err != nil {
			return nil, err
		}
		logger.Info("Gemini enabled", "model", cfg.LLMModel, "apiKeyCount", generator.APIKeyCount())
		return llm.NewFallbackReplyGenerator(generator, passthrough, logger).WithObserver(observer), nil
	default:
		return nil, fmt.Errorf("unsupported LLM_PROVIDER %q (use mock or gemini)", cfg.LLMProvider)
	}
}
