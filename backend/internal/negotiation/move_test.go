package negotiation

import (
	"errors"
	"strings"
	"testing"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
)

func TestValidateMove(t *testing.T) {
	rules := domain.DefaultScenarioRules()
	rules.Proposal = domain.ProposalConstraint{
		Kind:           "raise_percent",
		MaximumValue:   10,
		AlternativeIDs: []string{"review_in_3_months"},
	}

	tests := []struct {
		name    string
		move    PlayerMove
		wantErr bool
	}{
		{name: "ask interests", move: PlayerMove{Content: "Что для вас важно?", Intent: IntentAskInterest}},
		{name: "ask situation", move: PlayerMove{Content: "Как устроен текущий процесс?", Intent: IntentAskSituation}},
		{name: "identify problem", move: PlayerMove{Content: "Что мешает получить результат?", Intent: IntentIdentifyProblem}},
		{name: "explore implication", move: PlayerMove{Content: "К чему приведёт задержка?", Intent: IntentExploreImplication}},
		{name: "clarify need payoff", move: PlayerMove{Content: "Что даст решение проблемы?", Intent: IntentClarifyNeedPayoff}},
		{name: "state BATNA", move: PlayerMove{Content: "Если не договоримся, перенесём объём на следующий квартал.", Intent: IntentStateBATNA}},
		{name: "blank content", move: PlayerMove{Content: "  ", Intent: IntentAskInterest}, wantErr: true},
		{name: "content too long", move: PlayerMove{Content: strings.Repeat("я", maxMoveContentLength+1), Intent: IntentAskInterest}, wantErr: true},
		{name: "unknown intent", move: PlayerMove{Content: "Текст", Intent: "unknown"}, wantErr: true},
		{name: "proposal required", move: PlayerMove{Content: "Предлагаю", Intent: IntentPropose}, wantErr: true},
		{name: "proposal forbidden", move: PlayerMove{Content: "Вопрос", Intent: IntentAskInterest, Proposal: &MoveProposal{}}, wantErr: true},
		{name: "numeric minimum", move: numericMove(1)},
		{name: "numeric maximum", move: numericMove(10)},
		{name: "numeric zero", move: numericMove(0), wantErr: true},
		{name: "numeric above maximum", move: numericMove(11), wantErr: true},
		{name: "wrong numeric kind", move: PlayerMove{Content: "Предлагаю", Intent: IntentPropose, Proposal: &MoveProposal{Kind: "extension_days", Value: 5}}, wantErr: true},
		{name: "known alternative", move: alternativeMove("review_in_3_months")},
		{name: "unknown alternative", move: alternativeMove("unknown"), wantErr: true},
		{name: "mixed alternative", move: PlayerMove{Content: "Предлагаю", Intent: IntentPropose, Proposal: &MoveProposal{Kind: "raise_percent", Value: 5, AlternativeID: "review_in_3_months"}}, wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := ValidateMove(test.move, rules)
			if (err != nil) != test.wantErr {
				t.Fatalf("ValidateMove() error = %v, wantErr %v", err, test.wantErr)
			}
			if test.wantErr && !errors.Is(err, ErrInvalidMove) {
				t.Fatalf("expected ErrInvalidMove, got %v", err)
			}
		})
	}
}

func TestEvaluateMoveUpdatesStateAndScores(t *testing.T) {
	state := domain.InitialSessionState()

	ask := EvaluateMove(PlayerMove{Intent: IntentAskInterest}, state)
	if ask.TrustDelta != 2 || !ask.State.InterestsExplored || ask.State.Phase != domain.PhaseExploration {
		t.Fatalf("unexpected ask evaluation: %+v", ask)
	}

	evidence := EvaluateMove(PlayerMove{Intent: IntentPresentEvidence}, ask.State)
	if evidence.TrustDelta != 1 || evidence.ArgumentDelta != 2 || !evidence.State.EvidencePresented {
		t.Fatalf("unexpected evidence evaluation: %+v", evidence)
	}

	proposal := EvaluateMove(alternativeMove("review_in_3_months"), evidence.State)
	if !proposal.State.OfferMade || proposal.State.Phase != domain.PhaseBargaining || proposal.State.LastOfferID != "review_in_3_months" {
		t.Fatalf("unexpected proposal evaluation: %+v", proposal)
	}

	pressure := EvaluateMove(PlayerMove{Intent: IntentPressure}, proposal.State)
	if pressure.TrustDelta != -2 || pressure.PressureDelta != 2 || pressure.State.Phase != domain.PhaseBargaining {
		t.Fatalf("unexpected pressure evaluation: %+v", pressure)
	}
}

