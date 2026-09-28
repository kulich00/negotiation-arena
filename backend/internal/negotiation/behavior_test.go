package negotiation

import (
	"context"
	"reflect"
	"strings"
	"testing"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
	"github.com/kulich00/negotiation-arena/backend/internal/llm"
	"github.com/kulich00/negotiation-arena/backend/internal/repository"
)

func TestApplyOpponentBehaviorTracksMoodAndPriority(t *testing.T) {
	rules := difficultBehaviorRules()
	state := domain.InitialSessionState()
	InitializeOpponentState(&state, rules)
	if state.OpponentMode != OpponentModeDifficult || state.OpponentPriority != "срок" || state.OpponentMood != OpponentMoodNeutral {
		t.Fatalf("unexpected initial opponent state: %+v", state)
	}

	pressureMove := PlayerMove{Intent: IntentPressure}
	pressure := ApplyOpponentBehavior(pressureMove, 1, rules, EvaluateMove(pressureMove, state, rules))
	if pressure.TrustDelta != -4 || pressure.OpponentReaction == nil || pressure.OpponentReaction.Mood != OpponentMoodIrritated || pressure.OpponentReaction.TrustDelta != -2 {
		t.Fatalf("unexpected emotional reaction: %+v", pressure)
	}

	askMove := PlayerMove{Intent: IntentAskInterest}
	deescalated := ApplyOpponentBehavior(askMove, 2, rules, EvaluateMove(askMove, pressure.State, rules))
	if deescalated.TrustDelta != 3 || deescalated.OpponentReaction == nil || deescalated.OpponentReaction.Mood != OpponentMoodReceptive {
		t.Fatalf("unexpected de-escalation reaction: %+v", deescalated)
	}

	evidenceMove := PlayerMove{Intent: IntentPresentEvidence}
	shifted := ApplyOpponentBehavior(evidenceMove, 3, rules, EvaluateMove(evidenceMove, deescalated.State, rules))
	if shifted.TrustDelta != 0 || shifted.State.OpponentPriority != "риски" || shifted.State.AppliedPriorityShifts != 1 || shifted.OpponentReaction == nil || !shifted.OpponentReaction.PriorityChanged || shifted.OpponentReaction.Mood != OpponentMoodUncertain {
		t.Fatalf("unexpected priority shift: %+v", shifted)
	}
	reply := GenerateOpponentReply(evidenceMove, domain.Session{State: deescalated.State}, rules, shifted)
	if !strings.Contains(reply, "теперь для меня важнее «риски»") {
		t.Fatalf("priority shift is missing from reply: %q", reply)
	}

	guardedState := shifted.State
	guarded := ApplyOpponentBehavior(evidenceMove, 6, rules, EvaluateMove(evidenceMove, guardedState, rules))
	if guarded.OpponentReaction == nil || guarded.OpponentReaction.Mood != OpponentMoodGuarded || guarded.TrustDelta != 0 {
		t.Fatalf("unexpected volatile reaction: %+v", guarded)
	}
}

func TestDifficultBehaviorIsReproducibleAfterFork(t *testing.T) {
	ctx := context.Background()
	service := NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	if err := service.SeedDefaults(ctx); err != nil {
		t.Fatal(err)
	}
	parent, err := service.StartSession(ctx, "project-deadline")
	if err != nil {
		t.Fatal(err)
	}
	firstMoves := []PlayerMove{
		{Content: "Иного варианта у вас нет.", Intent: IntentPressure},
		{Content: "Что для вас сейчас важнее всего?", Intent: IntentAskInterest},
	}
	for _, move := range firstMoves {
		if _, err := service.ProcessMove(ctx, parent.ID, move); err != nil {
			t.Fatal(err)
		}
	}
	thirdMove := PlayerMove{Content: "Поэтапный запуск снизит риск на 20%.", Intent: IntentPresentEvidence}
	original, err := service.ProcessMove(ctx, parent.ID, thirdMove)
	if err != nil {
		t.Fatal(err)
	}
	child, err := service.ForkSession(ctx, parent.ID, 2)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := service.ProcessMove(ctx, child.ID, thirdMove)
	if err != nil {
		t.Fatal(err)
	}
	if original.Reply != replayed.Reply || original.Session.TrustScore != replayed.Session.TrustScore || !reflect.DeepEqual(original.Analysis.OpponentReaction, replayed.Analysis.OpponentReaction) {
		t.Fatalf("fork changed deterministic reaction:\noriginal=%+v\nreplayed=%+v", original, replayed)
	}
}

func TestDifficultBehaviorAppliesToLegacyTextMode(t *testing.T) {
	ctx := context.Background()
	service := NewService(repository.NewMemoryRepository(), llm.NewMockProvider())
	if err := service.SeedDefaults(ctx); err != nil {
		t.Fatal(err)
	}
	session, err := service.StartSession(ctx, "project-deadline")
	if err != nil {
		t.Fatal(err)
	}
	turn, err := service.ProcessMessage(ctx, session.ID, "Вы обязаны согласиться, иначе мы уйдём.")
	if err != nil {
		t.Fatal(err)
	}
	if turn.Session.TrustScore != 47 || turn.Session.PressureScore != 2 || turn.Session.State.OpponentMood != OpponentMoodIrritated {
		t.Fatalf("legacy scores did not include difficult behavior: %+v", turn.Session)
	}
	if turn.Analysis.OpponentReaction == nil || turn.Analysis.OpponentReaction.TrustDelta != -2 || !strings.HasPrefix(turn.Reply, "Меня раздражает такой тон.") {
		t.Fatalf("legacy reaction is missing: %+v", turn)
	}
}

func difficultBehaviorRules() domain.ScenarioRules {
	rules := domain.DefaultScenarioRules()
	rules.Behavior = domain.OpponentBehavior{
		Mode: OpponentModeDifficult, Emotionality: 2, Volatility: 2,
		InitialPriority: "срок",
		PriorityShifts:  []domain.OpponentPriorityShift{{Turn: 3, Priority: "риски"}},
	}
	return rules
}
