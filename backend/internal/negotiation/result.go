package negotiation

import (
	"math"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
)

func EvaluateResult(session domain.Session, rules domain.ScenarioRules) domain.Result {
	rawScore :=
		float64(session.TrustScore)*rules.TrustScoreWeight +
			float64(session.ArgumentScore*5)*rules.ArgumentScoreWeight -
			float64(session.PressureScore*4)*rules.PressureScoreWeight
	score := clamp(int(math.Round(rawScore)))

	result := domain.Result{
		SessionID:       session.ID,
		FinalScore:      score,
		Outcome:         "Переговоры требуют доработки",
		Strengths:       []string{},
		Mistakes:        []string{},
		Recommendations: []string{},
	}

	if session.State.StructuredMovesUsed {
		evaluateStructuredResult(&result, session, rules)
	} else {
		evaluateLegacyResult(&result, session)
	}

	if session.PressureScore > 2 {
		result.Mistakes = append(result.Mistakes, "Избыточное давление")
	}
	return result
}

func evaluateStructuredResult(result *domain.Result, session domain.Session, rules domain.ScenarioRules) {
	agreement := session.State.OfferMade &&
		session.State.OfferAccepted &&
		session.TrustScore >= rules.MinimumTrustForAgreement &&
		result.FinalScore >= 50

	if session.State.InterestsExplored {
		result.Strengths = append(result.Strengths, "Выявлены интересы второй стороны")
	} else if rules.RequiresInterestExploration {
		agreement = false
		result.Recommendations = append(result.Recommendations, "Перед предложением выясните интересы второй стороны")
	}

	if session.State.EvidencePresented {
		result.Strengths = append(result.Strengths, "Представлены подтверждающие аргументы")
	} else if rules.RequiresEvidence {
		agreement = false
		result.Recommendations = append(result.Recommendations, "Подкрепите позицию измеримыми фактами")
	}

	evaluateSPINAndBATNA(result, session.State)

	if !session.State.OfferMade {
		result.Recommendations = append(result.Recommendations, "Сформулируйте конкретное предложение")
	} else if !session.State.OfferAccepted {
		result.Recommendations = append(result.Recommendations, "Убедитесь, что итоговое предложение явно принято")
	} else {
		result.Strengths = append(result.Strengths, "Переговоры завершены принятием предложения")
	}

	if session.TrustScore < rules.MinimumTrustForAgreement {
		result.Recommendations = append(result.Recommendations, "Сначала укрепите доверие второй стороны")
	}

	switch {
	case agreement && result.FinalScore >= 70:
		result.Outcome = "Выгодное соглашение"
	case agreement:
		result.Outcome = "Компромисс"
	default:
		result.Outcome = "Соглашение не достигнуто"
	}
}

func evaluateSPINAndBATNA(result *domain.Result, state domain.SessionState) {
	if state.SPINStage == domain.SPINStageNeedPayoff {
		result.Strengths = append(result.Strengths, "Последовательно пройдены все этапы SPIN")
	} else if state.SituationExplored || state.ProblemIdentified || state.ImplicationsExplored || state.NeedPayoffEstablished {
		switch state.SPINStage {
		case domain.SPINStageNone:
			result.Recommendations = append(result.Recommendations, "Начните SPIN-анализ с уточнения ситуации")
		case domain.SPINStageSituation:
			result.Recommendations = append(result.Recommendations, "Продолжите SPIN-анализ формулированием проблемы")
		case domain.SPINStageProblem:
			result.Recommendations = append(result.Recommendations, "Исследуйте последствия выявленной проблемы")
		case domain.SPINStageImplication:
			result.Recommendations = append(result.Recommendations, "Уточните ценность решения для второй стороны")
		}
	}

	if state.BATNADefined {
		result.Strengths = append(result.Strengths, "Определена альтернатива на случай отсутствия соглашения")
	} else {
		result.Recommendations = append(result.Recommendations, "Заранее определите BATNA и границу приемлемого соглашения")
	}
}

func evaluateLegacyResult(result *domain.Result, session domain.Session) {
	if session.ArgumentScore > 2 {
		result.Strengths = append(result.Strengths, "Аргументы опирались на факты")
	} else {
		result.Recommendations = append(result.Recommendations, "Добавьте измеримые результаты и факты")
	}
	if session.TrustScore >= 53 {
		result.Strengths = append(result.Strengths, "Удалось сохранить доверие")
	} else {
		result.Recommendations = append(result.Recommendations, "Задавайте больше открытых вопросов")
	}

	if result.FinalScore >= 70 {
		result.Outcome = "Выгодное соглашение"
	} else if result.FinalScore >= 50 {
		result.Outcome = "Компромисс"
	}
}
