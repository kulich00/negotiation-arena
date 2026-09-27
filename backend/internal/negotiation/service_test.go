package negotiation

import (
	"context"
	"errors"
	"testing"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
	"github.com/kulich00/negotiation-arena/backend/internal/llm"
	"github.com/kulich00/negotiation-arena/backend/internal/repository"
)

func TestNegotiationFlow(t *testing.T) {
	service := NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	if err := service.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	salary, err := service.repo.Scenario(context.Background(), "salary-negotiation")
	if err != nil {
		t.Fatal(err)
	}
	deadline, err := service.repo.Scenario(context.Background(), "project-deadline")
	if err != nil {
		t.Fatal(err)
	}
	if salary.Rules.Proposal.Kind != "raise_percent" || salary.Rules.Proposal.MaximumValue != 10 || deadline.Rules.Proposal.Kind != "extension_days" || deadline.Rules.MaxTurns != 10 {
		t.Fatalf("unexpected scenario rules: salary=%+v deadline=%+v", salary.Rules, deadline.Rules)
	}
	customScenario := validScenarioFixture()
	customScenario.ID = ""
	customScenario.Title = "Custom"
	customScenario.Rules = domain.ScenarioRules{}
	created, err := service.CreateScenario(context.Background(), customScenario)
	if err != nil {
		t.Fatal(err)
	}
	if created.Rules.MaxTurns != domain.DefaultScenarioRules().MaxTurns || !created.Rules.RequiresEvidence {
		t.Fatalf("missing default rules: %+v", created.Rules)
	}
	session, err := service.StartSession(context.Background(), "salary-negotiation")
	if err != nil {
		t.Fatal(err)
	}
	if session.State.Phase != domain.PhaseOpening {
		t.Fatalf("unexpected initial state: %+v", session.State)
	}
	turn, err := service.ProcessMessage(context.Background(), session.ID, "Предлагаю компромисс. Какие интересы для вас важны?")
	if err != nil {
		t.Fatal(err)
	}
	if turn.Session.TrustScore <= 50 {
		t.Fatalf("expected trust to grow, got %d", turn.Session.TrustScore)
	}
	if turn.Analysis.Intent != "legacy" || turn.Analysis.Technique == "" || turn.Analysis.Summary == "" {
		t.Fatalf("missing legacy turn analysis: %+v", turn.Analysis)
	}
	result, err := service.Finish(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.FinalScore == 0 {
		t.Fatal("expected non-zero score")
	}
	finished, err := service.Session(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if finished.State.Phase != domain.PhaseFinished {
		t.Fatalf("unexpected final state: %+v", finished.State)
	}
}

func TestProcessMovePersistsDeterministicState(t *testing.T) {
	service := NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	if err := service.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	session, err := service.StartSession(context.Background(), "salary-negotiation")
	if err != nil {
		t.Fatal(err)
	}

	turn, err := service.ProcessMove(context.Background(), session.ID, PlayerMove{
		Content: "Расскажите, что для вас важно.",
		Intent:  IntentAskInterest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if turn.Session.TrustScore != 52 || !turn.Session.State.InterestsExplored || turn.Session.State.Phase != domain.PhaseExploration {
		t.Fatalf("unexpected session after structured move: %+v", turn.Session)
	}
	if turn.Analysis.Technique != "harvard_interests" || turn.Analysis.TrustDelta != 2 {
		t.Fatalf("unexpected structured analysis: %+v", turn.Analysis)
	}

	turn, err = service.ProcessMove(context.Background(), session.ID, PlayerMove{
		Content: "Результаты за квартал выросли на 20%.",
		Intent:  IntentPresentEvidence,
	})
	if err != nil {
		t.Fatal(err)
	}

	turn, err = service.ProcessMove(context.Background(), session.ID, alternativeMove("review_in_3_months"))
	if err != nil {
		t.Fatal(err)
	}
	if !turn.Session.State.OfferMade || turn.Session.State.LastOfferID != "review_in_3_months" || turn.Session.State.Phase != domain.PhaseBargaining {
		t.Fatalf("proposal state was not persisted: %+v", turn.Session.State)
	}

	turn, err = service.ProcessMove(context.Background(), session.ID, PlayerMove{Content: "Согласен.", Intent: IntentAccept})
	if err != nil {
		t.Fatal(err)
	}
	if !turn.Session.State.OfferAccepted {
		t.Fatalf("accepted offer was not persisted: %+v", turn.Session.State)
	}
	messages, err := service.Messages(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 9 {
		t.Fatalf("unexpected message count: %d", len(messages))
	}
	for index, message := range messages {
		if message.Sender == "player" && (message.Analysis == nil || message.Analysis.Technique == "") {
			t.Fatalf("player message %d has no analysis: %+v", index, message)
		}
		if message.Sender == "opponent" && message.Analysis != nil {
			t.Fatalf("opponent message %d unexpectedly has analysis: %+v", index, message)
		}
	}

	result, err := service.Finish(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.Outcome != "Компромисс" || result.FinalScore != 66 {
		t.Fatalf("unexpected structured result: %+v", result)
	}
}

func TestProcessMoveRejectsAcceptWithoutOffer(t *testing.T) {
	service := NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	if err := service.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	session, err := service.StartSession(context.Background(), "salary-negotiation")
	if err != nil {
		t.Fatal(err)
	}

	_, err = service.ProcessMove(context.Background(), session.ID, PlayerMove{Content: "Согласен.", Intent: IntentAccept})
	if !errors.Is(err, ErrInvalidMove) {
		t.Fatalf("expected ErrInvalidMove, got %v", err)
	}
}

func TestProcessMessageHonorsTurnLimit(t *testing.T) {
	repo := repository.NewMemoryRepository()
	service := NewService(repo, llm.NewMockProvider())
	rules := domain.DefaultScenarioRules()
	rules.MaxTurns = 1
	scenario := validScenarioFixture()
	scenario.ID = "one-turn"
	scenario.Title = "One turn"
	scenario.Rules = rules
	if _, err := service.CreateScenario(context.Background(), scenario); err != nil {
		t.Fatal(err)
	}
	session, err := service.StartSession(context.Background(), "one-turn")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ProcessMessage(context.Background(), session.ID, "Первый ход"); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ProcessMessage(context.Background(), session.ID, "Второй ход"); !errors.Is(err, ErrTurnLimitReached) {
		t.Fatalf("expected ErrTurnLimitReached, got %v", err)
	}
}

func TestProcessMoveDoesNotCallLegacyProvider(t *testing.T) {
	service := NewService(repository.NewMemoryRepository(), failingProvider{})
	if err := service.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	session, err := service.StartSession(context.Background(), "salary-negotiation")
	if err != nil {
		t.Fatal(err)
	}

	turn, err := service.ProcessMove(context.Background(), session.ID, PlayerMove{
		Content: "Какие ограничения для вас важны?",
		Intent:  IntentAskInterest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if turn.Reply == "" {
		t.Fatal("expected deterministic opponent reply")
	}
}

type failingProvider struct{}

func (failingProvider) Analyze(context.Context, llm.AnalysisRequest) (llm.AnalysisResult, error) {
	return llm.AnalysisResult{}, errors.New("legacy provider must not be called")
}
