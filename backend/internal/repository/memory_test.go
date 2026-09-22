package repository_test

import (
	"context"
	"testing"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
	"github.com/kulich00/negotiation-arena/backend/internal/repository"
)

func TestMemoryRepositoryPreservesRulesAndState(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryRepository()
	rules := domain.DefaultScenarioRules()
	rules.Proposal = domain.ProposalConstraint{Kind: "raise_percent", MaximumValue: 10, AlternativeIDs: []string{"review_later"}}
	scenario := domain.Scenario{ID: "salary", Rules: rules}
	if err := repo.SaveScenario(ctx, scenario); err != nil {
		t.Fatal(err)
	}
	scenario.Rules.Proposal.AlternativeIDs[0] = "changed"
	loadedScenario, err := repo.Scenario(ctx, "salary")
	if err != nil {
		t.Fatal(err)
	}
	if loadedScenario.Rules.Proposal.AlternativeIDs[0] != "review_later" {
		t.Fatal("scenario rules changed after saving")
	}

	session := domain.Session{ID: "session", ScenarioID: "salary", Status: "active", State: domain.InitialSessionState()}
	if err := repo.SaveSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	session.Turn = 1
	session.State.Phase = domain.PhaseExploration
	session.State.InterestsExplored = true
	if err := repo.ApplyTurn(ctx, session, 0, "Какие интересы?", "Обсудим бюджет"); err != nil {
		t.Fatal(err)
	}
	loadedSession, err := repo.Session(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedSession.State.Phase != domain.PhaseExploration || !loadedSession.State.InterestsExplored {
		t.Fatalf("state was not saved: %+v", loadedSession.State)
	}
}
