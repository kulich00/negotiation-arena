package negotiation

import "github.com/kulich00/negotiation-arena/backend/internal/domain"

// GenerateOpponentReply returns a deterministic response for a validated
// structured move. A future LLM provider may rephrase this response, while the
// decision itself remains controlled by the negotiation rules.
func GenerateOpponentReply(move PlayerMove, session domain.Session, rules domain.ScenarioRules, evaluation MoveEvaluation) string {
	switch move.Intent {
	case IntentAskInterest:
		return "Для меня важно снизить риски и понять взаимную выгоду. Какие варианты вы предлагаете?"
	case IntentPresentEvidence:
		if rules.RequiresInterestExploration && !evaluation.State.InterestsExplored {
			return "Факты полезны. Теперь уточните, какие интересы второй стороны учитывает ваше предложение."
		}
		return "Аргументы стали конкретнее. Покажите, как они связаны с предлагаемыми условиями."
	case IntentPropose:
		return proposalReply(session, rules, evaluation)
	case IntentAccept:
		return "Договорились. Зафиксируем согласованные условия."
	case IntentPressure:
		return "Давление не помогает договориться. Вернёмся к фактам и интересам сторон."
	default:
		return "Уточните вашу позицию."
	}
}

func proposalReply(session domain.Session, rules domain.ScenarioRules, evaluation MoveEvaluation) string {
	if rules.RequiresInterestExploration && !evaluation.State.InterestsExplored {
		return "Прежде чем обсуждать конкретные условия, выясните, что важно второй стороне."
	}
	if rules.RequiresEvidence && !evaluation.State.EvidencePresented {
		return "Для решения по предложению нужны факты и измеримые аргументы."
	}
	projectedTrust := clamp(session.TrustScore + evaluation.TrustDelta)
	if projectedTrust < rules.MinimumTrustForAgreement {
		return "Пока я не готов принять эти условия. Сначала нужно укрепить доверие и снизить риски."
	}
	return "Условия выглядят приемлемо. Если вы их подтверждаете, можем зафиксировать договорённость."
}