func TestFreeFormProposalForDefaultScenario(t *testing.T) {
	rules := domain.DefaultScenarioRules()
	move := PlayerMove{Content: "Предлагаю обсудить взаимовыгодные условия", Intent: IntentPropose}

	if err := ValidateMove(move, rules); err != nil {
		t.Fatal(err)
	}
	evaluation := EvaluateMove(move, domain.InitialSessionState())
	if !evaluation.State.OfferMade || evaluation.State.LastOfferID != "free_form" {
		t.Fatalf("unexpected free-form proposal state: %+v", evaluation.State)
	}

	move.Proposal = &MoveProposal{Kind: "none", Value: 1}
	if err := ValidateMove(move, rules); !errors.Is(err, ErrInvalidMove) {
		t.Fatalf("expected proposal details to be rejected, got %v", err)
	}
}

func TestEvaluateMoveTracksSPINAndBATNA(t *testing.T) {
	state := domain.InitialSessionState()

	situation := EvaluateMove(PlayerMove{Intent: IntentAskSituation}, state)
	if situation.TrustDelta != 1 || !situation.State.SituationExplored || situation.State.SPINStage != domain.SPINStageSituation || situation.State.Phase != domain.PhaseExploration {
		t.Fatalf("unexpected situation evaluation: %+v", situation)
	}
	problem := EvaluateMove(PlayerMove{Intent: IntentIdentifyProblem}, situation.State)
	if problem.TrustDelta != 1 || problem.ArgumentDelta != 1 || !problem.State.ProblemIdentified || problem.State.SPINStage != domain.SPINStageProblem {
		t.Fatalf("unexpected problem evaluation: %+v", problem)
	}
	implication := EvaluateMove(PlayerMove{Intent: IntentExploreImplication}, problem.State)
	if implication.ArgumentDelta != 2 || !implication.State.ImplicationsExplored || implication.State.SPINStage != domain.SPINStageImplication {
		t.Fatalf("unexpected implication evaluation: %+v", implication)
	}
	needPayoff := EvaluateMove(PlayerMove{Intent: IntentClarifyNeedPayoff}, implication.State)
	if needPayoff.TrustDelta != 2 || needPayoff.ArgumentDelta != 1 || !needPayoff.State.NeedPayoffEstablished || needPayoff.State.SPINStage != domain.SPINStageNeedPayoff {
		t.Fatalf("unexpected need-payoff evaluation: %+v", needPayoff)
	}
	batna := EvaluateMove(PlayerMove{Intent: IntentStateBATNA}, needPayoff.State)
	if batna.ArgumentDelta != 1 || !batna.State.BATNADefined {
		t.Fatalf("unexpected BATNA evaluation: %+v", batna)
	}
}

func TestEvaluateImplicationBeforeProblemHasReducedImpact(t *testing.T) {
	evaluation := EvaluateMove(PlayerMove{Intent: IntentExploreImplication}, domain.InitialSessionState())
	if evaluation.ArgumentDelta != 1 || !evaluation.State.ImplicationsExplored || evaluation.State.SPINStage != domain.SPINStageNone {
		t.Fatalf("unexpected out-of-order implication evaluation: %+v", evaluation)
	}

	problem := EvaluateMove(PlayerMove{Intent: IntentIdentifyProblem}, domain.InitialSessionState())
	if problem.TrustDelta != 0 || problem.ArgumentDelta != 1 || problem.State.SPINStage != domain.SPINStageNone {
		t.Fatalf("unexpected out-of-order problem evaluation: %+v", problem)
	}

	needPayoff := EvaluateMove(PlayerMove{Intent: IntentClarifyNeedPayoff}, domain.InitialSessionState())
	if needPayoff.TrustDelta != 1 || needPayoff.ArgumentDelta != 0 || needPayoff.State.SPINStage != domain.SPINStageNone {
		t.Fatalf("unexpected out-of-order need-payoff evaluation: %+v", needPayoff)
	}
}

func numericMove(value int) PlayerMove {
	return PlayerMove{
		Content:  "Предлагаю повышение",
		Intent:   IntentPropose,
		Proposal: &MoveProposal{Kind: "raise_percent", Value: value},
	}
}

func alternativeMove(id string) PlayerMove {
	return PlayerMove{
		Content:  "Предлагаю альтернативу",
		Intent:   IntentPropose,
		Proposal: &MoveProposal{AlternativeID: id},
	}
}
