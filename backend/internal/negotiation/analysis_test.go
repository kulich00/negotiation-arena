package negotiation

import (
	"testing"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
	"github.com/kulich00/negotiation-arena/backend/internal/llm"
)

func TestAnalyzeStructuredMoveExplainsMissingPreparation(t *testing.T) {
	rules := domain.DefaultScenarioRules()
	move := PlayerMove{Content: "Предлагаю вариант", Intent: IntentPropose}
	evaluation := EvaluateMove(move, domain.InitialSessionState(), rules)

	analysis := AnalyzeStructuredMove(move, domain.InitialSessionState(), rules, evaluation)
	if analysis.Technique != "harvard_mutual_gain" || analysis.TrustDelta != 0 {
		t.Fatalf("unexpected analysis: %+v", analysis)
	}
	if len(analysis.Risks) != 2 || analysis.Recommendation == "" {
		t.Fatalf("missing proposal risks: %+v", analysis)
	}
	if len(analysis.Errors) != 2 || analysis.Errors[0].Code != ErrorProposalBeforeInterests || analysis.Errors[1].Code != ErrorProposalWithoutEvidence {
		t.Fatalf("missing stable proposal error classes: %+v", analysis.Errors)
	}
}

func TestAnalyzeStructuredMoveClassifiesRejectedProposal(t *testing.T) {
	rules := domain.DefaultScenarioRules()
	rules.Proposal = domain.ProposalConstraint{
		Kind: "raise_percent", PreferredValue: 5, MaximumValue: 10, InputMaximumValue: 20,
	}
	state := domain.SessionState{InterestsExplored: true, EvidencePresented: true}
	move := PlayerMove{
		Content: "Предлагаю повышение на 15%", Intent: IntentPropose,
		Proposal: &MoveProposal{Kind: "raise_percent", Value: 15},
	}
	evaluation := EvaluateMove(move, state, rules)

	analysis := AnalyzeStructuredMove(move, state, rules, evaluation)
	if len(analysis.Errors) != 1 || analysis.Errors[0].Code != ErrorProposalOutsideLimit || analysis.Errors[0].Severity != domain.ErrorSeverityCritical {
		t.Fatalf("unexpected rejected proposal errors: %+v", analysis.Errors)
	}
}

func TestAnalyzeLegacyMoveRecognizesPressure(t *testing.T) {
	analysis := AnalyzeLegacyMove(llm.AnalysisResult{TrustDelta: -1, PressureDelta: 2})
	if analysis.Technique != "competitive_pressure" || len(analysis.Risks) != 1 || analysis.PressureDelta != 2 || len(analysis.Errors) != 1 || analysis.Errors[0].Code != ErrorPressureTactic {
		t.Fatalf("unexpected legacy analysis: %+v", analysis)
	}
}

func TestAnalyzeStructuredMoveExplainsSPINOrder(t *testing.T) {
	rules := domain.DefaultScenarioRules()
	move := PlayerMove{Content: "К чему приведёт проблема?", Intent: IntentExploreImplication}
	evaluation := EvaluateMove(move, domain.InitialSessionState(), rules)

	analysis := AnalyzeStructuredMove(move, domain.InitialSessionState(), rules, evaluation)
	if analysis.Technique != "spin_implication" || analysis.ArgumentDelta != 0 {
		t.Fatalf("unexpected analysis: %+v", analysis)
	}
	if len(analysis.Risks) != 1 || analysis.Recommendation != "Сначала согласуйте, какую именно проблему необходимо решить." {
		t.Fatalf("missing SPIN order explanation: %+v", analysis)
	}
}

func TestAnalyzeStructuredMoveExplainsPreparedBATNA(t *testing.T) {
	rules := domain.DefaultScenarioRules()
	state := domain.SessionState{InterestsExplored: true}
	move := PlayerMove{Content: "У нас есть альтернативный вариант.", Intent: IntentStateBATNA}
	evaluation := EvaluateMove(move, state, rules)

	analysis := AnalyzeStructuredMove(move, state, rules, evaluation)
	if analysis.Technique != "batna_preparation" || analysis.ArgumentDelta != 1 || len(analysis.Risks) != 0 {
		t.Fatalf("unexpected BATNA analysis: %+v", analysis)
	}
}
