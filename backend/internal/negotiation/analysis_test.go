package negotiation

import (
	"testing"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
	"github.com/kulich00/negotiation-arena/backend/internal/llm"
)

func TestAnalyzeStructuredMoveExplainsMissingPreparation(t *testing.T) {
	rules := domain.DefaultScenarioRules()
	move := PlayerMove{Content: "Предлагаю вариант", Intent: IntentPropose}
	evaluation := EvaluateMove(move, domain.InitialSessionState())

	analysis := AnalyzeStructuredMove(move, domain.InitialSessionState(), rules, evaluation)
	if analysis.Technique != "harvard_mutual_gain" || analysis.TrustDelta != 1 {
		t.Fatalf("unexpected analysis: %+v", analysis)
	}
	if len(analysis.Risks) != 2 || analysis.Recommendation == "" {
		t.Fatalf("missing proposal risks: %+v", analysis)
	}
}

func TestAnalyzeLegacyMoveRecognizesPressure(t *testing.T) {
	analysis := AnalyzeLegacyMove(llm.AnalysisResult{TrustDelta: -1, PressureDelta: 2})
	if analysis.Technique != "competitive_pressure" || len(analysis.Risks) != 1 || analysis.PressureDelta != 2 {
		t.Fatalf("unexpected legacy analysis: %+v", analysis)
	}
}

func TestAnalyzeStructuredMoveExplainsSPINOrder(t *testing.T) {
	rules := domain.DefaultScenarioRules()
	move := PlayerMove{Content: "К чему приведёт проблема?", Intent: IntentExploreImplication}
	evaluation := EvaluateMove(move, domain.InitialSessionState())

	analysis := AnalyzeStructuredMove(move, domain.InitialSessionState(), rules, evaluation)
	if analysis.Technique != "spin_implication" || analysis.ArgumentDelta != 1 {
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
	evaluation := EvaluateMove(move, state)

	analysis := AnalyzeStructuredMove(move, state, rules, evaluation)
	if analysis.Technique != "batna_preparation" || analysis.ArgumentDelta != 1 || len(analysis.Risks) != 0 {
		t.Fatalf("unexpected BATNA analysis: %+v", analysis)
	}
}
