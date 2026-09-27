package negotiation

import (
	"github.com/kulich00/negotiation-arena/backend/internal/domain"
	"github.com/kulich00/negotiation-arena/backend/internal/llm"
)

func AnalyzeStructuredMove(move PlayerMove, before domain.SessionState, rules domain.ScenarioRules, evaluation MoveEvaluation) domain.TurnAnalysis {
	analysis := domain.TurnAnalysis{
		Intent:         string(move.Intent),
		TrustDelta:     evaluation.TrustDelta,
		ArgumentDelta:  evaluation.ArgumentDelta,
		PressureDelta:  evaluation.PressureDelta,
		Strengths:      []string{},
		Risks:          []string{},
		Recommendation: "Продолжайте связывать позицию с интересами обеих сторон.",
	}

	switch move.Intent {
	case IntentAskInterest:
		analysis.Technique = "harvard_interests"
		analysis.Summary = "Вы исследовали интересы второй стороны и укрепили доверие."
		analysis.Strengths = append(analysis.Strengths, "Открытый вопрос помогает перейти от позиций к интересам")
		if rules.RequiresEvidence && !before.EvidencePresented {
			analysis.Recommendation = "После уточнения интересов подкрепите позицию измеримыми фактами."
		}
	case IntentPresentEvidence:
		analysis.Technique = "evidence_based_argument"
		analysis.Summary = "Вы усилили позицию фактами и конкретными аргументами."
		analysis.Strengths = append(analysis.Strengths, "Аргументация стала проверяемой и конкретной")
		if rules.RequiresInterestExploration && !before.InterestsExplored {
			analysis.Risks = append(analysis.Risks, "Факты представлены до выяснения интересов второй стороны")
			analysis.Recommendation = "Уточните интересы собеседника и свяжите факты с его задачами."
		} else {
			analysis.Recommendation = "Свяжите доказательства с конкретным взаимовыгодным предложением."
		}
	case IntentPropose:
		analysis.Technique = "harvard_mutual_gain"
		analysis.Summary = "Вы перевели обсуждение к конкретному варианту соглашения."
		analysis.Strengths = append(analysis.Strengths, "Сформулировано конкретное предложение")
		if rules.RequiresInterestExploration && !before.InterestsExplored {
			analysis.Risks = append(analysis.Risks, "Предложение сделано до исследования интересов")
		}
		if rules.RequiresEvidence && !before.EvidencePresented {
			analysis.Risks = append(analysis.Risks, "Предложению не хватает подтверждающих фактов")
		}
		if len(analysis.Risks) > 0 {
			analysis.Recommendation = "Закройте отмеченные условия перед фиксацией договорённости."
		} else {
			analysis.Recommendation = "Проверьте готовность второй стороны принять предложенные условия."
		}
	case IntentAccept:
		analysis.Technique = "agreement_confirmation"
		analysis.Summary = "Вы подтвердили принятие сформулированного предложения."
		analysis.Strengths = append(analysis.Strengths, "Договорённость явно зафиксирована")
		analysis.Recommendation = "Зафиксируйте сроки, ответственных и критерии выполнения соглашения."
	case IntentPressure:
		analysis.Technique = "competitive_pressure"
		analysis.Summary = "Давление повысило напряжение и снизило доверие."
		analysis.Risks = append(analysis.Risks, "Жёсткая формулировка может вызвать сопротивление")
		analysis.Recommendation = "Замените требование открытым вопросом, фактами или взаимовыгодным вариантом."
	}

	return analysis
}

func AnalyzeLegacyMove(result llm.AnalysisResult) domain.TurnAnalysis {
	analysis := domain.TurnAnalysis{
		Intent:         "legacy",
		Technique:      "neutral_statement",
		TrustDelta:     result.TrustDelta,
		ArgumentDelta:  result.ArgumentDelta,
		PressureDelta:  result.PressureDelta,
		Summary:        "Реплика не изменила ключевые показатели переговоров.",
		Strengths:      append([]string{}, result.DetectedStrengths...),
		Risks:          []string{},
		Recommendation: "Добавьте открытый вопрос, проверяемый факт или конкретное предложение.",
	}

	switch {
	case result.PressureDelta > 0:
		analysis.Technique = "competitive_pressure"
		analysis.Summary = "В реплике обнаружено давление, которое повышает напряжение."
		analysis.Risks = append(analysis.Risks, "Давление может снизить доверие и затруднить соглашение")
		analysis.Recommendation = "Переформулируйте требование через интересы сторон и объективные критерии."
	case result.ArgumentDelta > 0:
		analysis.Technique = "evidence_based_argument"
		analysis.Summary = "Реплика содержит конкретные аргументы или измеримые факты."
		analysis.Recommendation = "Покажите, какую выгоду эти факты дают второй стороне."
	case result.TrustDelta > 0:
		analysis.Technique = "relationship_building"
		analysis.Summary = "Формулировка поддерживает диалог и укрепляет доверие."
		analysis.Recommendation = "Продолжите открытым вопросом и уточните приоритеты собеседника."
	}

	return analysis
}
