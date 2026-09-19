package negotiation

import (
	"context"
	"testing"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
	"github.com/kulich00/negotiation-arena/backend/internal/llm"
	"github.com/kulich00/negotiation-arena/backend/internal/repository"
)

func TestNegotiationFlow(t *testing.T) {
	service := NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	if err := service.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	salary, err := service.repo.Scenario(context.Background(), "salary-negotiation")
	if err != nil {
		t.Fatal(err)
	}
	deadline, err := service.repo.Scenario(context.Background(), "project-deadline")
	if err != nil {
		t.Fatal(err)
	}
	if salary.Rules.Proposal.Kind != "raise_percent" || salary.Rules.Proposal.MaximumValue != 10 || deadline.Rules.Proposal.Kind != "extension_days" || deadline.Rules.MaxTurns != 10 {
		t.Fatalf("unexpected scenario rules: salary=%+v deadline=%+v", salary.Rules, deadline.Rules)
	}
	created, err := service.CreateScenario(context.Background(), domain.Scenario{Title: "Custom", InitialMessage: "Здравствуйте"})
	if err != nil {
		t.Fatal(err)
	}
	if created.Rules.MaxTurns != domain.DefaultScenarioRules().MaxTurns || !created.Rules.RequiresEvidence {
		t.Fatalf("missing default rules: %+v", created.Rules)
	}
	session, err := service.StartSession(context.Background(), "salary-negotiation")
	if err != nil {
		t.Fatal(err)
	}
	if session.State.Phase != domain.PhaseOpening {
		t.Fatalf("unexpected initial state: %+v", session.State)
	}
	turn, err := service.ProcessMessage(context.Background(), session.ID, "Предлагаю компромисс. Какие интересы для вас важны?")
	if err != nil {
		t.Fatal(err)
	}
	if turn.Session.TrustScore <= 50 {
		t.Fatalf("expected trust to grow, got %d", turn.Session.TrustScore)
	}
	result, err := service.Finish(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.FinalScore == 0 {
		t.Fatal("expected non-zero score")
	}
	finished, err := service.Session(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if finished.State.Phase != domain.PhaseFinished {
		t.Fatalf("unexpected final state: %+v", finished.State)
	}
}
