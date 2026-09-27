package adminauth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/kulich00/negotiation-arena/backend/internal/repository"
)

func TestAdminSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	service := NewService(repository.NewMemoryRepository(), time.Hour)
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	service.now = func() time.Time { return now }

	if err := service.Bootstrap(ctx, " Admin@Example.com ", "correct-password"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Login(ctx, "admin@example.com", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
		t.Fatalf("expected invalid credentials, got %v", err)
	}
	session, err := service.Login(ctx, "ADMIN@example.com", "correct-password")
	if err != nil {
		t.Fatal(err)
	}
	if session.Token == "" || !session.ExpiresAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("unexpected session: %+v", session)
	}
	if err := service.Authenticate(ctx, session.Token); err != nil {
		t.Fatal(err)
	}

	now = now.Add(time.Hour)
	if err := service.Authenticate(ctx, session.Token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected expired token, got %v", err)
	}
}

func TestPasswordChangeRevokesSessions(t *testing.T) {
	ctx := context.Background()
	service := NewService(repository.NewMemoryRepository(), time.Hour)
	if err := service.Bootstrap(ctx, "admin@example.com", "first-password"); err != nil {
		t.Fatal(err)
	}
	session, err := service.Login(ctx, "admin@example.com", "first-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Bootstrap(ctx, "admin@example.com", "second-password"); err != nil {
		t.Fatal(err)
	}
	if err := service.Authenticate(ctx, session.Token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected revoked token, got %v", err)
	}
	if _, err := service.Login(ctx, "admin@example.com", "second-password"); err != nil {
		t.Fatal(err)
	}
}

func TestLogoutRevokesToken(t *testing.T) {
	ctx := context.Background()
	service := NewService(repository.NewMemoryRepository(), time.Hour)
	if err := service.Bootstrap(ctx, "admin@example.com", "correct-password"); err != nil {
		t.Fatal(err)
	}
	session, err := service.Login(ctx, "admin@example.com", "correct-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := service.Logout(ctx, session.Token); err != nil {
		t.Fatal(err)
	}
	if err := service.Authenticate(ctx, session.Token); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("expected revoked token, got %v", err)
	}
}
