package repository

import (
	"context"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
)

// Repository stores negotiation state and commits each turn as a single unit.
type Repository interface {
	ListScenarios(context.Context) ([]domain.Scenario, error)
	SaveScenario(context.Context, domain.Scenario) error
	Scenario(context.Context, string) (domain.Scenario, error)
	SaveSession(context.Context, domain.Session) error
	Session(context.Context, string) (domain.Session, error)
	ApplyTurn(context.Context, domain.Session, int, string, string) error
	Messages(context.Context, string) ([]domain.Message, error)
	Finish(context.Context, domain.Session, domain.Result) error
	Result(context.Context, string) (domain.Result, error)
}
