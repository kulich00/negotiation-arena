package negotiation

import (
	"testing"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
)

func TestGenerateOpponentReplyForProposal(t *testing.T) {
	rules := domain.DefaultScenarioRules()
	rules.Proposal = domain.ProposalConstraint{Kind: "raise_percent", PreferredValue: 3, MaximumValue: 10, InputMaximumValue: 20}
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
			evaluation := EvaluateMove(move, session.State, rules)
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
			evaluation := MoveEvaluation{State: session.State}
			if got := GenerateOpponentReply(move, session, rules, evaluation); got != test.want {
				t.Fatalf("unexpected reply: %q", got)
			}
		})
	}
}

func TestGenerateOpponentReplyForSPINAndBATNA(t *testing.T) {
	rules := domain.DefaultScenarioRules()
	tests := []struct {
		name  string
		move  PlayerMove
		state domain.SessionState
		want  string
	}{
		{
			name:  "situation",
			move:  PlayerMove{Intent: IntentAskSituation},
			state: domain.InitialSessionState(),
			want:  "Сейчас для меня важны сроки, доступные ресурсы и предсказуемость результата.",
		},
		{
			name:  "problem without context",
			move:  PlayerMove{Intent: IntentIdentifyProblem},
			state: domain.InitialSessionState(),
			want:  "Сначала уточните исходные условия, чтобы мы одинаково понимали проблему.",
		},
		{
			name:  "implication after problem",
			move:  PlayerMove{Intent: IntentExploreImplication},
			state: domain.SessionState{ProblemIdentified: true, SPINStage: domain.SPINStageProblem},
			want:  "Если ничего не менять, риски и издержки действительно возрастут. Какой результат вы предлагаете?",
		},
		{
			name:  "need payoff after implications",
			move:  PlayerMove{Intent: IntentClarifyNeedPayoff},
			state: domain.SessionState{ImplicationsExplored: true, SPINStage: domain.SPINStageImplication},
			want:  "Такой результат был бы полезен. Теперь предложите конкретные условия его достижения.",
		},
		{
			name:  "BATNA after interests",
			move:  PlayerMove{Intent: IntentStateBATNA},
			state: domain.SessionState{InterestsExplored: true},
			want:  "Альтернатива понятна. Сравним её с возможным соглашением по объективным критериям.",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			session := domain.Session{TrustScore: 50, State: test.state}
			evaluation := EvaluateMove(test.move, session.State, rules)
			if got := GenerateOpponentReply(test.move, session, rules, evaluation); got != test.want {
				t.Fatalf("unexpected reply:\n got: %q\nwant: %q", got, test.want)
			}
		})
	}
}

func TestGenerateOpponentReplyRejectsOfferBeyondConcessionLimit(t *testing.T) {
	rules := domain.DefaultScenarioRules()
	rules.RequiresInterestExploration = false
	rules.RequiresEvidence = false
	rules.Proposal = domain.ProposalConstraint{Kind: "raise_percent", PreferredValue: 5, MaximumValue: 10, InputMaximumValue: 30}
	move := PlayerMove{Intent: IntentPropose, Proposal: &MoveProposal{Kind: "raise_percent", Value: 20}}
	session := domain.Session{TrustScore: 70, State: domain.InitialSessionState()}
	evaluation := EvaluateMove(move, session.State, rules)

	got := GenerateOpponentReply(move, session, rules, evaluation)
	want := "Это выходит за мой предел уступки. Готов обсуждать значение не более 10."
	if got != want {
		t.Fatalf("unexpected reply: %q", got)
	}
}

func TestGenerateOpponentReplyVariesByTurnWithoutLLM(t *testing.T) {
	tests := []struct {
		name string
		move PlayerMove
	}{
		{name: "neutral", move: PlayerMove{Intent: IntentNeutral}},
		{name: "interests", move: PlayerMove{Intent: IntentAskInterest}},
		{name: "evidence", move: PlayerMove{Intent: IntentPresentEvidence}},
		{name: "situation", move: PlayerMove{Intent: IntentAskSituation}},
		{name: "pressure", move: PlayerMove{Intent: IntentPressure}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			seen := make(map[string]struct{})
			previous := ""
			for turn := 0; turn < 4; turn++ {
				session := domain.Session{Turn: turn, TrustScore: 60, State: readyState()}
				evaluation := MoveEvaluation{State: session.State}
				reply := GenerateOpponentReply(test.move, session, domain.DefaultScenarioRules(), evaluation)
				if reply == previous {
					t.Fatalf("consecutive duplicate on turn %d: %q", turn, reply)
				}
				if len([]rune(reply)) < 20 {
					t.Fatalf("reply is not actionable: %q", reply)
				}
				seen[reply] = struct{}{}
				previous = reply
			}
			if len(seen) < 4 {
				t.Fatalf("only %d unique replies generated", len(seen))
			}
		})
	}
}

func TestRepeatedMoveRepliesExplainNextStepAndDoNotLoop(t *testing.T) {
	intents := []MoveIntent{
		IntentAskInterest,
		IntentPresentEvidence,
		IntentAskSituation,
		IntentIdentifyProblem,
		IntentExploreImplication,
		IntentClarifyNeedPayoff,
		IntentStateBATNA,
		IntentPropose,
		IntentPressure,
	}
	for _, intent := range intents {
		t.Run(string(intent), func(t *testing.T) {
			seen := make(map[string]struct{})
			previous := ""
			for turn := 0; turn < 6; turn++ {
				session := domain.Session{Turn: turn, TrustScore: 60, State: readyState()}
				reply := GenerateOpponentReply(
					PlayerMove{Intent: intent}, session, domain.DefaultScenarioRules(),
					MoveEvaluation{State: session.State, Repeated: true},
				)
				if reply == previous {
					t.Fatalf("consecutive duplicate on turn %d: %q", turn, reply)
				}
				if len([]rune(reply)) < 40 {
					t.Fatalf("repeated move reply lacks a next step: %q", reply)
				}
				seen[reply] = struct{}{}
				previous = reply
			}
			if len(seen) != 6 {
				t.Fatalf("got %d unique replies, want 6", len(seen))
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
