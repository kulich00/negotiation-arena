package negotiation

import (
	"testing"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
)

func TestGenerateOpponentReplyForProposal(t *testing.T) {
	rules := domain.DefaultScenarioRules()
	move := PlayerMove{Intent: IntentPropose, Proposal: &MoveProposal{Kind: "raise_percent", Value: 5}}

	tests := []struct {
		name  string
		state domain.SessionState
		trust int
		want  string
	}{
		{
			name:  "interests missing",
			state: domain.InitialSessionState(),
			trust: 60,
			want:  "Прежде чем обсуждать конкретные условия, выясните, что важно второй стороне.",
		},
		{
			name:  "evidence missing",
			state: domain.SessionState{Phase: domain.PhaseExploration, InterestsExplored: true},
			trust: 60,
			want:  "Для решения по предложению нужны факты и измеримые аргументы.",
		},
		{
			name:  "trust missing",
			state: readyState(),
			trust: rules.MinimumTrustForAgreement - 2,
			want:  "Пока я не готов принять эти условия. Сначала нужно укрепить доверие и снизить риски.",
		},
		{
			name:  "ready",
			state: readyState(),
			trust: rules.MinimumTrustForAgreement - 1,
			want:  "Условия выглядят приемлемо. Если вы их подтверждаете, можем зафиксировать договорённость.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			session := domain.Session{TrustScore: test.trust, State: test.state}
			evaluation := EvaluateMove(move, session.State)
			if got := GenerateOpponentReply(move, session, rules, evaluation); got != test.want {
				t.Fatalf("unexpected reply:\n got: %q\nwant: %q", got, test.want)
			}
		})
	}
}

func TestGenerateOpponentReplyByIntent(t *testing.T) {
	rules := domain.DefaultScenarioRules()
	session := domain.Session{TrustScore: 60, State: readyState()}
	tests := []struct {
		intent MoveIntent
		want   string
	}{
		{IntentAskInterest, "Для меня важно снизить риски и понять взаимную выгоду. Какие варианты вы предлагаете?"},
		{IntentPresentEvidence, "Аргументы стали конкретнее. Покажите, как они связаны с предлагаемыми условиями."},
		{IntentAccept, "Договорились. Зафиксируем согласованные условия."},
		{IntentPressure, "Давление не помогает договориться. Вернёмся к фактам и интересам сторон."},
	}

	for _, test := range tests {
		t.Run(string(test.intent), func(t *testing.T) {
			move := PlayerMove{Intent: test.intent}
			evaluation := EvaluateMove(move, session.State)
			if got := GenerateOpponentReply(move, session, rules, evaluation); got != test.want {
				t.Fatalf("unexpected reply: %q", got)
			}
		})
	}
}

func readyState() domain.SessionState {
	return domain.SessionState{
		Phase:             domain.PhaseExploration,
		InterestsExplored: true,
		EvidencePresented: true,
	}
}
