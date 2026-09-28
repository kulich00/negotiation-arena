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
	if salary.Rules.Proposal.Kind != "raise_percent" || salary.Rules.Proposal.PreferredValue != 6 || salary.Rules.Proposal.MaximumValue != 10 || salary.Rules.Proposal.InputMaximumValue != 30 || deadline.Rules.Proposal.Kind != "extension_days" || deadline.Rules.MaxTurns != 10 || deadline.Rules.MinimumArgumentScoreForAgreement != 3 || deadline.Rules.MaximumPressureForAgreement != 2 || deadline.Rules.Behavior.Mode != negotiation.OpponentModeDifficult || len(deadline.Rules.Behavior.PriorityShifts) != 2 {
		t.Fatalf("unexpected seeded rules: salary=%+v deadline=%+v", salary.Rules, deadline.Rules)
	}
	rules := domain.DefaultScenarioRules()
	rules.Proposal = domain.ProposalConstraint{Kind: "raise_percent", MaximumValue: 7, AlternativeIDs: []string{"review_later"}}
	scenario := domain.Scenario{ID: "integration-" + time.Now().Format("20060102150405.000000000"), Title: "Integration", Difficulty: negotiation.DifficultyEasy, InitialMessage: "Начнём переговоры", Rules: rules}
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
	player, err := service.CreatePlayer(ctx, "Integration Player")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM negotiation_sessions WHERE scenario_id=$1`, scenario.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM player_profiles WHERE id=$1`, player.ID)
	}()
	session, err := service.StartSessionForPlayer(ctx, scenario.ID, player.ID)
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
	repeatedResult, err := service.Finish(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if repeatedResult.OutcomeCode != result.OutcomeCode || repeatedResult.FinalScore != result.FinalScore {
		t.Fatalf("idempotent finish returned a different result: first=%+v repeated=%+v", result, repeatedResult)
	}
	scenario.InitialMessage = "Updated introduction"
	if err := repo.UpdateScenario(ctx, scenario); err != nil {
		t.Fatalf("expected update after session finish, got %v", err)
	}
	forkedSession, err := service.ForkSession(ctx, session.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	abandonedSession, err := service.StartSession(ctx, scenario.ID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ProcessMove(ctx, abandonedSession.ID, negotiation.PlayerMove{Content: "Других вариантов у вас нет.", Intent: negotiation.IntentPressure}); err != nil {
		t.Fatal(err)
	}
	abandonedResult, err := service.Abandon(ctx, abandonedSession.ID)
	if err != nil {
		t.Fatal(err)
	}
	if abandonedResult.OutcomeCode != "abandoned" {
		t.Fatalf("unexpected abandoned result: %+v", abandonedResult)
	}

	reopened := repository.NewPostgresRepository(pool)
	loadedScenario, err := reopened.Scenario(ctx, scenario.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedScenario.Rules.Proposal.PreferredValue != 7 || loadedScenario.Rules.Proposal.MaximumValue != 7 || loadedScenario.Rules.Proposal.InputMaximumValue != 7 || len(loadedScenario.Rules.Proposal.AlternativeIDs) != 1 || loadedScenario.Rules.Proposal.AlternativeIDs[0] != "review_later" {
		t.Fatalf("unexpected scenario rules: %+v", loadedScenario.Rules)
	}
	loadedSession, err := reopened.Session(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedSession.PlayerID != player.ID || loadedSession.Status != "finished" || loadedSession.Turn != 5 || loadedSession.InitialMessage != originalInitialMessage || loadedSession.State.Phase != domain.PhaseFinished || !loadedSession.State.StructuredMovesUsed || !loadedSession.State.InterestsExplored || !loadedSession.State.BATNADefined || !loadedSession.State.EvidencePresented || !loadedSession.State.OfferMade || !loadedSession.State.OfferAccepted || loadedSession.State.LastOfferID != "review_later" || loadedSession.State.LastOfferQuality != domain.OfferQualityAcceptable {
		t.Fatalf("unexpected session: %+v", loadedSession)
	}
	if loadedSession.FinishedAt == nil {
		t.Fatal("finished timestamp was not persisted")
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
	checkpoints, err := reopened.Checkpoints(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(checkpoints) != 6 || checkpoints[0].Turn != 0 || checkpoints[5].Turn != 5 || !checkpoints[5].State.OfferAccepted {
		t.Fatalf("turn checkpoints were not persisted: %+v", checkpoints)
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
	if loadedResult.OutcomeCode != "advantageous_agreement" {
		t.Fatalf("unexpected outcome code: %+v", loadedResult)
	}
	if !hasAchievement(loadedResult.Achievements, negotiation.AchievementDealMaker) || !hasAchievement(loadedResult.Achievements, negotiation.AchievementWellPrepared) || !hasAchievement(loadedResult.Achievements, negotiation.AchievementCleanRun) {
		t.Fatalf("achievements were not persisted: %+v", loadedResult.Achievements)
	}
	if loadedResult.Analysis.AnalyzedTurns != 5 || len(loadedResult.Analysis.Techniques) != 5 {
		t.Fatalf("aggregate analysis was not persisted: %+v", loadedResult.Analysis)
	}
	if loadedResult.Analysis.BestMove == nil || loadedResult.Analysis.BestMove.Turn != 3 || loadedResult.Analysis.BestMove.Technique != "evidence_based_argument" {
		t.Fatalf("unexpected persisted best move: %+v", loadedResult.Analysis.BestMove)
	}
	loadedPlayer, err := reopened.Player(ctx, player.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedPlayer.CompletedSessions != 1 || loadedPlayer.SuccessfulSessions != 1 || loadedPlayer.CurrentWinStreak != 1 || loadedPlayer.BestWinStreak != 1 || loadedPlayer.UnlockedDifficulty != negotiation.DifficultyEasy || len(loadedPlayer.Achievements) == 0 {
		t.Fatalf("player progression was not persisted: %+v", loadedPlayer)
	}
	loadedAbandonedSession, err := reopened.Session(ctx, abandonedSession.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedAbandonedSession.Status != domain.SessionStatusAbandoned || loadedAbandonedSession.State.Phase != domain.PhaseFinished {
		t.Fatalf("abandoned session was not persisted: %+v", loadedAbandonedSession)
	}
	loadedAbandonedResult, err := reopened.Result(ctx, abandonedSession.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedAbandonedResult.OutcomeCode != "abandoned" || loadedAbandonedResult.Analysis.AnalyzedTurns != 1 || len(loadedAbandonedResult.Analysis.ErrorClasses) != 1 || loadedAbandonedResult.Analysis.ErrorClasses[0].Code != negotiation.ErrorPressureTactic || loadedAbandonedResult.Analysis.ErrorClasses[0].Count != 1 {
		t.Fatalf("abandoned result was not persisted: %+v", loadedAbandonedResult)
	}
	loadedFork, err := reopened.Session(ctx, forkedSession.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedFork.PlayerID != player.ID || loadedFork.ParentSessionID != session.ID || loadedFork.ForkedFromTurn == nil || *loadedFork.ForkedFromTurn != 2 || loadedFork.Turn != 2 || loadedFork.TrustScore != 52 || loadedFork.ArgumentScore != 1 || loadedFork.PressureScore != 0 || !loadedFork.State.BATNADefined {
		t.Fatalf("fork metadata or state was not persisted: %+v", loadedFork)
	}
	forkMessages, err := reopened.Messages(ctx, forkedSession.ID)
	if err != nil {
		t.Fatal(err)
	}
	forkCheckpoints, err := reopened.Checkpoints(ctx, forkedSession.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(forkMessages) != 5 || len(forkCheckpoints) != 3 || forkCheckpoints[2].Turn != 2 {
		t.Fatalf("forked history was not persisted: messages=%+v checkpoints=%+v", forkMessages, forkCheckpoints)
	}
	page, err := reopened.ListSessions(ctx, repository.SessionFilter{ScenarioID: scenario.ID, Status: domain.SessionStatusActive, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].ID != forkedSession.ID || page.Items[0].PlayerID != player.ID || page.Items[0].ParentSessionID != session.ID || page.Items[0].ForkedFromTurn == nil || *page.Items[0].ForkedFromTurn != 2 {
		t.Fatalf("unexpected persisted session page: %+v", page)
	}
	statistics, err := reopened.SessionStatistics(ctx, scenario.ID)
	if err != nil {
		t.Fatal(err)
	}
	if statistics.Total != 3 || statistics.Active != 1 || statistics.Finished != 1 || statistics.Abandoned != 1 || len(statistics.Outcomes) != 2 {
		t.Fatalf("unexpected persisted statistics: %+v", statistics)
	}
}

func hasAchievement(items []domain.Achievement, code string) bool {
	for _, item := range items {
		if item.Code == code {
			return true
		}
	}
	return false
}
