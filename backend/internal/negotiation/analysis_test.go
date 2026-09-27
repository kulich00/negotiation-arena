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
