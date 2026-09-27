package adminauth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/kulich00/negotiation-arena/backend/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrInvalidToken = errors.New("invalid token")
var ErrTooManyLoginAttempts = errors.New("too many login attempts")

type Config struct {
	SessionTTL       time.Duration
	MaxLoginAttempts int
	LoginWindow      time.Duration
}

type RateLimitError struct {
	RetryAfter time.Duration
}

func (e *RateLimitError) Error() string { return ErrTooManyLoginAttempts.Error() }
func (e *RateLimitError) Unwrap() error { return ErrTooManyLoginAttempts }

type Session struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type Service struct {
	store       repository.AdminRepository
	config      Config
	now         func() time.Time
	mu          sync.Mutex
	adminEmail  string
	loginBlocks map[string]loginBlock
}

type loginBlock struct {
	attempts int
	resetAt  time.Time
}

func NewService(store repository.AdminRepository, config Config) *Service {
	return &Service{store: store, config: config, now: time.Now, loginBlocks: make(map[string]loginBlock)}
}

func (s *Service) Bootstrap(ctx context.Context, email, password string) error {
	email = normalizeEmail(email)
	if email == "" || len(email) > 254 {
		return fmt.Errorf("admin email is invalid")
	}
	if len(password) < 8 || len(password) > 72 {
		return fmt.Errorf("admin password must contain between 8 and 72 bytes")
	}
	if s.config.SessionTTL <= 0 {
		return fmt.Errorf("admin session TTL must be positive")
	}
	if s.config.MaxLoginAttempts < 1 {
		return fmt.Errorf("admin login attempt limit must be positive")
	}
	if s.config.LoginWindow <= 0 {
		return fmt.Errorf("admin login window must be positive")
	}
	s.mu.Lock()
	s.adminEmail = email
	s.mu.Unlock()

	currentHash, err := s.store.AdminPasswordHash(ctx, email)
	if err == nil && bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(password)) == nil {
		return s.store.DeleteExpiredAdminSessions(ctx, s.now().UTC())
	}
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return err
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	if err := s.store.SetAdminPassword(ctx, email, string(passwordHash)); err != nil {
		return err
	}
	return s.store.DeleteExpiredAdminSessions(ctx, s.now().UTC())
}

func (s *Service) Login(ctx context.Context, email, password string) (Session, error) {
	email = normalizeEmail(email)
	now := s.now().UTC()
	loginKey := s.loginKey(email)
	if err := s.beginLoginAttempt(loginKey, now); err != nil {
		return Session{}, err
	}
	if email == "" || password == "" || len(email) > 254 || len(password) > 72 {
		return Session{}, ErrInvalidCredentials
	}
	passwordHash, err := s.store.AdminPasswordHash(ctx, email)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return Session{}, ErrInvalidCredentials
		}
		return Session{}, err
	}
	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
		return Session{}, ErrInvalidCredentials
	}
	s.clearLoginFailures(loginKey)

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return Session{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	if err := s.store.DeleteExpiredAdminSessions(ctx, now); err != nil {
		return Session{}, err
	}
	expiresAt := now.Add(s.config.SessionTTL)
	if err := s.store.SaveAdminSession(ctx, hashToken(token), email, expiresAt); err != nil {
		return Session{}, err
	}
	return Session{Token: token, ExpiresAt: expiresAt}, nil
}

func (s *Service) loginKey(email string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if email != "" && email == s.adminEmail {
		return email
	}
	return "unknown"
}

func (s *Service) beginLoginAttempt(key string, now time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	block, exists := s.loginBlocks[key]
	if !exists || !now.Before(block.resetAt) {
		block = loginBlock{resetAt: now.Add(s.config.LoginWindow)}
	}
	if block.attempts >= s.config.MaxLoginAttempts {
		return &RateLimitError{RetryAfter: block.resetAt.Sub(now)}
	}
	block.attempts++
	s.loginBlocks[key] = block
	return nil
}

func (s *Service) clearLoginFailures(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.loginBlocks, key)
}

func (s *Service) Authenticate(ctx context.Context, token string) error {
	if token == "" || len(token) > 128 {
		return ErrInvalidToken
	}
	valid, err := s.store.ValidAdminSession(ctx, hashToken(token), s.now().UTC())
	if err != nil {
		return err
	}
	if !valid {
		return ErrInvalidToken
	}
	return nil
}

func (s *Service) Logout(ctx context.Context, token string) error {
	if token == "" || len(token) > 128 {
		return nil
	}
	return s.store.DeleteAdminSession(ctx, hashToken(token))
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func hashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}
