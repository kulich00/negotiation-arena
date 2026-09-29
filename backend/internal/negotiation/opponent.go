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
	if evaluation.Repeated {
		return repeatedMoveReply(move.Intent, session.Turn)
	}
	reply := func(variants ...string) string {
		return replyForTurn(session.Turn, variants...)
	}

	switch move.Intent {
	case IntentNeutral:
		return neutralReply(session.Turn)
	case IntentAskInterest:
		return reply(
			"Для меня важно снизить риски и понять взаимную выгоду. Какие варианты вы предлагаете?",
			"В первую очередь мне нужны предсказуемость и понятная польза для обеих сторон. Как вы это учтёте?",
			"Мой главный интерес — контролируемый риск при ощутимом результате. Что может это обеспечить?",
			"Я готов обсуждать условия, если увижу баланс выгоды и рисков. Какой подход вы предлагаете?",
		)
	case IntentPresentEvidence:
		if rules.RequiresInterestExploration && !evaluation.State.InterestsExplored {
			return reply(
				"Факты полезны. Теперь уточните, какие интересы второй стороны учитывает ваше предложение.",
				"Данные принял, но пока неясно, как они связаны с моими приоритетами. Сначала уточните их.",
				"Цифры добавляют конкретики. Объясните, какую мою задачу они помогают решить.",
				"Аргумент понятен, однако без связи с интересами стороны он недостаточен. Покажите эту связь.",
			)
		}
		return reply(
			"Аргументы стали конкретнее. Покажите, как они связаны с предлагаемыми условиями.",
			"Эти данные можно учитывать. Какой вывод из них следует для условий сделки?",
			"Факт усиливает вашу позицию. Переведите его в конкретную пользу и параметры соглашения.",
			"Доказательство принято. Теперь свяжите результат с предложением, которое готовы зафиксировать.",
		)
	case IntentAskSituation:
		return reply(
			"Сейчас для меня важны сроки, доступные ресурсы и предсказуемость результата.",
			"Текущие ограничения — время, загрузка команды и риск непредвиденных расходов.",
			"Решение зависит от сроков, имеющихся ресурсов и того, насколько управляемым будет процесс.",
			"На старте нужно учитывать ограниченный ресурс и требование получить прогнозируемый результат.",
		)
	case IntentIdentifyProblem:
		if session.State.SPINStage < domain.SPINStageSituation {
			return reply(
				"Сначала уточните исходные условия, чтобы мы одинаково понимали проблему.",
				"Проблему рано оценивать без контекста. Сначала выясните, как сейчас устроен процесс.",
				"Начните с текущей ситуации и ограничений — тогда сможем точно назвать препятствие.",
			)
		}
		return reply(
			"Да, это основное препятствие. Давайте разберём, к чему оно приводит.",
			"Вы верно выделили проблему. Теперь важно оценить её последствия для результата.",
			"Это действительно мешает договориться. Что произойдёт, если оставить всё как есть?",
			"Проблема сформулирована точно. Покажите её влияние на сроки, ресурсы или риски.",
		)
	case IntentExploreImplication:
		if session.State.SPINStage < domain.SPINStageProblem {
			return reply(
				"Пока неясно, какую именно проблему вы анализируете. Сформулируйте её точнее.",
				"Сначала назовите конкретное препятствие, иначе последствия получатся слишком общими.",
				"Уточните саму проблему, а затем разберём, к чему она приводит.",
			)
		}
		return reply(
			"Если ничего не менять, риски и издержки действительно возрастут. Какой результат вы предлагаете?",
			"Последствия понятны: без изменений мы потеряем время и увеличим риск. Что должно измениться?",
			"Да, сохранение текущей ситуации создаёт дополнительные затраты. Какое решение снимет их?",
			"Влияние проблемы обозначено. Теперь покажите ценность результата, к которому предлагаете прийти.",
		)
	case IntentClarifyNeedPayoff:
		if session.State.SPINStage < domain.SPINStageImplication {
			return reply(
				"Поясните последствия текущей ситуации, тогда ценность решения будет понятнее.",
				"Сначала покажите цену бездействия — после этого сможем оценить пользу решения.",
				"Ценность пока неочевидна. Свяжите её с последствиями уже выявленной проблемы.",
			)
		}
		return reply(
			"Такой результат был бы полезен. Теперь предложите конкретные условия его достижения.",
			"Польза понятна и выглядит значимой. Как именно вы предлагаете получить этот результат?",
			"Это решает обозначенную проблему. Перейдём к измеримым условиям и обязательствам сторон.",
			"Ценность решения ясна. Сформулируйте вариант соглашения, который её обеспечит.",
		)
	case IntentStateBATNA:
		if !session.State.InterestsExplored {
			return reply(
				"Я услышал вашу альтернативу, но сначала важно понять интересы обеих сторон.",
				"Альтернатива понятна, однако сравнивать варианты рано без ясных приоритетов сторон.",
				"Принял ваш запасной вариант. Теперь уточните, какие интересы должно закрыть соглашение.",
			)
		}
		return reply(
			"Альтернатива понятна. Сравним её с возможным соглашением по объективным критериям.",
			"Ваш запасной вариант зафиксирован. Проверим, может ли сделка дать обеим сторонам больше.",
			"BATNA обозначена. Теперь сравним риски, выгоду и реализуемость каждого варианта.",
			"Это даёт понятную точку сравнения. Предложите условия, которые будут лучше альтернативы.",
		)
	case IntentPropose:
		return proposalReply(session, rules, evaluation)
	case IntentAccept:
		return reply(
			"Договорились. Зафиксируем согласованные условия.",
			"Согласен с этим вариантом. Закрепим условия и обязательства сторон.",
			"Принимаю предложение. Давайте точно зафиксируем итоговую договорённость.",
		)
	case IntentPressure:
		return reply(
			"Давление не помогает договориться. Вернёмся к фактам и интересам сторон.",
			"Ультиматум снижает готовность к соглашению. Обоснуйте позицию без угроз.",
			"В таком тоне конструктивного решения не получится. Предложите проверяемые аргументы.",
			"Я не готов обсуждать условия под давлением. Вернитесь к критериям и взаимной выгоде.",
		)
	default:
		return reply("Уточните вашу позицию.", "Сформулируйте вашу мысль конкретнее.", "Поясните, чего вы хотите добиться этой репликой.")
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
		return replyForTurn(session.Turn,
			"Прежде чем обсуждать конкретные условия, выясните, что важно второй стороне.",
			"Предложение преждевременно: сначала уточните мои приоритеты и ограничения.",
			"Я пока не готов оценивать условия. Сначала покажите, какие мои интересы вы учитываете.",
		)
	}
	if rules.RequiresEvidence && !evaluation.State.EvidencePresented {
		return replyForTurn(session.Turn,
			"Для решения по предложению нужны факты и измеримые аргументы.",
			"Условия понятны, но им не хватает доказательной базы. Подкрепите их данными.",
			"Я смогу оценить предложение после конкретных фактов о результате и рисках.",
		)
	}
	if evaluation.State.LastOfferQuality == domain.OfferQualityRejected {
		return rejectedProposalReply(rules.Proposal, session.Turn)
	}
	projectedTrust := clamp(session.TrustScore + evaluation.TrustDelta)
	if projectedTrust < rules.MinimumTrustForAgreement {
		return replyForTurn(session.Turn,
			"Пока я не готов принять эти условия. Сначала нужно укрепить доверие и снизить риски.",
			"Условия могут обсуждаться, но доверия пока недостаточно. Снимите ключевые риски.",
			"Мне рано соглашаться: сначала нужны гарантии и более предсказуемый план исполнения.",
		)
	}
	if evaluation.State.LastOfferQuality == domain.OfferQualityPreferred {
		return replyForTurn(session.Turn,
			"Предложение учитывает мои приоритеты. Готов зафиксировать эти условия.",
			"Этот вариант соответствует моим ключевым интересам. Можно переходить к подтверждению.",
			"Условия выглядят выгодными и управляемыми. Я готов их принять после фиксации деталей.",
		)
	}
	return replyForTurn(session.Turn,
		"Условия выглядят приемлемо. Если вы их подтверждаете, можем зафиксировать договорённость.",
		"Такой вариант можно принять. Подтвердите параметры, и я зафиксирую соглашение.",
		"Предложение находится в допустимых рамках. Готов перейти к окончательному подтверждению.",
		"Вижу рабочий баланс интересов. Если условия окончательные, можем договориться.",
	)
}

