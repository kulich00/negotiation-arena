package llm

import (
	"context"
	"regexp"
	"strconv"
	"strings"
)

// RuleBasedMoveInterpreter keeps semantic scoring available when Gemini is
// disabled or temporarily unavailable. It intentionally recognizes only
// explicit negotiation actions; an arbitrary question must not earn points.
type RuleBasedMoveInterpreter struct{}

var integerPattern = regexp.MustCompile(`\d+`)

func (RuleBasedMoveInterpreter) InterpretMove(_ context.Context, request InterpretationRequest) (MoveInterpretation, error) {
	message := strings.ReplaceAll(strings.ToLower(strings.TrimSpace(request.Message)), "ё", "е")
	result := MoveInterpretation{Intent: "neutral", Relevant: boolPointer(false), Source: "local"}
	if message == "" {
		return result, nil
	}

	switch {
	case containsAnyPhrase(message, "иначе", "обязаны", "должны", "требую", "пожалеете", "последний шанс", "нет выбора"):
		result.Intent = "pressure"
	case request.OfferMade && containsAnyPhrase(message, "согласен", "согласна", "принимаю", "договорились", "подтверждаю", "меня устраивает", "условия подходят", "готов принять", "готова принять", "фиксируем"):
		result.Intent = "accept"
	case containsAnyPhrase(message, "если не договоримся", "моя альтернатива", "альтернативный вариант", "другой поставщик", "другое предложение"):
		result.Intent = "state_batna"
	case containsAnyPhrase(message, "к чему привед", "какие последствия", "что произойдет", "что произойдёт", "что случится", "чем грозит"):
		result.Intent = "explore_implication"
	case containsAnyPhrase(message, "что даст", "какую пользу", "какая выгода", "какую ценность", "что изменится после"):
		result.Intent = "clarify_need_payoff"
	case containsAnyPhrase(message, "что мешает", "в чем проблема", "какая проблема", "какие препятствия", "какая сложность"):
		result.Intent = "identify_problem"
	case containsAnyPhrase(message, "как сейчас", "как устроен", "как происходит", "какой процесс", "какие ресурсы", "какой бюджет"):
		result.Intent = "ask_situation"
	case asksAboutInterests(message):
		result.Intent = "ask_interest"
	case containsAnyPhrase(message, "предлагаю", "готовы ли вы", "давайте зафиксируем", "если мы пойдем навстречу", "если мы пойдём навстречу"):
		result.Intent = "propose"
		populateProposal(&result, request, message)
	case containsEvidence(message):
		result.Intent = "present_evidence"
	default:
		return result, nil
	}

	result.Relevant = boolPointer(true)
	return result, nil
}

func containsEvidence(message string) bool {
	if strings.Contains(message, "%") || containsAnyPhrase(message, "по данным", "исследование показало", "результат вырос", "результат снизился", "метрика", "статистика", "факты показывают") {
		return true
	}
	if !integerPattern.MatchString(message) {
		return false
	}
	return containsAnyPhrase(message,
		"процент", "день", "недел", "месяц", "год", "руб", "доллар", "евро",
		"рост", "вырос", "сниз", "сократ", "увелич", "конверси", "выручк",
		"затрат", "эконом", "срок", "результат", "тест", "замер", "данн",
	)
}

func asksAboutInterests(message string) bool {
	if !containsAnyPhrase(message, "что важно", "для вас важно", "приоритет", "критич", "критери", "интерес", "цель") {
		return false
	}
	return strings.Contains(message, "?") || containsAnyPhrase(message,
		"расскажите", "уточните", "объясните", "хочу понять", "хотел бы понять", "хотела бы понять", "давайте выясним",
	)
}

func populateProposal(result *MoveInterpretation, request InterpretationRequest, message string) {
	for _, alternative := range request.ProposalAlternatives {
		if strings.Contains(message, strings.ToLower(alternative)) || strings.Contains(message, strings.ReplaceAll(strings.ToLower(alternative), "_", " ")) {
			result.AlternativeID = alternative
			return
		}
	}
	if request.ProposalKind == "none" {
		return
	}
	value := integerPattern.FindString(message)
	if value == "" {
		return
	}
	parsed, err := strconv.Atoi(value)
	if err == nil && parsed > 0 && (request.ProposalMaximum == 0 || parsed <= request.ProposalMaximum) {
		result.ProposalValue = parsed
	}
}

func containsAnyPhrase(value string, variants ...string) bool {
	for _, variant := range variants {
		if strings.Contains(value, variant) {
			return true
		}
	}
	return false
}

func boolPointer(value bool) *bool {
	return &value
}
