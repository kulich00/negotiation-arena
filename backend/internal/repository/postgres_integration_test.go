package repository_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/kulich00/negotiation-arena/backend/internal/adminauth"
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
	adminEmail := "integration-" + time.Now().Format("20060102150405.000000000") + "@example.com"
	defer func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM admins WHERE email=$1`, adminEmail)
	}()
	authConfig := adminauth.Config{SessionTTL: time.Minute, MaxLoginAttempts: 5, LoginWindow: time.Minute}
	authService := adminauth.NewService(repo, authConfig)
	if err := authService.Bootstrap(ctx, adminEmail, "integration-password"); err != nil {
		t.Fatal(err)
	}
	if err := repo.SaveAdminSession(ctx, "expired-integration-token", adminEmail, time.Now().UTC().Add(-time.Minute)); err != nil {
		t.Fatal(err)
	}
	adminSession, err := authService.Login(ctx, adminEmail, "integration-password")
	if err != nil {
		t.Fatal(err)
	}
	var expiredSessions int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM admin_sessions WHERE token_hash='expired-integration-token'`).Scan(&expiredSessions); err != nil {
		t.Fatal(err)
	}
	if expiredSessions != 0 {
		t.Fatalf("expired admin sessions were not deleted")
	}
	reopenedAuth := adminauth.NewService(repository.NewPostgresRepository(pool), authConfig)
	if err := reopenedAuth.Authenticate(ctx, adminSession.Token); err != nil {
		t.Fatalf("persisted admin session is invalid: %v", err)
	}
	if err := reopenedAuth.Logout(ctx, adminSession.Token); err != nil {
		t.Fatal(err)
	}
	if err := reopenedAuth.Authenticate(ctx, adminSession.Token); !errors.Is(err, adminauth.ErrInvalidToken) {
		t.Fatalf("expected revoked admin session, got %v", err)
	}

	seedService := negotiation.NewService(repo, llm.NewMockProvider())
	if err := seedService.SeedDefaults(ctx); err != nil {
		t.Fatal(err)
	}
	salary, err := repo.Scenario(ctx, "salary-negotiation")
	if err != nil {
		t.Fatal(err)
	}
	deadline, err := repo.Scenario(ctx, "project-deadline")
	if err != nil {
		t.Fatal(err)
	}
	if salary.Rules.Proposal.Kind != "raise_percent" || salary.Rules.Proposal.MaximumValue != 10 || deadline.Rules.Proposal.Kind != "extension_days" || deadline.Rules.MaxTurns != 10 {
		t.Fatalf("unexpected seeded rules: salary=%+v deadline=%+v", salary.Rules, deadline.Rules)
	}
	rules := domain.DefaultScenarioRules()
	rules.Proposal = domain.ProposalConstraint{Kind: "raise_percent", MaximumValue: 7, AlternativeIDs: []string{"review_later"}}
	scenario := domain.Scenario{ID: "integration-" + time.Now().Format("20060102150405.000000000"), Title: "Integration", InitialMessage: "Начнём переговоры", Rules: rules}
	originalInitialMessage := scenario.InitialMessage
	if err := repo.CreateScenario(ctx, scenario); err != nil {
		t.Fatal(err)
	}
	if err := repo.CreateScenario(ctx, scenario); !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("expected duplicate scenario conflict, got %v", err)
	}
	deletableScenario := scenario
	deletableScenario.ID += "-deletable"
	if err := repo.CreateScenario(ctx, deletableScenario); err != nil {
		t.Fatal(err)
	}
	if err := repo.DeleteScenario(ctx, deletableScenario.ID); err != nil {
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
	scenario.Title = "Updated integration scenario"
	if err := repo.UpdateScenario(ctx, scenario); !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("expected active-session update conflict, got %v", err)
	}
	if err := repo.DeleteScenario(ctx, scenario.ID); !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("expected used-scenario delete conflict, got %v", err)
	}
	_, err = service.ProcessMove(ctx, session.ID, negotiation.PlayerMove{
		Content: "Какие условия возможны?",
		Intent:  negotiation.IntentAskInterest,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.ProcessMove(ctx, session.ID, negotiation.PlayerMove{
		Content: "Если не договоримся, вернёмся к пересмотру в следующем квартале.",
		Intent:  negotiation.IntentStateBATNA,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.ProcessMove(ctx, session.ID, negotiation.PlayerMove{
		Content: "Показатели за квартал выросли.",
		Intent:  negotiation.IntentPresentEvidence,
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.ProcessMove(ctx, session.ID, negotiation.PlayerMove{
		Content:  "Предлагаю пересмотреть условия позже.",
		Intent:   negotiation.IntentPropose,
		Proposal: &negotiation.MoveProposal{AlternativeID: "review_later"},
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = service.ProcessMove(ctx, session.ID, negotiation.PlayerMove{
		Content: "Согласен с этим вариантом.",
		Intent:  negotiation.IntentAccept,
	})
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.Finish(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	scenario.InitialMessage = "Updated introduction"
	if err := repo.UpdateScenario(ctx, scenario); err != nil {
		t.Fatalf("expected update after session finish, got %v", err)
	}

	reopened := repository.NewPostgresRepository(pool)
	loadedScenario, err := reopened.Scenario(ctx, scenario.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedScenario.Rules.Proposal.MaximumValue != 7 || len(loadedScenario.Rules.Proposal.AlternativeIDs) != 1 || loadedScenario.Rules.Proposal.AlternativeIDs[0] != "review_later" {
		t.Fatalf("unexpected scenario rules: %+v", loadedScenario.Rules)
	}
	loadedSession, err := reopened.Session(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedSession.Status != "finished" || loadedSession.Turn != 5 || loadedSession.InitialMessage != originalInitialMessage || loadedSession.State.Phase != domain.PhaseFinished || !loadedSession.State.StructuredMovesUsed || !loadedSession.State.InterestsExplored || !loadedSession.State.BATNADefined || !loadedSession.State.EvidencePresented || !loadedSession.State.OfferMade || !loadedSession.State.OfferAccepted || loadedSession.State.LastOfferID != "review_later" {
		t.Fatalf("unexpected session: %+v", loadedSession)
	}
	messages, err := reopened.Messages(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 11 || messages[0].Content != originalInitialMessage || messages[1].Content != "Какие условия возможны?" {
		t.Fatalf("unexpected messages: %+v", messages)
	}
	if messages[1].Analysis == nil || messages[1].Analysis.Technique != "harvard_interests" || messages[2].Analysis != nil || messages[3].Analysis == nil || messages[3].Analysis.Technique != "batna_preparation" || messages[9].Analysis == nil || messages[9].Analysis.Technique != "agreement_confirmation" {
		t.Fatalf("turn analysis was not persisted correctly: %+v", messages)
	}
	loadedResult, err := reopened.Result(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedResult.FinalScore != result.FinalScore {
		t.Fatalf("unexpected result: %+v", loadedResult)
	}
	if loadedResult.Outcome != "Выгодное соглашение" {
		t.Fatalf("unexpected outcome: %+v", loadedResult)
	}
	if loadedResult.Analysis.AnalyzedTurns != 5 || len(loadedResult.Analysis.Techniques) != 5 {
		t.Fatalf("aggregate analysis was not persisted: %+v", loadedResult.Analysis)
	}
	if loadedResult.Analysis.BestMove == nil || loadedResult.Analysis.BestMove.Turn != 3 || loadedResult.Analysis.BestMove.Technique != "evidence_based_argument" {
		t.Fatalf("unexpected persisted best move: %+v", loadedResult.Analysis.BestMove)
	}
}
