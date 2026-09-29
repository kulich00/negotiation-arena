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
