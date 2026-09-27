package adminauth

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/kulich00/negotiation-arena/backend/internal/repository"
)

func TestAdminSessionLifecycle(t *testing.T) {
	ctx := context.Background()
	service := newTestService(repository.NewMemoryRepository(), time.Hour)
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
	service := newTestService(repository.NewMemoryRepository(), time.Hour)
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
	service := newTestService(repository.NewMemoryRepository(), time.Hour)
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

func TestLoginRateLimit(t *testing.T) {
	ctx := context.Background()
	service := NewService(repository.NewMemoryRepository(), Config{
		SessionTTL:       time.Hour,
		MaxLoginAttempts: 2,
		LoginWindow:      time.Minute,
	})
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	service.now = func() time.Time { return now }
	if err := service.Bootstrap(ctx, "admin@example.com", "correct-password"); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if _, err := service.Login(ctx, "admin@example.com", "wrong-password"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected invalid credentials, got %v", err)
		}
	}
	_, err := service.Login(ctx, "admin@example.com", "correct-password")
	var rateLimitError *RateLimitError
	if !errors.As(err, &rateLimitError) || rateLimitError.RetryAfter != time.Minute {
		t.Fatalf("expected one-minute rate limit, got %v", err)
	}

	now = now.Add(time.Minute)
	if _, err := service.Login(ctx, "admin@example.com", "correct-password"); err != nil {
		t.Fatalf("expected login after rate-limit window, got %v", err)
	}
}

func TestLoginRateLimitIsConcurrent(t *testing.T) {
	ctx := context.Background()
	service := NewService(repository.NewMemoryRepository(), Config{
		SessionTTL:       time.Hour,
		MaxLoginAttempts: 1,
		LoginWindow:      time.Minute,
	})
	if err := service.Bootstrap(ctx, "admin@example.com", "correct-password"); err != nil {
		t.Fatal(err)
	}

	const requests = 8
	start := make(chan struct{})
	errorsChannel := make(chan error, requests)
	var group sync.WaitGroup
	for range requests {
		group.Add(1)
		go func() {
			defer group.Done()
			<-start
			_, err := service.Login(ctx, "admin@example.com", "wrong-password")
			errorsChannel <- err
		}()
	}
	close(start)
	group.Wait()
	close(errorsChannel)

	invalidCredentials := 0
	rateLimited := 0
	for err := range errorsChannel {
		switch {
		case errors.Is(err, ErrTooManyLoginAttempts):
			rateLimited++
		case errors.Is(err, ErrInvalidCredentials):
			invalidCredentials++
		default:
			t.Fatalf("unexpected login error: %v", err)
		}
	}
	if invalidCredentials != 1 || rateLimited != requests-1 {
		t.Fatalf("invalid credentials=%d rate limited=%d", invalidCredentials, rateLimited)
	}
}

func newTestService(repo repository.AdminRepository, sessionTTL time.Duration) *Service {
	return NewService(repo, Config{
		SessionTTL:       sessionTTL,
		MaxLoginAttempts: 5,
		LoginWindow:      15 * time.Minute,
	})
}
