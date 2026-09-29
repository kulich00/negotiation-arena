package negotiation

import (
	"context"
	"testing"
	"time"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
	"github.com/kulich00/negotiation-arena/backend/internal/llm"
	"github.com/kulich00/negotiation-arena/backend/internal/repository"
)

func TestExportCorpusBuildsAnnotatedDialogue(t *testing.T) {
	service := NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	if err := service.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	session, err := service.StartSession(context.Background(), "vendor-introduction")
	if err != nil {
		t.Fatal(err)
	}
	turn, err := service.ProcessMove(context.Background(), session.ID, PlayerMove{
		Content: "Что для вас важно при пробной поставке?", Intent: IntentAskInterest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Abandon(context.Background(), session.ID); err != nil {
		t.Fatal(err)
	}

	corpus, err := service.ExportCorpus(context.Background(), repository.SessionFilter{Status: domain.SessionStatusAbandoned, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if corpus.SchemaVersion != domain.CorpusSchemaVersion || corpus.Total != 1 || len(corpus.Items) != 1 {
		t.Fatalf("unexpected corpus page: %+v", corpus)
	}
	dialogue := corpus.Items[0]
	if dialogue.Origin != "runtime" || dialogue.DialogueID != session.ID || dialogue.Scenario.OpponentGoal == "" || len(dialogue.Turns) != 1 {
		t.Fatalf("incomplete dialogue: %+v", dialogue)
	}
	exportedTurn := dialogue.Turns[0]
	if exportedTurn.Analysis.Intent != string(IntentAskInterest) || exportedTurn.Analysis.InterpretationSource != "structured" || exportedTurn.Analysis.ReplySource != "local" {
		t.Fatalf("missing training labels: %+v", exportedTurn.Analysis)
	}
	if exportedTurn.Before == nil || exportedTurn.After == nil || exportedTurn.Before.TrustScore != 50 || exportedTurn.After.TrustScore != turn.Session.TrustScore {
		t.Fatalf("invalid score snapshots: %+v", exportedTurn)
	}
	if !dialogue.Quality.Complete || len(dialogue.Quality.Issues) != 0 {
		t.Fatalf("new dialogue was marked incomplete: %+v", dialogue.Quality)
	}
	if dialogue.InitialMessage.CreatedAt.IsZero() || exportedTurn.Player.CreatedAt.IsZero() || exportedTurn.Opponent.CreatedAt.IsZero() {
		t.Fatalf("message timestamps are missing: %+v", dialogue)
	}
}

func TestBuildCorpusDialogueMarksEmptyDialogueIncomplete(t *testing.T) {
	dialogue := buildCorpusDialogue(domain.SessionDetail{
		Session: domain.Session{
			ID:             "empty",
			Status:         domain.SessionStatusActive,
			InitialMessage: "Начало",
			StartedAt:      time.Now().UTC(),
		},
		Messages: []domain.Message{{Sender: "opponent", Content: "Начало", CreatedAt: time.Now().UTC()}},
	})
	if dialogue.Quality.Complete || len(dialogue.Turns) != 0 {
		t.Fatalf("empty dialogue quality was not reported: %+v", dialogue)
	}
	if len(dialogue.Quality.Issues) != 1 || dialogue.Quality.Issues[0] != "no_player_turns" {
		t.Fatalf("unexpected empty dialogue issues: %+v", dialogue.Quality.Issues)
	}
}

func TestBuildCorpusDialogueMarksLegacyMissingCheckpoints(t *testing.T) {
	detail := domain.SessionDetail{
		Session: domain.Session{ID: "legacy", Status: domain.SessionStatusFinished, InitialMessage: "Начало", StartedAt: time.Now().UTC()},
		Messages: []domain.Message{
			{Sender: "opponent", Content: "Начало", CreatedAt: time.Now().UTC()},
			{Sender: "player", Content: "Ход", CreatedAt: time.Now().UTC(), Analysis: &domain.TurnAnalysis{Intent: "neutral"}},
			{Sender: "opponent", Content: "Ответ", CreatedAt: time.Now().UTC()},
		},
	}
	dialogue := buildCorpusDialogue(detail)
	if dialogue.Quality.Complete || len(dialogue.Quality.Issues) < 2 || len(dialogue.Turns) != 1 {
		t.Fatalf("legacy quality was not reported: %+v", dialogue)
	}
	if dialogue.Turns[0].Before != nil || dialogue.Turns[0].After != nil {
		t.Fatalf("missing checkpoints were fabricated: %+v", dialogue.Turns[0])
	}
}
