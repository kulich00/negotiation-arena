package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
	"github.com/kulich00/negotiation-arena/backend/internal/repository"
)

func TestMemoryRepositoryPreservesRulesAndState(t *testing.T) {
	ctx := context.Background()
	repo := repository.NewMemoryRepository()
	rules := domain.DefaultScenarioRules()
	rules.Proposal = domain.ProposalConstraint{Kind: "raise_percent", MaximumValue: 10, AlternativeIDs: []string{"review_later"}, PreferredAlternativeIDs: []string{"review_later"}}
	scenario := domain.Scenario{ID: "salary", Rules: rules}
	if err := repo.SaveScenario(ctx, scenario); err != nil {
		t.Fatal(err)
	}
	scenario.Rules.Proposal.AlternativeIDs[0] = "changed"
	scenario.Rules.Proposal.PreferredAlternativeIDs[0] = "changed"
	loadedScenario, err := repo.Scenario(ctx, "salary")
	if err != nil {
		t.Fatal(err)
	}
	if loadedScenario.Rules.Proposal.AlternativeIDs[0] != "review_later" {
		t.Fatal("scenario rules changed after saving")
	}
	if loadedScenario.Rules.Proposal.PreferredAlternativeIDs[0] != "review_later" {
		t.Fatal("preferred scenario alternatives changed after saving")
	}

	session := domain.Session{ID: "session", ScenarioID: "salary", Status: "active", StartedAt: time.Now().UTC(), State: domain.InitialSessionState()}
	if err := repo.SaveSession(ctx, session); err != nil {
		t.Fatal(err)
	}
	session.Turn = 1
	session.State.Phase = domain.PhaseExploration
	session.State.InterestsExplored = true
	analysis := domain.TurnAnalysis{
		Intent: "ask_interest", Technique: "harvard_interests", Strengths: []string{"Открытый вопрос"}, Risks: []string{},
		Errors: []domain.NegotiationError{{Code: "sample_error", Label: "Ошибка", Severity: domain.ErrorSeverityLow, Message: "Описание"}},
	}
	if err := repo.ApplyTurn(ctx, session, 0, "Какие интересы?", "Обсудим бюджет", analysis); err != nil {
		t.Fatal(err)
	}
	loadedSession, err := repo.Session(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loadedSession.State.Phase != domain.PhaseExploration || !loadedSession.State.InterestsExplored {
		t.Fatalf("state was not saved: %+v", loadedSession.State)
	}
	messages, err := repo.Messages(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 3 || messages[1].Analysis == nil || messages[1].Analysis.Technique != "harvard_interests" || messages[2].Analysis != nil {
		t.Fatalf("analysis was not saved on the player message: %+v", messages)
	}
	messages[1].Analysis.Strengths[0] = "changed"
	messages[1].Analysis.Errors[0].Code = "changed"
	reloadedMessages, err := repo.Messages(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloadedMessages[1].Analysis.Strengths[0] != "Открытый вопрос" || reloadedMessages[1].Analysis.Errors[0].Code != "sample_error" {
		t.Fatal("stored message analysis was mutated through returned data")
	}
	checkpoints, err := repo.Checkpoints(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(checkpoints) != 2 || checkpoints[0].Turn != 0 || checkpoints[1].Turn != 1 || !checkpoints[1].State.InterestsExplored {
		t.Fatalf("unexpected checkpoints: %+v", checkpoints)
	}
	checkpoints[1].State.InterestsExplored = false
	reloadedCheckpoints, err := repo.Checkpoints(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !reloadedCheckpoints[1].State.InterestsExplored {
		t.Fatal("stored checkpoint was mutated through returned data")
	}

	session.Status = "finished"
	finishedAt := time.Now().UTC()
	session.FinishedAt = &finishedAt
	result := domain.Result{
		SessionID:       session.ID,
		FinalScore:      70,
		OutcomeCode:     "advantageous_agreement",
		Strengths:       []string{"Сильная сторона"},
		Mistakes:        []string{},
		Recommendations: []string{"Рекомендация"},
		Achievements:    []domain.Achievement{{Code: "first_round", Title: "Первый раунд"}},
		Analysis: domain.SessionAnalysis{
			Techniques:              []domain.TechniqueUsage{{Technique: "harvard_interests", Count: 1}},
			ErrorClasses:            []domain.ErrorClass{{Code: "sample_error", Count: 1, Turns: []int{1}}},
			RepeatedRisks:           []domain.RepeatedRisk{},
			PriorityRecommendations: []string{"Рекомендация"},
			BestMove:                &domain.BestMoveInsight{Turn: 1, Technique: "harvard_interests"},
		},
	}
	if err := repo.Finish(ctx, session, result); err != nil {
		t.Fatal(err)
	}
	loadedResult, err := repo.Result(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	loadedResult.Strengths[0] = "changed"
	loadedResult.Analysis.Techniques[0].Technique = "changed"
	loadedResult.Achievements[0].Code = "changed"
	loadedResult.Analysis.ErrorClasses[0].Turns[0] = 99
	loadedResult.Analysis.PriorityRecommendations[0] = "changed"
	loadedResult.Analysis.BestMove.Technique = "changed"
	reloadedResult, err := repo.Result(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloadedResult.Strengths[0] != "Сильная сторона" || reloadedResult.Achievements[0].Code != "first_round" || reloadedResult.Analysis.Techniques[0].Technique != "harvard_interests" || reloadedResult.Analysis.ErrorClasses[0].Turns[0] != 1 || reloadedResult.Analysis.PriorityRecommendations[0] != "Рекомендация" || reloadedResult.Analysis.BestMove.Technique != "harvard_interests" {
		t.Fatalf("stored result was mutated through returned data: %+v", reloadedResult)
	}

	page, err := repo.ListSessions(ctx, repository.SessionFilter{Status: domain.SessionStatusFinished, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Items) != 1 || page.Items[0].FinalScore == nil || *page.Items[0].FinalScore != 70 || page.Items[0].OutcomeCode != "advantageous_agreement" {
		t.Fatalf("unexpected session page: %+v", page)
	}
	*page.Items[0].FinishedAt = time.Time{}
	reloadedSession, err := repo.Session(ctx, session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if reloadedSession.FinishedAt == nil || reloadedSession.FinishedAt.IsZero() {
		t.Fatal("stored finish time was mutated through session summary")
	}
	statistics, err := repo.SessionStatistics(ctx, "salary")
	if err != nil {
		t.Fatal(err)
	}
	if statistics.Total != 1 || statistics.Finished != 1 || statistics.AverageFinalScore != 70 || len(statistics.Outcomes) != 1 || statistics.Outcomes[0].OutcomeCode != "advantageous_agreement" {
		t.Fatalf("unexpected session statistics: %+v", statistics)
	}
}
