package negotiation

import (
	"fmt"
	"sort"
	"strings"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
)

const maxPriorityRecommendations = 3

var techniqueLabels = map[string]string{
	"harvard_interests":       "Исследование интересов",
	"evidence_based_argument": "Аргументация фактами",
	"harvard_mutual_gain":     "Поиск взаимной выгоды",
	"agreement_confirmation":  "Фиксация договорённости",
	"competitive_pressure":    "Конкурентное давление",
	"relationship_building":   "Укрепление отношений",
	"neutral_statement":       "Нейтральная позиция",
	"spin_situation":          "SPIN: ситуация",
	"spin_problem":            "SPIN: проблема",
	"spin_implication":        "SPIN: последствия",
	"spin_need_payoff":        "SPIN: ценность решения",
	"batna_preparation":       "Подготовка BATNA",
}

func AggregateSessionAnalysis(messages []domain.Message) domain.SessionAnalysis {
	result := domain.SessionAnalysis{
		Techniques:              []domain.TechniqueUsage{},
		RepeatedRisks:           []domain.RepeatedRisk{},
		PriorityRecommendations: []string{},
	}
	techniqueCounts := make(map[string]int)
	riskCounts := make(map[string]int)
	recommendationCounts := make(map[string]int)

	for _, message := range messages {
		if message.Sender != "player" || message.Analysis == nil {
			continue
		}
		analysis := message.Analysis
		result.AnalyzedTurns++
		turn := result.AnalyzedTurns

		if analysis.Technique != "" {
			techniqueCounts[analysis.Technique]++
		}
		for _, risk := range analysis.Risks {
			if risk = strings.TrimSpace(risk); risk != "" {
				riskCounts[risk]++
			}
		}
		if recommendation := strings.TrimSpace(analysis.Recommendation); recommendation != "" {
			recommendationCounts[recommendation]++
		}

		impact := analysis.TrustDelta + 2*analysis.ArgumentDelta - 2*analysis.PressureDelta
		if result.BestMove == nil || impact > result.BestMove.ScoreImpact {
			result.BestMove = &domain.BestMoveInsight{
				Turn:        turn,
				Technique:   analysis.Technique,
				Label:       techniqueLabel(analysis.Technique),
				Summary:     analysis.Summary,
				ScoreImpact: impact,
			}
		}
	}

	for technique, count := range techniqueCounts {
		result.Techniques = append(result.Techniques, domain.TechniqueUsage{
			Technique: technique,
			Label:     techniqueLabel(technique),
			Count:     count,
		})
	}
	sort.Slice(result.Techniques, func(i, j int) bool {
		if result.Techniques[i].Count != result.Techniques[j].Count {
			return result.Techniques[i].Count > result.Techniques[j].Count
		}
		return result.Techniques[i].Technique < result.Techniques[j].Technique
	})

	for risk, count := range riskCounts {
		if count >= 2 {
			result.RepeatedRisks = append(result.RepeatedRisks, domain.RepeatedRisk{Risk: risk, Count: count})
		}
	}
	sort.Slice(result.RepeatedRisks, func(i, j int) bool {
		if result.RepeatedRisks[i].Count != result.RepeatedRisks[j].Count {
			return result.RepeatedRisks[i].Count > result.RepeatedRisks[j].Count
		}
		return result.RepeatedRisks[i].Risk < result.RepeatedRisks[j].Risk
	})

	type countedText struct {
		text  string
		count int
	}
	recommendations := make([]countedText, 0, len(recommendationCounts))
	for recommendation, count := range recommendationCounts {
		recommendations = append(recommendations, countedText{text: recommendation, count: count})
	}
	sort.Slice(recommendations, func(i, j int) bool {
		if recommendations[i].count != recommendations[j].count {
			return recommendations[i].count > recommendations[j].count
		}
		return recommendations[i].text < recommendations[j].text
	})
	for index, recommendation := range recommendations {
		if index == maxPriorityRecommendations {
			break
		}
		result.PriorityRecommendations = append(result.PriorityRecommendations, recommendation.text)
	}

	return result
}

func EnrichResultWithSessionAnalysis(result *domain.Result) {
	if result == nil {
		return
	}
	for _, technique := range result.Analysis.Techniques {
		if technique.Technique == "competitive_pressure" || technique.Technique == "neutral_statement" {
			continue
		}
		result.Strengths = appendUnique(result.Strengths, fmt.Sprintf("Наиболее часто применялась техника «%s» (%d)", technique.Label, technique.Count))
		break
	}
	for _, risk := range result.Analysis.RepeatedRisks {
		result.Mistakes = appendUnique(result.Mistakes, fmt.Sprintf("Повторяющийся риск (%d): %s", risk.Count, risk.Risk))
	}
	for _, recommendation := range result.Analysis.PriorityRecommendations {
		result.Recommendations = appendUnique(result.Recommendations, recommendation)
	}
}

func techniqueLabel(technique string) string {
	if label, ok := techniqueLabels[technique]; ok {
		return label
	}
	if technique == "" {
		return "Не определена"
	}
	return technique
}

func appendUnique(items []string, value string) []string {
	for _, item := range items {
		if item == value {
			return items
		}
	}
	return append(items, value)
}