func rejectedProposalReply(constraint domain.ProposalConstraint, turn int) string {
	if constraint.Kind != "none" {
		return replyForTurn(turn,
			fmt.Sprintf("Это выходит за мой предел уступки. Готов обсуждать значение не более %d.", constraint.MaximumValue),
			fmt.Sprintf("Такое значение неприемлемо. Моя верхняя граница — %d.", constraint.MaximumValue),
			fmt.Sprintf("Предложение превышает допустимый предел. Скорректируйте его до %d или ниже.", constraint.MaximumValue),
		)
	}
	return replyForTurn(turn,
		"Этот вариант для меня неприемлем. Нужна другая конфигурация условий.",
		"На такие условия я не соглашусь. Предложите вариант с другим балансом обязательств.",
		"Текущая конфигурация не проходит по моим ограничениям. Пересмотрите ключевые параметры.",
	)
}

func repeatedMoveReply(intent MoveIntent, turn int) string {
	nextStep := "Добавьте новый аргумент или предложите следующий конкретный шаг."
	switch intent {
	case IntentAskInterest:
		nextStep = "Мои приоритеты уже обозначены; свяжите с ними факты или возможные условия."
	case IntentPresentEvidence:
		nextStep = "Приведите новый факт либо объясните, как данные влияют на условия соглашения."
	case IntentAskSituation:
		nextStep = "Исходная ситуация уже ясна; переходите к конкретной проблеме."
	case IntentIdentifyProblem:
		nextStep = "Проблема зафиксирована; разберите её последствия."
	case IntentExploreImplication:
		nextStep = "Последствия уже рассмотрены; покажите ценность решения."
	case IntentClarifyNeedPayoff:
		nextStep = "Польза решения понятна; сформулируйте конкретные условия."
	case IntentStateBATNA:
		nextStep = "Альтернатива зафиксирована; сравните с ней предлагаемое соглашение."
	case IntentPropose:
		nextStep = "Это предложение уже на столе; измените параметры или подтвердите готовность договориться."
	case IntentPressure:
		nextStep = "Повторение ультиматума только снижает доверие; вернитесь к фактам и критериям."
	}
	leads := []string{
		"Мы уже обсудили этот пункт.",
		"Этот шаг уже зафиксирован.",
		"Повтор не продвигает переговоры.",
		"Здесь новой информации пока нет.",
		"Вернёмся к развитию позиции.",
		"Чтобы двигаться дальше, нужен новый элемент.",
	}
	bridges := []string{"", "Нужен другой подход. ", "Добавьте новую информацию. "}
	if turn < 0 {
		turn = 0
	}
	return leads[turn%len(leads)] + " " + bridges[(turn/len(leads))%len(bridges)] + nextStep
}

