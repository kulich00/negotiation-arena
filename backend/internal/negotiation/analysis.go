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
	case IntentAskSituation:
		analysis.Technique = "spin_situation"
		analysis.Summary = "Вы уточнили исходную ситуацию и контекст второй стороны."
		analysis.Strengths = append(analysis.Strengths, "Собран контекст для дальнейшего исследования проблемы")
		analysis.Recommendation = "Перейдите к вопросу о конкретной проблеме или препятствии."
	case IntentIdentifyProblem:
		analysis.Technique = "spin_problem"
		analysis.Summary = "Вы помогли второй стороне сформулировать проблему."
		if before.SPINStage < domain.SPINStageSituation {
			analysis.Risks = append(analysis.Risks, "Проблема обсуждается без уточнения исходной ситуации")
			analysis.Recommendation = "Уточните контекст и ограничения, чтобы проверить понимание проблемы."
		} else {
			analysis.Strengths = append(analysis.Strengths, "Проблема связана с ранее уточнённым контекстом")
			analysis.Recommendation = "Исследуйте последствия проблемы для второй стороны."
		}
	case IntentExploreImplication:
		analysis.Technique = "spin_implication"
		analysis.Summary = "Вы исследовали последствия нерешённой проблемы."
		if before.SPINStage < domain.SPINStageProblem {
			analysis.Risks = append(analysis.Risks, "Последствия обсуждаются до формулирования проблемы")
			analysis.Recommendation = "Сначала согласуйте, какую именно проблему необходимо решить."
		} else {
			analysis.Strengths = append(analysis.Strengths, "Показана значимость решения проблемы")
			analysis.Recommendation = "Уточните, какую ценность даст желаемое решение."
		}
	case IntentClarifyNeedPayoff:
		analysis.Technique = "spin_need_payoff"
		analysis.Summary = "Вы помогли сформулировать ценность и желаемый результат решения."
		if before.SPINStage < domain.SPINStageImplication {
			analysis.Risks = append(analysis.Risks, "Ценность решения обсуждается без анализа последствий проблемы")
			analysis.Recommendation = "Покажите последствия текущей проблемы, прежде чем переходить к выгоде решения."
		} else {
			analysis.Strengths = append(analysis.Strengths, "Ценность решения сформулирована второй стороной")
			analysis.Recommendation = "Свяжите выявленную ценность с конкретным предложением."
		}
	case IntentStateBATNA:
		analysis.Technique = "batna_preparation"
		analysis.Summary = "Вы обозначили альтернативу на случай отсутствия соглашения."
		analysis.Strengths = append(analysis.Strengths, "Определена граница приемлемого соглашения")
		if !before.InterestsExplored {
			analysis.Risks = append(analysis.Risks, "BATNA обозначена до исследования интересов второй стороны")
			analysis.Recommendation = "Сначала уточните интересы собеседника, затем сравнивайте варианты с BATNA."
		} else {
			analysis.Recommendation = "Сравните предложение и BATNA по одинаковым объективным критериям."
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
