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
	"time"

	"github.com/kulich00/negotiation-arena/backend/internal/repository"
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrInvalidToken = errors.New("invalid token")

type Session struct {
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

type Service struct {
	store repository.AdminRepository
	ttl   time.Duration
	now   func() time.Time
}

func NewService(store repository.AdminRepository, ttl time.Duration) *Service {
	return &Service{store: store, ttl: ttl, now: time.Now}
}

func (s *Service) Bootstrap(ctx context.Context, email, password string) error {
	email = normalizeEmail(email)
	if email == "" || len(email) > 254 {
		return fmt.Errorf("admin email is invalid")
	}
	if len(password) < 8 || len(password) > 72 {
		return fmt.Errorf("admin password must contain between 8 and 72 bytes")
	}
	if s.ttl <= 0 {
		return fmt.Errorf("admin session TTL must be positive")
	}

	currentHash, err := s.store.AdminPasswordHash(ctx, email)
	if err == nil && bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(password)) == nil {
		return nil
	}
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return err
	}
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	return s.store.SetAdminPassword(ctx, email, string(passwordHash))
}

func (s *Service) Login(ctx context.Context, email, password string) (Session, error) {
	email = normalizeEmail(email)
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

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return Session{}, err
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	expiresAt := s.now().UTC().Add(s.ttl)
	if err := s.store.SaveAdminSession(ctx, hashToken(token), email, expiresAt); err != nil {
		return Session{}, err
	}
	return Session{Token: token, ExpiresAt: expiresAt}, nil
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
