package repository_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/kulich00/negotiation-arena/backend/internal/database"
	"github.com/kulich00/negotiation-arena/backend/internal/domain"
	"github.com/kulich00/negotiation-arena/backend/internal/llm"
	"github.com/kulich00/negotiation-arena/backend/internal/negotiation"
	"github.com/kulich00/negotiation-arena/backend/internal/repository"
)

func TestPostgresPersistence(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run PostgreSQL integration test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := database.Migrate(ctx, url); err != nil {
		t.Fatal(err)
	}
	pool, err := database.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	repo := repository.NewPostgresRepository(pool)
	scenario := domain.Scenario{ID: "integration-" + time.Now().Format("20060102150405.000000000"), Title: "Integration", InitialMessage: "Начнём переговоры"}
	if err := repo.SaveScenario(ctx, scenario); err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM negotiation_sessions WHERE scenario_id=$1`, scenario.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM scenarios WHERE id=$1`, scenario.ID)
	}()
	service := negotiation.NewService(repo, llm.NewMockProvider())
	session, err := service.StartSession(ctx, scenario.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ProcessMessage(ctx, session.ID, "Какие условия возможны?"); err != nil {
		t.Fatal(err)
	}
	result, err := service.Finish(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}

	reopened := repository.NewPostgresRepository(pool)
	loadedSession, err := reopened.Session(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedSession.Status != "finished" || loadedSession.Turn != 1 {
		t.Fatalf("unexpected session: %+v", loadedSession)
	}
	messages, err := reopened.Messages(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 3 || messages[0].Content != scenario.InitialMessage || messages[1].Content != "Какие условия возможны?" {
		t.Fatalf("unexpected messages: %+v", messages)
	}
	loadedResult, err := reopened.Result(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedResult.FinalScore != result.FinalScore {
		t.Fatalf("unexpected result: %+v", loadedResult)
	}
}
