package negotiation

import (
	"fmt"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
)

// GenerateOpponentReply returns a deterministic response for a validated
// structured move. A future LLM provider may rephrase this response, while the
// decision itself remains controlled by the negotiation rules.
func GenerateOpponentReply(move PlayerMove, session domain.Session, rules domain.ScenarioRules, evaluation MoveEvaluation) string {
	reply := standardOpponentReply(move, session, rules, evaluation)
	return difficultOpponentReply(reply, evaluation.OpponentReaction)
}

func standardOpponentReply(move PlayerMove, session domain.Session, rules domain.ScenarioRules, evaluation MoveEvaluation) string {
	switch move.Intent {
	case IntentNeutral:
		return "Уточните вашу позицию: задайте вопрос, приведите факт или предложите конкретные условия."
	case IntentAskInterest:
		return "Для меня важно снизить риски и понять взаимную выгоду. Какие варианты вы предлагаете?"
	case IntentPresentEvidence:
		if rules.RequiresInterestExploration && !evaluation.State.InterestsExplored {
			return "Факты полезны. Теперь уточните, какие интересы второй стороны учитывает ваше предложение."
		}
		return "Аргументы стали конкретнее. Покажите, как они связаны с предлагаемыми условиями."
	case IntentAskSituation:
		return "Сейчас для меня важны сроки, доступные ресурсы и предсказуемость результата."
	case IntentIdentifyProblem:
		if session.State.SPINStage < domain.SPINStageSituation {
			return "Сначала уточните исходные условия, чтобы мы одинаково понимали проблему."
		}
		return "Да, это основное препятствие. Давайте разберём, к чему оно приводит."
	case IntentExploreImplication:
		if session.State.SPINStage < domain.SPINStageProblem {
			return "Пока неясно, какую именно проблему вы анализируете. Сформулируйте её точнее."
		}
		return "Если ничего не менять, риски и издержки действительно возрастут. Какой результат вы предлагаете?"
	case IntentClarifyNeedPayoff:
		if session.State.SPINStage < domain.SPINStageImplication {
			return "Поясните последствия текущей ситуации, тогда ценность решения будет понятнее."
		}
		return "Такой результат был бы полезен. Теперь предложите конкретные условия его достижения."
	case IntentStateBATNA:
		if !session.State.InterestsExplored {
			return "Я услышал вашу альтернативу, но сначала важно понять интересы обеих сторон."
		}
		return "Альтернатива понятна. Сравним её с возможным соглашением по объективным критериям."
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

func difficultOpponentReply(reply string, reaction *domain.OpponentReaction) string {
	if reaction == nil {
		return reply
	}
	switch {
	case reaction.Mood == OpponentMoodIrritated && reaction.Priority != "":
		return fmt.Sprintf("Меня раздражает такой тон. Мой текущий приоритет — «%s». %s", reaction.Priority, reply)
	case reaction.Mood == OpponentMoodIrritated:
		return "Меня раздражает такой тон. " + reply
	case reaction.PriorityChanged:
		return fmt.Sprintf("Ситуация изменилась: теперь для меня важнее «%s». %s", reaction.Priority, reply)
	case reaction.Mood == OpponentMoodGuarded && reaction.Priority != "":
		return fmt.Sprintf("Я пока настроен скептически. Мой текущий приоритет — «%s». %s", reaction.Priority, reply)
	case reaction.Mood == OpponentMoodGuarded:
		return "Я пока настроен скептически. " + reply
	case reaction.Mood == OpponentMoodReceptive:
		return "Такой подход помогает снизить напряжение. " + reply
	default:
		return reply
	}
}

func proposalReply(session domain.Session, rules domain.ScenarioRules, evaluation MoveEvaluation) string {
	if rules.RequiresInterestExploration && !evaluation.State.InterestsExplored {
		return "Прежде чем обсуждать конкретные условия, выясните, что важно второй стороне."
	}
	if rules.RequiresEvidence && !evaluation.State.EvidencePresented {
		return "Для решения по предложению нужны факты и измеримые аргументы."
	}
	if evaluation.State.LastOfferQuality == domain.OfferQualityRejected {
		return rejectedProposalReply(rules.Proposal)
	}
	projectedTrust := clamp(session.TrustScore + evaluation.TrustDelta)
	if projectedTrust < rules.MinimumTrustForAgreement {
		return "Пока я не готов принять эти условия. Сначала нужно укрепить доверие и снизить риски."
	}
	if evaluation.State.LastOfferQuality == domain.OfferQualityPreferred {
		return "Предложение учитывает мои приоритеты. Готов зафиксировать эти условия."
	}
	return "Условия выглядят приемлемо. Если вы их подтверждаете, можем зафиксировать договорённость."
}

func rejectedProposalReply(constraint domain.ProposalConstraint) string {
	if constraint.Kind != "none" {
		return fmt.Sprintf("Это выходит за мой предел уступки. Готов обсуждать значение не более %d.", constraint.MaximumValue)
	}
	return "Этот вариант для меня неприемлем. Нужна другая конфигурация условий."
}
