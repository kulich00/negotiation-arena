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
	if salary.Rules.Proposal.Kind != "raise_percent" || salary.Rules.Proposal.PreferredValue != 6 || salary.Rules.Proposal.MaximumValue != 10 || salary.Rules.Proposal.InputMaximumValue != 30 || deadline.Rules.Proposal.Kind != "extension_days" || deadline.Rules.MaxTurns != 10 || deadline.Rules.MinimumArgumentScoreForAgreement != 3 || deadline.Rules.MaximumPressureForAgreement != 2 || deadline.Rules.Behavior.Mode != OpponentModeDifficult || len(deadline.Rules.Behavior.PriorityShifts) != 2 {
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
	if result.Analysis.AnalyzedTurns != 1 || result.Analysis.BestMove == nil {
		t.Fatalf("missing session analysis: %+v", result.Analysis)
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
	if result.Analysis.AnalyzedTurns != 4 || len(result.Analysis.Techniques) != 4 {
		t.Fatalf("unexpected aggregate analysis: %+v", result.Analysis)
	}
	if result.Analysis.BestMove == nil || result.Analysis.BestMove.Turn != 2 || result.Analysis.BestMove.Technique != "evidence_based_argument" {
		t.Fatalf("unexpected best move: %+v", result.Analysis.BestMove)
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

func TestProcessMoveRejectsAcceptanceOfRejectedOffer(t *testing.T) {
	service := NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	if err := service.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	session, err := service.StartSession(context.Background(), "salary-negotiation")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ProcessMove(context.Background(), session.ID, PlayerMove{Content: "Что для вас важно?", Intent: IntentAskInterest}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.ProcessMove(context.Background(), session.ID, PlayerMove{Content: "Результаты выросли на 20%.", Intent: IntentPresentEvidence}); err != nil {
		t.Fatal(err)
	}
	turn, err := service.ProcessMove(context.Background(), session.ID, numericMove(20))
	if err != nil {
		t.Fatal(err)
	}
	if turn.Session.State.LastOfferQuality != domain.OfferQualityRejected {
		t.Fatalf("offer was not rejected: %+v", turn.Session.State)
	}
	if _, err := service.ProcessMove(context.Background(), session.ID, PlayerMove{Content: "Принимаю.", Intent: IntentAccept}); !errors.Is(err, ErrInvalidMove) {
		t.Fatalf("expected rejected offer acceptance error, got %v", err)
	}
}

func TestProcessMessageAutomaticallyFinishesAtTurnLimit(t *testing.T) {
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
	turn, err := service.ProcessMessage(context.Background(), session.ID, "Первый ход")
	if err != nil {
		t.Fatal(err)
	}
	if turn.Result == nil || turn.Session.Status != domain.SessionStatusFinished || turn.Session.State.Phase != domain.PhaseFinished {
		t.Fatalf("last turn did not finish the session: %+v", turn)
	}
	stored, err := service.Result(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stored.SessionID != session.ID || stored.OutcomeCode == "" {
		t.Fatalf("automatic result was not stored: %+v", stored)
	}
	repeated, err := service.Finish(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if repeated.OutcomeCode != stored.OutcomeCode || repeated.FinalScore != stored.FinalScore {
		t.Fatalf("idempotent finish returned a different result: first=%+v repeated=%+v", stored, repeated)
	}
	if _, err := service.ProcessMessage(context.Background(), session.ID, "Второй ход"); !errors.Is(err, repository.ErrConflict) {
		t.Fatalf("expected finished-session conflict, got %v", err)
	}
}

func TestAbandonPersistsIdempotentResult(t *testing.T) {
	service := NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	if err := service.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	session, err := service.StartSession(context.Background(), "salary-negotiation")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ProcessMove(context.Background(), session.ID, PlayerMove{Content: "Что для вас важно?", Intent: IntentAskInterest}); err != nil {
		t.Fatal(err)
	}

	result, err := service.Abandon(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if result.OutcomeCode != "abandoned" || result.Outcome != "Переговоры прерваны игроком" || result.Analysis.AnalyzedTurns != 1 {
		t.Fatalf("unexpected abandoned result: %+v", result)
	}
	loaded, err := service.Session(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Status != domain.SessionStatusAbandoned || loaded.State.Phase != domain.PhaseFinished {
		t.Fatalf("session was not abandoned: %+v", loaded)
	}
	repeated, err := service.Abandon(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if repeated.OutcomeCode != result.OutcomeCode || repeated.FinalScore != result.FinalScore {
		t.Fatalf("idempotent abandon returned a different result: first=%+v repeated=%+v", result, repeated)
	}
}

func TestForkSessionRestoresCheckpointAndKeepsParentUnchanged(t *testing.T) {
	service := NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	if err := service.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	parent, err := service.StartSession(context.Background(), "salary-negotiation")
	if err != nil {
		t.Fatal(err)
	}
	moves := []PlayerMove{
		{Content: "Что для вас важно?", Intent: IntentAskInterest},
		{Content: "Результат вырос на 20%.", Intent: IntentPresentEvidence},
		{Content: "Других вариантов у вас нет.", Intent: IntentPressure},
	}
	for _, move := range moves {
		if _, err := service.ProcessMove(context.Background(), parent.ID, move); err != nil {
			t.Fatal(err)
		}
	}

	child, err := service.ForkSession(context.Background(), parent.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if child.ParentSessionID != parent.ID || child.ForkedFromTurn == nil || *child.ForkedFromTurn != 2 || child.Turn != 2 || child.Status != domain.SessionStatusActive {
		t.Fatalf("unexpected fork metadata: %+v", child)
	}
	if child.TrustScore != 53 || child.ArgumentScore != 2 || child.PressureScore != 0 || !child.State.InterestsExplored || !child.State.EvidencePresented {
		t.Fatalf("checkpoint state was not restored: %+v", child)
	}
	childMessages, err := service.Messages(context.Background(), child.ID)
	if err != nil {
		t.Fatal(err)
	}
	childCheckpoints, err := service.Checkpoints(context.Background(), child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(childMessages) != 5 || len(childCheckpoints) != 3 {
		t.Fatalf("unexpected forked history: messages=%d checkpoints=%d", len(childMessages), len(childCheckpoints))
	}
	if _, err := service.ProcessMove(context.Background(), child.ID, alternativeMove("review_in_3_months")); err != nil {
		t.Fatal(err)
	}
	parentAfterFork, err := service.Session(context.Background(), parent.ID)
	if err != nil {
		t.Fatal(err)
	}
	if parentAfterFork.Turn != 3 || parentAfterFork.State.OfferMade {
		t.Fatalf("parent was changed by child branch: %+v", parentAfterFork)
	}
	if _, err := service.ForkSession(context.Background(), parent.ID, 99); !errors.Is(err, ErrInvalidFork) {
		t.Fatalf("expected invalid fork point, got %v", err)
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

func TestProcessMoveUsesReplyGeneratorWithoutChangingEvaluation(t *testing.T) {
	generator := &recordingReplyGenerator{reply: "Сформулированный LLM ответ"}
	service := NewService(
		repository.NewMemoryRepository(), llm.NewMockProvider(),
		WithReplyGenerator(generator),
	)
	if err := service.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	session, err := service.StartSession(context.Background(), "salary-negotiation")
	if err != nil {
		t.Fatal(err)
	}
	turn, err := service.ProcessMove(context.Background(), session.ID, PlayerMove{
		Content: "Какие ограничения бюджета для вас важны?",
		Intent:  IntentAskInterest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if turn.Reply != generator.reply {
		t.Fatalf("reply = %q", turn.Reply)
	}
	if turn.Analysis.TrustDelta == 0 || turn.Session.TrustScore != session.TrustScore+turn.Analysis.TrustDelta {
		t.Fatalf("LLM changed or bypassed deterministic evaluation: %+v", turn)
	}
	if generator.request.BaseReply == "" || generator.request.OpponentRole == "" || len(generator.request.History) != 1 {
		t.Fatalf("incomplete reply context: %+v", generator.request)
	}
}

func TestProcessMoveFallsBackWhenReplyGeneratorFails(t *testing.T) {
	service := NewService(
		repository.NewMemoryRepository(), llm.NewMockProvider(),
		WithReplyGenerator(failingReplyGenerator{}),
	)
	if err := service.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	session, err := service.StartSession(context.Background(), "salary-negotiation")
	if err != nil {
		t.Fatal(err)
	}
	turn, err := service.ProcessMove(context.Background(), session.ID, PlayerMove{
		Content: "Какие ограничения бюджета для вас важны?",
		Intent:  IntentAskInterest,
	})
	if err != nil {
		t.Fatal(err)
	}
	if turn.Reply == "" {
		t.Fatal("expected deterministic fallback reply")
	}
}

func TestProcessMessageUsesSemanticPressureInterpretation(t *testing.T) {
	service := NewService(
		repository.NewMemoryRepository(), llm.NewMockProvider(),
		WithMoveInterpreter(staticMoveInterpreter{interpretation: llm.MoveInterpretation{Intent: "pressure"}}),
	)
	if err := service.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	session, err := service.StartSession(context.Background(), "vendor-introduction")
	if err != nil {
		t.Fatal(err)
	}
	turn, err := service.ProcessMessage(context.Background(), session.ID, "Сделайте как я сказал, иначе пожалеете")
	if err != nil {
		t.Fatal(err)
	}
	if turn.Session.TrustScore != 48 || turn.Session.PressureScore != 2 {
		t.Fatalf("pressure did not reduce trust: %+v", turn.Session)
	}
	if turn.Analysis.Intent != "pressure" || turn.Analysis.Technique != "competitive_pressure" {
		t.Fatalf("unexpected semantic analysis: %+v", turn.Analysis)
	}
}

func TestProcessMessageFallsBackToLocalAnalysisWhenInterpretationFails(t *testing.T) {
	service := NewService(
		repository.NewMemoryRepository(), llm.NewMockProvider(),
		WithMoveInterpreter(failingMoveInterpreter{}),
	)
	if err := service.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	session, err := service.StartSession(context.Background(), "vendor-introduction")
	if err != nil {
		t.Fatal(err)
	}
	turn, err := service.ProcessMessage(context.Background(), session.ID, "Вы обязаны выполнить требование")
	if err != nil {
		t.Fatal(err)
	}
	if turn.Session.TrustScore != 49 || turn.Analysis.Intent != "legacy" {
		t.Fatalf("local fallback was not used: %+v", turn)
	}
}

func TestProcessMovePersistsSPINAndBATNAState(t *testing.T) {
	service := NewService(repository.NewMemoryRepository(), failingProvider{})
	if err := service.SeedDefaults(context.Background()); err != nil {
		t.Fatal(err)
	}
	session, err := service.StartSession(context.Background(), "salary-negotiation")
	if err != nil {
		t.Fatal(err)
	}

	moves := []PlayerMove{
		{Content: "Как сейчас устроен процесс пересмотра?", Intent: IntentAskSituation},
		{Content: "Что мешает пересмотреть условия сейчас?", Intent: IntentIdentifyProblem},
		{Content: "К чему приведёт сохранение текущих условий?", Intent: IntentExploreImplication},
		{Content: "Что даст компании согласованный план роста?", Intent: IntentClarifyNeedPayoff},
		{Content: "Без соглашения я продолжу оценивать другие варианты.", Intent: IntentStateBATNA},
	}
	for _, move := range moves {
		turn, err := service.ProcessMove(context.Background(), session.ID, move)
		if err != nil {
			t.Fatal(err)
		}
		if turn.Analysis.Technique == "" {
			t.Fatalf("missing analysis for %s", move.Intent)
		}
	}

	loaded, err := service.Session(context.Background(), session.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.State.SituationExplored || !loaded.State.ProblemIdentified || !loaded.State.ImplicationsExplored || !loaded.State.NeedPayoffEstablished || loaded.State.SPINStage != domain.SPINStageNeedPayoff || !loaded.State.BATNADefined {
		t.Fatalf("SPIN/BATNA state was not persisted: %+v", loaded.State)
	}
}

type failingProvider struct{}

func (failingProvider) Analyze(context.Context, llm.AnalysisRequest) (llm.AnalysisResult, error) {
	return llm.AnalysisResult{}, errors.New("legacy provider must not be called")
}

type recordingReplyGenerator struct {
	reply   string
	request llm.ReplyRequest
}

func (generator *recordingReplyGenerator) GenerateReply(_ context.Context, request llm.ReplyRequest) (string, error) {
	generator.request = request
	return generator.reply, nil
}

type failingReplyGenerator struct{}

func (failingReplyGenerator) GenerateReply(context.Context, llm.ReplyRequest) (string, error) {
	return "", errors.New("LLM unavailable")
}

type staticMoveInterpreter struct {
	interpretation llm.MoveInterpretation
}

func (interpreter staticMoveInterpreter) InterpretMove(context.Context, llm.InterpretationRequest) (llm.MoveInterpretation, error) {
	return interpreter.interpretation, nil
}

type failingMoveInterpreter struct{}

func (failingMoveInterpreter) InterpretMove(context.Context, llm.InterpretationRequest) (llm.MoveInterpretation, error) {
	return llm.MoveInterpretation{}, errors.New("LLM unavailable")
}