func neutralReply(turn int) string {
	if turn < 0 {
		turn = 0
	}
	if turn == 0 {
		return "Уточните вашу позицию: задайте вопрос, приведите факт или предложите конкретные условия."
	}
	openers := []string{
		"Пока я не вижу конкретного шага.",
		"Давайте сделаем реплику предметной.",
		"Мне нужна более точная позиция.",
		"Продолжим по существу.",
		"Свяжите вашу мысль с целью переговоров.",
		"Сейчас в реплике не хватает новой информации.",
	}
	actions := []string{
		"Сформулируйте вопрос, аргумент или вариант соглашения.",
		"Скажите, что именно вы хотите выяснить или предложить.",
		"Назовите интерес, факт либо условие, которое хотите обсудить.",
		"Уточните цель этой реплики и ожидаемый результат.",
		"Спросите об ограничениях, приведите данные или предложите решение.",
		"Выберите один конкретный следующий шаг и объясните его пользу.",
	}
	index := (turn - 1) % len(openers)
	cycle := (turn - 1) / len(openers)
	actionIndex := (index + cycle) % len(actions)
	return openers[index] + " " + actions[actionIndex]
}

func replyForTurn(turn int, variants ...string) string {
	if len(variants) == 0 {
		return ""
	}
	if turn < 0 {
		turn = 0
	}
	return variants[turn%len(variants)]
}
