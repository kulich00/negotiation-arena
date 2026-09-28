package negotiation

import (
	"reflect"
	"testing"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
)

func TestAggregateSessionAnalysis(t *testing.T) {
	messages := []domain.Message{
		{Sender: "opponent", Content: "Начнём"},
		playerMessage(domain.TurnAnalysis{Technique: "harvard_interests", TrustDelta: 2, Summary: "Уточнены интересы", Risks: []string{"Поспешный вывод"}, Recommendation: "Уточните критерии."}),
		{Sender: "opponent", Content: "Ответ"},
		playerMessage(domain.TurnAnalysis{Technique: "evidence_based_argument", ArgumentDelta: 2, Summary: "Приведены факты", Risks: []string{"Поспешный вывод"}, Recommendation: "Свяжите факты с выгодой."}),
		playerMessage(domain.TurnAnalysis{Technique: "harvard_interests", TrustDelta: 1, Summary: "Повторно уточнены интересы", Recommendation: "Уточните критерии."}),
		playerMessage(domain.TurnAnalysis{Technique: "competitive_pressure", TrustDelta: -1, PressureDelta: 2, Summary: "Использовано давление", Recommendation: "Снизьте давление."}),
	}

	analysis := AggregateSessionAnalysis(messages)

	if analysis.AnalyzedTurns != 4 {
		t.Fatalf("unexpected analyzed turn count: %d", analysis.AnalyzedTurns)
	}
	wantTechniques := []domain.TechniqueUsage{
		{Technique: "harvard_interests", Label: "Исследование интересов", Count: 2},
		{Technique: "competitive_pressure", Label: "Конкурентное давление", Count: 1},
		{Technique: "evidence_based_argument", Label: "Аргументация фактами", Count: 1},
	}
	if !reflect.DeepEqual(analysis.Techniques, wantTechniques) {
		t.Fatalf("unexpected technique statistics: %+v", analysis.Techniques)
	}
	if len(analysis.RepeatedRisks) != 1 || analysis.RepeatedRisks[0].Risk != "Поспешный вывод" || analysis.RepeatedRisks[0].Count != 2 {
		t.Fatalf("unexpected repeated risks: %+v", analysis.RepeatedRisks)
	}
	if len(analysis.PriorityRecommendations) != 3 || analysis.PriorityRecommendations[0] != "Уточните критерии." {
		t.Fatalf("unexpected priority recommendations: %+v", analysis.PriorityRecommendations)
	}
	if analysis.BestMove == nil || analysis.BestMove.Turn != 2 || analysis.BestMove.Technique != "evidence_based_argument" || analysis.BestMove.ScoreImpact != 4 {
		t.Fatalf("unexpected best move: %+v", analysis.BestMove)
	}
}

func TestEnrichResultWithSessionAnalysis(t *testing.T) {
	result := domain.Result{
		Strengths:       []string{},
		Mistakes:        []string{},
		Recommendations: []string{"Уточните критерии."},
		Analysis: domain.SessionAnalysis{
			Techniques: []domain.TechniqueUsage{
				{Technique: "competitive_pressure", Label: "Конкурентное давление", Count: 3},
				{Technique: "harvard_interests", Label: "Исследование интересов", Count: 2},
			},
			RepeatedRisks: []domain.RepeatedRisk{{Risk: "Поспешный вывод", Count: 2}},
			PriorityRecommendations: []string{
				"Уточните критерии.",
				"Снизьте давление.",
			},
		},
	}

	EnrichResultWithSessionAnalysis(&result)

	if !reflect.DeepEqual(result.Strengths, []string{"Наиболее часто применялась техника «Исследование интересов» (2)"}) {
		t.Fatalf("unexpected strengths: %+v", result.Strengths)
	}
	if !reflect.DeepEqual(result.Mistakes, []string{"Повторяющийся риск (2): Поспешный вывод"}) {
		t.Fatalf("unexpected mistakes: %+v", result.Mistakes)
	}
	if !reflect.DeepEqual(result.Recommendations, []string{"Уточните критерии.", "Снизьте давление."}) {
		t.Fatalf("unexpected recommendations: %+v", result.Recommendations)
	}
}

func TestAggregateSessionAnalysisUsesFirstMoveToBreakBestMoveTie(t *testing.T) {
	messages := []domain.Message{
		playerMessage(domain.TurnAnalysis{Technique: "harvard_interests", TrustDelta: 2, Summary: "Первый"}),
		playerMessage(domain.TurnAnalysis{Technique: "relationship_building", TrustDelta: 2, Summary: "Второй"}),
	}

	analysis := AggregateSessionAnalysis(messages)
	if analysis.BestMove == nil || analysis.BestMove.Turn != 1 || analysis.BestMove.Summary != "Первый" {
		t.Fatalf("unexpected best move for equal impact: %+v", analysis.BestMove)
	}
}

func TestAggregateSessionAnalysisGroupsStableErrorClasses(t *testing.T) {
	messages := []domain.Message{
		playerMessage(domain.TurnAnalysis{Errors: []domain.NegotiationError{
			{Code: ErrorPressureTactic, Label: "Избыточное давление", Severity: domain.ErrorSeverityHigh, Message: "Давление"},
		}}),
		playerMessage(domain.TurnAnalysis{Errors: []domain.NegotiationError{
			{Code: ErrorProposalOutsideLimit, Label: "Неприемлемые условия предложения", Severity: domain.ErrorSeverityCritical, Message: "За пределом"},
			{Code: ErrorPressureTactic, Label: "Избыточное давление", Severity: domain.ErrorSeverityHigh, Message: "Давление"},
		}}),
	}

	analysis := AggregateSessionAnalysis(messages)
	if len(analysis.ErrorClasses) != 2 {
		t.Fatalf("unexpected error classes: %+v", analysis.ErrorClasses)
	}
	if analysis.ErrorClasses[0].Code != ErrorProposalOutsideLimit || analysis.ErrorClasses[0].Severity != domain.ErrorSeverityCritical || analysis.ErrorClasses[0].Count != 1 {
		t.Fatalf("critical error must be first: %+v", analysis.ErrorClasses)
	}
	pressure := analysis.ErrorClasses[1]
	if pressure.Code != ErrorPressureTactic || pressure.Count != 2 || !reflect.DeepEqual(pressure.Turns, []int{1, 2}) {
		t.Fatalf("unexpected pressure aggregation: %+v", pressure)
	}
}

func playerMessage(analysis domain.TurnAnalysis) domain.Message {
	return domain.Message{Sender: "player", Analysis: &analysis}
}
