package repository

import (
	"context"
	"time"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
)

// Repository stores negotiation state and commits each turn as a single unit.
type Repository interface {
	ListScenarios(context.Context) ([]domain.Scenario, error)
	SaveScenario(context.Context, domain.Scenario) error
	CreateScenario(context.Context, domain.Scenario) error
	UpdateScenario(context.Context, domain.Scenario) error
	DeleteScenario(context.Context, string) error
	Scenario(context.Context, string) (domain.Scenario, error)
	SaveSession(context.Context, domain.Session) error
	Session(context.Context, string) (domain.Session, error)
	ApplyTurn(context.Context, domain.Session, int, string, string, domain.TurnAnalysis) error
	Messages(context.Context, string) ([]domain.Message, error)
	Finish(context.Context, domain.Session, domain.Result) error
	Result(context.Context, string) (domain.Result, error)
}

// AdminRepository stores administrator credentials and short-lived sessions.
// Raw passwords and bearer tokens must never be passed to this interface.
type AdminRepository interface {
	SetAdminPassword(context.Context, string, string) error
	AdminPasswordHash(context.Context, string) (string, error)
	SaveAdminSession(context.Context, string, string, time.Time) error
	ValidAdminSession(context.Context, string, time.Time) (bool, error)
	DeleteAdminSession(context.Context, string) error
	DeleteExpiredAdminSessions(context.Context, time.Time) error
}

type AppRepository interface {
	Repository
	AdminRepository
}
