package negotiation

import (
	"context"
	"errors"
	"math"
	"strings"
	"testing"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
	"github.com/kulich00/negotiation-arena/backend/internal/llm"
	"github.com/kulich00/negotiation-arena/backend/internal/repository"
)

func TestValidateScenario(t *testing.T) {
	tests := []struct {
		name   string
		change func(*domain.Scenario)
	}{
		{name: "missing title", change: func(s *domain.Scenario) { s.Title = " " }},
		{name: "title too long", change: func(s *domain.Scenario) { s.Title = strings.Repeat("я", 121) }},
		{name: "invalid id", change: func(s *domain.Scenario) { s.ID = "invalid id" }},
		{name: "invalid difficulty", change: func(s *domain.Scenario) { s.Difficulty = "extreme" }},
		{name: "negative turns", change: func(s *domain.Scenario) { s.Rules.MaxTurns = -1 }},
		{name: "too many turns", change: func(s *domain.Scenario) { s.Rules.MaxTurns = 51 }},
		{name: "trust above range", change: func(s *domain.Scenario) { s.Rules.MinimumTrustForAgreement = 101 }},
		{name: "argument above range", change: func(s *domain.Scenario) { s.Rules.MinimumArgumentScoreForAgreement = 101 }},
		{name: "pressure above range", change: func(s *domain.Scenario) { s.Rules.MaximumPressureForAgreement = 101 }},
		{name: "negative weight", change: func(s *domain.Scenario) { s.Rules.TrustScoreWeight = -1 }},
		{name: "not finite weight", change: func(s *domain.Scenario) { s.Rules.ArgumentScoreWeight = math.NaN() }},
		{name: "invalid proposal kind", change: func(s *domain.Scenario) { s.Rules.Proposal.Kind = "Raise Percent" }},
		{name: "none with maximum", change: func(s *domain.Scenario) { s.Rules.Proposal.MaximumValue = 1 }},
		{name: "numeric without maximum", change: func(s *domain.Scenario) { s.Rules.Proposal.Kind = "raise_percent" }},
		{name: "preferred above maximum", change: func(s *domain.Scenario) {
			s.Rules.Proposal = domain.ProposalConstraint{Kind: "raise_percent", PreferredValue: 11, MaximumValue: 10, InputMaximumValue: 20}
		}},
		{name: "maximum above input maximum", change: func(s *domain.Scenario) {
			s.Rules.Proposal = domain.ProposalConstraint{Kind: "raise_percent", PreferredValue: 5, MaximumValue: 10, InputMaximumValue: 9}
		}},
		{name: "duplicate alternative", change: func(s *domain.Scenario) { s.Rules.Proposal.AlternativeIDs = []string{"review_later", "review_later"} }},
		{name: "invalid alternative", change: func(s *domain.Scenario) { s.Rules.Proposal.AlternativeIDs = []string{"review later"} }},
		{name: "unknown preferred alternative", change: func(s *domain.Scenario) { s.Rules.Proposal.PreferredAlternativeIDs = []string{"review_later"} }},
		{name: "unknown behavior mode", change: func(s *domain.Scenario) { s.Rules.Behavior.Mode = "random" }},
		{name: "standard with difficult settings", change: func(s *domain.Scenario) { s.Rules.Behavior.Emotionality = 1 }},
		{name: "difficult without emotionality", change: func(s *domain.Scenario) {
			s.Rules.Behavior = domain.OpponentBehavior{Mode: OpponentModeDifficult, Volatility: 2, InitialPriority: "срок"}
		}},
		{name: "difficult without volatility", change: func(s *domain.Scenario) {
			s.Rules.Behavior = domain.OpponentBehavior{Mode: OpponentModeDifficult, Emotionality: 2, InitialPriority: "срок"}
		}},
		{name: "difficult without initial priority", change: func(s *domain.Scenario) {
			s.Rules.Behavior = domain.OpponentBehavior{Mode: OpponentModeDifficult, Emotionality: 2, Volatility: 2}
		}},
		{name: "unordered priority shifts", change: func(s *domain.Scenario) {
			s.Rules.Behavior = domain.OpponentBehavior{Mode: OpponentModeDifficult, Emotionality: 2, Volatility: 2, InitialPriority: "срок", PriorityShifts: []domain.OpponentPriorityShift{{Turn: 3, Priority: "риски"}, {Turn: 2, Priority: "цена"}}}
		}},
		{name: "priority shift after turn limit", change: func(s *domain.Scenario) {
			s.Rules.Behavior = domain.OpponentBehavior{Mode: OpponentModeDifficult, Emotionality: 2, Volatility: 2, InitialPriority: "срок", PriorityShifts: []domain.OpponentPriorityShift{{Turn: s.Rules.MaxTurns + 1, Priority: "риски"}}}
		}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			scenario := validScenarioFixture()
			test.change(&scenario)
			if err := ValidateScenario(scenario); !errors.Is(err, ErrInvalidScenario) {
				t.Fatalf("expected ErrInvalidScenario, got %v", err)
			}
		})
	}
}

