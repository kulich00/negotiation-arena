package negotiation

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/kulich00/negotiation-arena/backend/internal/llm"
	"github.com/kulich00/negotiation-arena/backend/internal/repository"
)

func TestPlayerProgressionUnlocksScenarioDifficulties(t *testing.T) {
	ctx := context.Background()
	service := NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	if err := service.SeedDefaults(ctx); err != nil {
		t.Fatal(err)
	}
	player, err := service.CreatePlayer(ctx, "  Алекс  ")
	if err != nil {
		t.Fatal(err)
	}
	if player.DisplayName != "Алекс" || player.UnlockedDifficulty != DifficultyEasy || len(player.Achievements) != 0 {
		t.Fatalf("unexpected initial profile: %+v", player)
	}
	if _, err := service.StartSessionForPlayer(ctx, "salary-negotiation", player.ID); !errors.Is(err, ErrDifficultyLocked) {
		t.Fatalf("medium scenario must be locked, got %v", err)
	}
	if _, err := service.StartSessionForPlayer(ctx, "project-deadline", player.ID); !errors.Is(err, ErrDifficultyLocked) {
		t.Fatalf("hard scenario must be locked, got %v", err)
	}

	for attempt := 1; attempt <= 5; attempt++ {
		completeSuccessfulPlayerSession(t, ctx, service, player.ID)
		profile, err := service.Player(ctx, player.ID)
		if err != nil {
			t.Fatal(err)
		}
		if profile.CompletedSessions != attempt || profile.SuccessfulSessions != attempt || profile.CurrentWinStreak != attempt || profile.BestWinStreak != attempt {
			t.Fatalf("unexpected counters after attempt %d: %+v", attempt, profile)
		}
		if attempt == 2 && profile.UnlockedDifficulty != DifficultyMedium {
			t.Fatalf("medium difficulty was not unlocked: %+v", profile)
		}
		if attempt == 5 && profile.UnlockedDifficulty != DifficultyHard {
			t.Fatalf("hard difficulty was not unlocked: %+v", profile)
		}
	}

	if _, err := service.StartSessionForPlayer(ctx, "salary-negotiation", player.ID); err != nil {
		t.Fatalf("medium scenario remained locked: %v", err)
	}
	if _, err := service.StartSessionForPlayer(ctx, "project-deadline", player.ID); err != nil {
		t.Fatalf("hard scenario remained locked: %v", err)
	}
	profile, err := service.Player(ctx, player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(profile.Achievements) == 0 || profile.Achievements[0].UnlockedAt.IsZero() {
		t.Fatalf("achievements were not accumulated: %+v", profile.Achievements)
	}
}

func TestPlayerValidationAndDefaultName(t *testing.T) {
	service := NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	player, err := service.CreatePlayer(context.Background(), " ")
	if err != nil {
		t.Fatal(err)
	}
	if player.DisplayName != "Игрок" {
		t.Fatalf("unexpected default name: %q", player.DisplayName)
	}
	if _, err := service.CreatePlayer(context.Background(), strings.Repeat("я", 51)); !errors.Is(err, ErrInvalidPlayer) {
		t.Fatalf("expected invalid player error, got %v", err)
	}
}

func completeSuccessfulPlayerSession(t *testing.T, ctx context.Context, service *Service, playerID string) {
	t.Helper()
	session, err := service.StartSessionForPlayer(ctx, "vendor-introduction", playerID)
	if err != nil {
		t.Fatal(err)
	}
	moves := []PlayerMove{
		{Content: "Какие условия для вас важны?", Intent: IntentAskInterest},
		{Content: "Пробная партия снизит риск на 20%.", Intent: IntentPresentEvidence},
		{Content: "Предлагаю начать с пробной партии.", Intent: IntentPropose},
		{Content: "Согласен с условиями.", Intent: IntentAccept},
	}
	for _, move := range moves {
		if _, err := service.ProcessMove(ctx, session.ID, move); err != nil {
			t.Fatal(err)
		}
	}
	result, err := service.Finish(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.OutcomeCode != "compromise" && result.OutcomeCode != "advantageous_agreement" && result.OutcomeCode != "mutual_gain" {
		t.Fatalf("session did not produce a successful outcome: %+v", result)
	}
}

func TestFailedSessionResetsCurrentStreak(t *testing.T) {
	ctx := context.Background()
	service := NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	if err := service.SeedDefaults(ctx); err != nil {
		t.Fatal(err)
	}
	player, err := service.CreatePlayer(ctx, "Игрок")
	if err != nil {
		t.Fatal(err)
	}
	completeSuccessfulPlayerSession(t, ctx, service, player.ID)
	session, err := service.StartSessionForPlayer(ctx, "vendor-introduction", player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ProcessMove(ctx, session.ID, PlayerMove{Content: "Это моё последнее требование.", Intent: IntentPressure}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.Abandon(ctx, session.ID); err != nil {
		t.Fatal(err)
	}
	profile, err := service.Player(ctx, player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if profile.CompletedSessions != 2 || profile.SuccessfulSessions != 1 || profile.CurrentWinStreak != 0 || profile.BestWinStreak != 1 {
		t.Fatalf("unexpected profile after failure: %+v", profile)
	}
}
