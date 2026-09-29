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
		Kind:              "raise_percent",
		PreferredValue:    5,
		MaximumValue:      10,
		InputMaximumValue: 20,
		AlternativeIDs:    []string{"review_in_3_months"},
	}

	tests := []struct {
		name    string
		move    PlayerMove
		wantErr bool
	}{
		{name: "ask interests", move: PlayerMove{Content: "Что для вас важно?", Intent: IntentAskInterest}},
		{name: "neutral", move: PlayerMove{Content: "Здравствуйте", Intent: IntentNeutral}},
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
		{name: "numeric above concession limit", move: numericMove(11)},
		{name: "numeric above input maximum", move: numericMove(21), wantErr: true},
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
	rules := domain.DefaultScenarioRules()

	ask := EvaluateMove(PlayerMove{Intent: IntentAskInterest}, state, rules)
	if ask.TrustDelta != 2 || !ask.State.InterestsExplored || ask.State.Phase != domain.PhaseExploration {
		t.Fatalf("unexpected ask evaluation: %+v", ask)
	}

	evidence := EvaluateMove(PlayerMove{Intent: IntentPresentEvidence}, ask.State, rules)
	if evidence.TrustDelta != 1 || evidence.ArgumentDelta != 2 || !evidence.State.EvidencePresented {
		t.Fatalf("unexpected evidence evaluation: %+v", evidence)
	}

	proposal := EvaluateMove(alternativeMove("review_in_3_months"), evidence.State, rules)
	if !proposal.State.OfferMade || proposal.State.Phase != domain.PhaseBargaining || proposal.State.LastOfferID != "review_in_3_months" {
		t.Fatalf("unexpected proposal evaluation: %+v", proposal)
	}

	pressure := EvaluateMove(PlayerMove{Intent: IntentPressure}, proposal.State, rules)
	if pressure.TrustDelta != -2 || pressure.PressureDelta != 2 || pressure.State.Phase != domain.PhaseBargaining {
		t.Fatalf("unexpected pressure evaluation: %+v", pressure)
	}

	neutral := EvaluateMove(PlayerMove{Intent: IntentNeutral}, state, rules)
	if neutral.TrustDelta != 0 || neutral.ArgumentDelta != 0 || neutral.PressureDelta != 0 {
		t.Fatalf("neutral move changed scores: %+v", neutral)
	}
}

func TestFreeFormProposalForDefaultScenario(t *testing.T) {
	rules := domain.DefaultScenarioRules()
	move := PlayerMove{Content: "Предлагаю обсудить взаимовыгодные условия", Intent: IntentPropose}

	if err := ValidateMove(move, rules); err != nil {
		t.Fatal(err)
	}
	evaluation := EvaluateMove(move, domain.InitialSessionState(), rules)
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
	rules := domain.DefaultScenarioRules()

	situation := EvaluateMove(PlayerMove{Intent: IntentAskSituation}, state, rules)
	if situation.TrustDelta != 1 || !situation.State.SituationExplored || situation.State.SPINStage != domain.SPINStageSituation || situation.State.Phase != domain.PhaseExploration {
		t.Fatalf("unexpected situation evaluation: %+v", situation)
	}
	problem := EvaluateMove(PlayerMove{Intent: IntentIdentifyProblem}, situation.State, rules)
	if problem.TrustDelta != 1 || problem.ArgumentDelta != 1 || !problem.State.ProblemIdentified || problem.State.SPINStage != domain.SPINStageProblem {
		t.Fatalf("unexpected problem evaluation: %+v", problem)
	}
	implication := EvaluateMove(PlayerMove{Intent: IntentExploreImplication}, problem.State, rules)
	if implication.ArgumentDelta != 2 || !implication.State.ImplicationsExplored || implication.State.SPINStage != domain.SPINStageImplication {
		t.Fatalf("unexpected implication evaluation: %+v", implication)
	}
	needPayoff := EvaluateMove(PlayerMove{Intent: IntentClarifyNeedPayoff}, implication.State, rules)
	if needPayoff.TrustDelta != 2 || needPayoff.ArgumentDelta != 1 || !needPayoff.State.NeedPayoffEstablished || needPayoff.State.SPINStage != domain.SPINStageNeedPayoff {
		t.Fatalf("unexpected need-payoff evaluation: %+v", needPayoff)
	}
	preparedState := needPayoff.State
	preparedState.InterestsExplored = true
	batna := EvaluateMove(PlayerMove{Intent: IntentStateBATNA}, preparedState, rules)
	if batna.ArgumentDelta != 1 || !batna.State.BATNADefined {
		t.Fatalf("unexpected BATNA evaluation: %+v", batna)
	}
}

