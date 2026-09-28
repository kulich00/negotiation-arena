package negotiation

import (
	"errors"
	"strings"
	"time"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
)

const (
	DifficultyEasy   = "easy"
	DifficultyMedium = "medium"
	DifficultyHard   = "hard"
)

var ErrInvalidPlayer = errors.New("invalid player")
var ErrDifficultyLocked = errors.New("difficulty locked")

func normalizePlayerName(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		value = "Игрок"
	}
	if len([]rune(value)) > 50 {
		return "", ErrInvalidPlayer
	}
	return value, nil
}

func difficultyUnlocked(unlocked, requested string) bool {
	ranks := map[string]int{DifficultyEasy: 1, DifficultyMedium: 2, DifficultyHard: 3}
	return ranks[requested] > 0 && ranks[requested] <= ranks[unlocked]
}

func initialPlayerProfile(id, displayName string, now time.Time) domain.PlayerProfile {
	return domain.PlayerProfile{
		ID: id, DisplayName: displayName, UnlockedDifficulty: DifficultyEasy,
		Achievements: []domain.UnlockedAchievement{}, CreatedAt: now, UpdatedAt: now,
	}
}
