package llm

import (
	"context"
	"errors"
	"testing"
)

func TestRuleBasedMoveInterpreterUsesMeaningInsteadOfQuestionMark(t *testing.T) {
	interpreter := RuleBasedMoveInterpreter{}
	tests := []struct {
		name    string
		message string
		want    string
	}{
		{name: "interest", message: "Что для вас важно при выборе поставщика?", want: "ask_interest"},
		{name: "evidence", message: "По данным теста результат вырос на 20 процентов.", want: "present_evidence"},
		{name: "problem", message: "Что мешает согласовать условия?", want: "identify_problem"},
		{name: "generic question", message: "Ну и что?", want: "neutral"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := interpreter.InterpretMove(context.Background(), InterpretationRequest{Message: test.message})
			if err != nil {
				t.Fatal(err)
			}
			if result.Intent != test.want {
				t.Fatalf("intent = %q, want %q", result.Intent, test.want)
			}
		})
	}
}

func TestRuleBasedMoveInterpreterExtractsProposal(t *testing.T) {
	result, err := (RuleBasedMoveInterpreter{}).InterpretMove(context.Background(), InterpretationRequest{
		Message: "Предлагаю повышение на 7 процентов.", ProposalKind: "raise_percent", ProposalMaximum: 30,
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Intent != "propose" || result.ProposalValue != 7 {
		t.Fatalf("unexpected proposal: %+v", result)
	}
}

func TestRuleBasedMoveInterpreterRecognizesNaturalOfflinePhrases(t *testing.T) {
	tests := []struct {
		name      string
		message   string
		offerMade bool
		want      string
	}{
		{name: "interest without question mark", message: "Расскажите, что для вас важно при выборе подрядчика", want: "ask_interest"},
		{name: "interest criteria", message: "Хочу понять ваши главные критерии", want: "ask_interest"},
		{name: "situation", message: "Как сейчас устроен процесс согласования?", want: "ask_situation"},
		{name: "decision maker situation", message: "Кто принимает окончательное решение?", want: "ask_situation"},
		{name: "problem", message: "Какая сложность мешает уложиться в срок?", want: "identify_problem"},
		{name: "implication", message: "Что произойдёт, если ничего не менять?", want: "explore_implication"},
		{name: "implication consequences", message: "Каковы последствия бездействия?", want: "explore_implication"},
		{name: "consequence statement is neutral", message: "Последствия уже понятны", want: "neutral"},
		{name: "need payoff", message: "Какую пользу даст автоматизация?", want: "clarify_need_payoff"},
		{name: "batna", message: "Если не договоримся, я выберу другого поставщика", want: "state_batna"},
		{name: "proposal", message: "Предлагаю начать с пробной поставки", want: "propose"},
		{name: "evidence percent", message: "Конверсия выросла на 18 процентов", want: "present_evidence"},
		{name: "evidence duration", message: "Тест показал сокращение срока на 4 дня", want: "present_evidence"},
		{name: "accept suitable", message: "Меня устраивает этот вариант, фиксируем", offerMade: true, want: "accept"},
		{name: "pressure", message: "У вас нет выбора, вы обязаны согласиться", want: "pressure"},
		{name: "number is not evidence", message: "У меня есть 2 вопроса", want: "neutral"},
		{name: "list is not evidence", message: "Давайте обсудим 3 пункта", want: "neutral"},
		{name: "greeting", message: "Добрый день, рад встрече", want: "neutral"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result, err := (RuleBasedMoveInterpreter{}).InterpretMove(context.Background(), InterpretationRequest{
				Message: test.message, OfferMade: test.offerMade, ProposalKind: "none",
			})
			if err != nil {
				t.Fatal(err)
			}
			if result.Intent != test.want {
				t.Fatalf("intent = %q, want %q", result.Intent, test.want)
			}
		})
	}
}

func TestFallbackReplyGeneratorPrefersExplicitLocalClassification(t *testing.T) {
	generator := NewFallbackReplyGenerator(conflictingGenerator{}, NewPassthroughReplyGenerator(), nil)
	result, err := generator.InterpretMove(context.Background(), InterpretationRequest{
		Message: "Что для вас важно при выборе поставщика?",
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.Intent != "ask_interest" {
		t.Fatalf("explicit local intent was replaced with %q", result.Intent)
	}
}

type conflictingGenerator struct{}

func (conflictingGenerator) GenerateReply(context.Context, ReplyRequest) (string, error) {
	return "", errors.New("not used")
}

func (conflictingGenerator) InterpretMove(context.Context, InterpretationRequest) (MoveInterpretation, error) {
	return MoveInterpretation{Intent: "ask_situation"}, nil
}
