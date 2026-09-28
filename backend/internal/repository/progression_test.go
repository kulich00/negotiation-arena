package repository

import (
	"context"
	"testing"
	"time"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
)

func TestPerfectScoreUnlocksNextScenarioDifficulty(t *testing.T) {
	ctx := context.Background()
	repo := NewMemoryRepository()
	now := time.Now().UTC()
	player := domain.PlayerProfile{
		ID: "player", DisplayName: "Player", UnlockedDifficulty: "easy",
		CreatedAt: now, UpdatedAt: now,
	}
	if err := repo.CreatePlayer(ctx, player); err != nil {
		t.Fatal(err)
	}

	finishPerfectSession := func(id, difficulty string) {
		t.Helper()
		scenarioID := "scenario-" + difficulty
		if err := repo.SaveScenario(ctx, domain.Scenario{ID: scenarioID, Difficulty: difficulty}); err != nil {
			t.Fatal(err)
		}
		session := domain.Session{
			ID: id, ScenarioID: scenarioID, PlayerID: player.ID,
			Status: domain.SessionStatusActive, StartedAt: now,
		}
		if err := repo.SaveSession(ctx, session); err != nil {
			t.Fatal(err)
		}
		finishedAt := now.Add(time.Minute)
		session.Status = domain.SessionStatusFinished
		session.FinishedAt = &finishedAt
		result := domain.Result{
			SessionID: id, FinalScore: 100, OutcomeCode: "advantageous_agreement",
		}
		if err := repo.Finish(ctx, session, result); err != nil {
			t.Fatal(err)
		}
	}

	finishPerfectSession("easy-perfect", "easy")
	profile, err := repo.Player(ctx, player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if profile.SuccessfulSessions != 1 || profile.UnlockedDifficulty != "medium" {
		t.Fatalf("perfect easy session did not unlock medium: %+v", profile)
	}

	finishPerfectSession("medium-perfect", "medium")
	profile, err = repo.Player(ctx, player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if profile.SuccessfulSessions != 2 || profile.UnlockedDifficulty != "hard" {
		t.Fatalf("perfect medium session did not unlock hard: %+v", profile)
	}
}

func TestPerfectScoreUnlockRequiresSuccessfulOutcomeAndCurrentLevel(t *testing.T) {
	tests := []struct {
		name                string
		current             string
		successfulSessions  int
		completedDifficulty string
		finalScore          int
		successful          bool
		want                string
	}{
		{name: "score below perfect", current: "easy", successfulSessions: 1, completedDifficulty: "easy", finalScore: 99, successful: true, want: "easy"},
		{name: "unsuccessful perfect score", current: "easy", completedDifficulty: "easy", finalScore: 100, successful: false, want: "easy"},
		{name: "easy replay cannot unlock hard", current: "medium", successfulSessions: 3, completedDifficulty: "easy", finalScore: 100, successful: true, want: "medium"},
		{name: "existing hard level never downgrades", current: "hard", completedDifficulty: "easy", finalScore: 40, successful: false, want: "hard"},
		{name: "success count still unlocks medium", current: "easy", successfulSessions: 2, completedDifficulty: "easy", finalScore: 70, successful: true, want: "medium"},
		{name: "success count still unlocks hard", current: "medium", successfulSessions: 5, completedDifficulty: "medium", finalScore: 70, successful: true, want: "hard"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := unlockedDifficulty(
				test.current, test.successfulSessions, test.completedDifficulty,
				test.finalScore, test.successful,
			)
			if got != test.want {
				t.Fatalf("unlockedDifficulty() = %q, want %q", got, test.want)
			}
		})
	}
}