func TestEvaluateImplicationBeforeProblemHasReducedImpact(t *testing.T) {
	rules := domain.DefaultScenarioRules()
	evaluation := EvaluateMove(PlayerMove{Intent: IntentExploreImplication}, domain.InitialSessionState(), rules)
	if evaluation.ArgumentDelta != 0 || !evaluation.State.ImplicationsExplored || evaluation.State.SPINStage != domain.SPINStageNone {
		t.Fatalf("unexpected out-of-order implication evaluation: %+v", evaluation)
	}

	problem := EvaluateMove(PlayerMove{Intent: IntentIdentifyProblem}, domain.InitialSessionState(), rules)
	if problem.TrustDelta != 0 || problem.ArgumentDelta != 0 || problem.State.SPINStage != domain.SPINStageNone {
		t.Fatalf("unexpected out-of-order problem evaluation: %+v", problem)
	}

	needPayoff := EvaluateMove(PlayerMove{Intent: IntentClarifyNeedPayoff}, domain.InitialSessionState(), rules)
	if needPayoff.TrustDelta != 0 || needPayoff.ArgumentDelta != 0 || needPayoff.State.SPINStage != domain.SPINStageNone {
		t.Fatalf("unexpected out-of-order need-payoff evaluation: %+v", needPayoff)
	}
}

func TestEvaluateMoveRequiresContextForEvidenceBATNAAndProposal(t *testing.T) {
	rules := domain.DefaultScenarioRules()

	evidence := EvaluateMove(PlayerMove{Intent: IntentPresentEvidence}, domain.InitialSessionState(), rules)
	if evidence.TrustDelta != 0 || evidence.ArgumentDelta != 1 {
		t.Fatalf("evidence before interests received full reward: %+v", evidence)
	}
	batna := EvaluateMove(PlayerMove{Intent: IntentStateBATNA}, domain.InitialSessionState(), rules)
	if batna.ArgumentDelta != 0 {
		t.Fatalf("BATNA before interests changed argument score: %+v", batna)
	}
	proposal := EvaluateMove(PlayerMove{Content: "Предлагаю условия", Intent: IntentPropose}, domain.InitialSessionState(), rules)
	if proposal.TrustDelta != 0 {
		t.Fatalf("premature proposal changed trust: %+v", proposal)
	}
}

func TestEvaluateMoveDoesNotRewardCompletedStepAgain(t *testing.T) {
	rules := domain.DefaultScenarioRules()
	first := EvaluateMove(PlayerMove{Intent: IntentAskInterest}, domain.InitialSessionState(), rules)
	second := EvaluateMove(PlayerMove{Intent: IntentAskInterest}, first.State, rules)
	if second.TrustDelta != 0 || !second.Repeated {
		t.Fatalf("completed interest step was rewarded twice: %+v", second)
	}

	prepared := first.State
	firstEvidence := EvaluateMove(PlayerMove{Intent: IntentPresentEvidence}, prepared, rules)
	additionalEvidence := EvaluateMove(PlayerMove{Intent: IntentPresentEvidence}, firstEvidence.State, rules)
	if additionalEvidence.TrustDelta != 0 || additionalEvidence.ArgumentDelta != 1 {
		t.Fatalf("additional evidence received the initial reward again: %+v", additionalEvidence)
	}
}

func TestEvaluateMoveDoesNotRewardSameOfferTwice(t *testing.T) {
	rules := domain.DefaultScenarioRules()
	prepared := domain.SessionState{InterestsExplored: true, EvidencePresented: true}
	first := EvaluateMove(PlayerMove{Intent: IntentPropose}, prepared, rules)
	second := EvaluateMove(PlayerMove{Intent: IntentPropose}, first.State, rules)
	if first.TrustDelta != 1 || second.TrustDelta != 0 || !second.Repeated {
		t.Fatalf("unexpected repeated offer evaluation: first=%+v second=%+v", first, second)
	}
}

func TestEvaluateProposalQuality(t *testing.T) {
	rules := domain.DefaultScenarioRules()
	rules.Proposal = domain.ProposalConstraint{
		Kind:                    "raise_percent",
		PreferredValue:          5,
		MaximumValue:            10,
		InputMaximumValue:       20,
		AlternativeIDs:          []string{"review_later", "extra_budget"},
		PreferredAlternativeIDs: []string{"review_later"},
	}
	tests := []struct {
		name string
		move PlayerMove
		want domain.OfferQuality
	}{
		{name: "preferred numeric", move: numericMove(5), want: domain.OfferQualityPreferred},
		{name: "acceptable numeric", move: numericMove(8), want: domain.OfferQualityAcceptable},
		{name: "rejected numeric", move: numericMove(15), want: domain.OfferQualityRejected},
		{name: "preferred alternative", move: alternativeMove("review_later"), want: domain.OfferQualityPreferred},
		{name: "acceptable alternative", move: alternativeMove("extra_budget"), want: domain.OfferQualityAcceptable},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			evaluation := EvaluateMove(test.move, domain.InitialSessionState(), rules)
			if evaluation.State.LastOfferQuality != test.want {
				t.Fatalf("offer quality = %q, want %q", evaluation.State.LastOfferQuality, test.want)
			}
		})
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