func TestCreateScenarioNormalizesAndAppliesDefaults(t *testing.T) {
	service := NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	scenario := validScenarioFixture()
	scenario.ID = ""
	scenario.Title = "  Новый сценарий!  "
	scenario.Difficulty = " MEDIUM "
	scenario.Rules = domain.ScenarioRules{}

	created, err := service.CreateScenario(context.Background(), scenario)
	if err != nil {
		t.Fatal(err)
	}
	if created.Title != "Новый сценарий!" || created.Difficulty != "medium" {
		t.Fatalf("scenario was not normalized: %+v", created)
	}
	if !strings.HasPrefix(created.ID, "новый-сценарий-") {
		t.Fatalf("unexpected generated id: %q", created.ID)
	}
	if created.Rules.MaxTurns != domain.DefaultScenarioRules().MaxTurns || created.Rules.Proposal.Kind != "none" {
		t.Fatalf("default rules were not applied: %+v", created.Rules)
	}
}

func TestScenarioManagementLifecycle(t *testing.T) {
	ctx := context.Background()
	service := NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	scenario := validScenarioFixture()
	created, err := service.CreateScenario(ctx, scenario)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateScenario(ctx, scenario); !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("expected duplicate conflict, got %v", err)
	}

	updatedInput := created
	updatedInput.ID = ""
	updatedInput.Title = "  Updated scenario  "
	updated, err := service.UpdateScenario(ctx, created.ID, updatedInput)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != created.ID || updated.Title != "Updated scenario" {
		t.Fatalf("unexpected updated scenario: %+v", updated)
	}
	updatedInput.ID = "different-id"
	if _, err := service.UpdateScenario(ctx, created.ID, updatedInput); !errors.Is(err, ErrInvalidScenario) {
		t.Fatalf("expected id mismatch validation error, got %v", err)
	}

	session, err := service.StartSession(ctx, created.ID)
	if err != nil {
		t.Fatal(err)
	}
	updatedInput.ID = created.ID
	if _, err := service.UpdateScenario(ctx, created.ID, updatedInput); !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("expected active-session update conflict, got %v", err)
	}
	if err := service.DeleteScenario(ctx, created.ID); !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("expected used-scenario delete conflict, got %v", err)
	}
	if _, err := service.Finish(ctx, session.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.UpdateScenario(ctx, created.ID, updatedInput); err != nil {
		t.Fatalf("expected update after session finish, got %v", err)
	}
}

func TestDeleteUnusedScenario(t *testing.T) {
	ctx := context.Background()
	service := NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	scenario := validScenarioFixture()
	if _, err := service.CreateScenario(ctx, scenario); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteScenario(ctx, scenario.ID); err != nil {
		t.Fatal(err)
	}
	if err := service.DeleteScenario(ctx, scenario.ID); !errors.Is(err, repository.ErrNotFound) {
		t.Fatalf("expected not found after deletion, got %v", err)
	}
}

func validScenarioFixture() domain.Scenario {
	return domain.Scenario{
		ID:             "valid-scenario",
		Title:          "Переговоры о проекте",
		Sphere:         "IT",
		Topic:          "Сроки проекта",
		Difficulty:     "medium",
		OpponentRole:   "Заказчик",
		OpponentTone:   "Требовательный",
		PlayerGoal:     "Согласовать реалистичный срок",
		OpponentGoal:   "Снизить риски задержки",
		InitialMessage: "Почему нужно переносить срок?",
		Rules:          domain.DefaultScenarioRules(),
	}
}
