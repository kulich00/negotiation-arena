package negotiation

import (
	"slices"
	"testing"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
)

func TestEvaluateResultStructuredAgreement(t *testing.T) {
	rules := domain.DefaultScenarioRules()
	session := domain.Session{
		ID:            "session",
		TrustScore:    60,
		ArgumentScore: 4,
		State: domain.SessionState{
			StructuredMovesUsed: true,
			InterestsExplored:   true,
			EvidencePresented:   true,
			OfferMade:           true,
			OfferAccepted:       true,
		},
	}

	result := EvaluateResult(session, rules)
	if result.FinalScore != 80 || result.Outcome != "Выгодное соглашение" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !slices.Contains(result.Strengths, "Переговоры завершены принятием предложения") {
		t.Fatalf("acceptance strength is missing: %+v", result.Strengths)
	}
}

func TestEvaluateResultStructuredRequirements(t *testing.T) {
	rules := domain.DefaultScenarioRules()
	tests := []struct {
		name           string
		change         func(*domain.Session)
		recommendation string
	}{
		{
			name: "interests not explored",
			change: func(session *domain.Session) {
				session.State.InterestsExplored = false
			},
			recommendation: "Перед предложением выясните интересы второй стороны",
		},
		{
			name: "evidence not presented",
			change: func(session *domain.Session) {
				session.State.EvidencePresented = false
			},
			recommendation: "Подкрепите позицию измеримыми фактами",
		},
		{
			name: "offer not made",
			change: func(session *domain.Session) {
				session.State.OfferMade = false
				session.State.OfferAccepted = false
			},
			recommendation: "Сформулируйте конкретное предложение",
		},
		{
			name: "offer not accepted",
			change: func(session *domain.Session) {
				session.State.OfferAccepted = false
			},
			recommendation: "Убедитесь, что итоговое предложение явно принято",
		},
		{
			name: "trust below threshold",
			change: func(session *domain.Session) {
				session.TrustScore = rules.MinimumTrustForAgreement - 1
			},
			recommendation: "Сначала укрепите доверие второй стороны",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			session := successfulStructuredSession()
			test.change(&session)
			result := EvaluateResult(session, rules)
			if result.Outcome != "Соглашение не достигнуто" {
				t.Fatalf("unexpected outcome: %+v", result)
			}
			if !slices.Contains(result.Recommendations, test.recommendation) {
				t.Fatalf("missing recommendation %q: %+v", test.recommendation, result.Recommendations)
			}
		})
	}
}

func TestEvaluateResultPreservesLegacyOutcome(t *testing.T) {
	rules := domain.DefaultScenarioRules()
	session := domain.Session{ID: "legacy", TrustScore: 55}

	result := EvaluateResult(session, rules)
	if result.Outcome != "Компромисс" || result.FinalScore != 55 {
		t.Fatalf("unexpected legacy result: %+v", result)
	}
}

func TestEvaluateResultUsesWeightsAndPressure(t *testing.T) {
	rules := domain.DefaultScenarioRules()
	rules.TrustScoreWeight = 0.5
	rules.ArgumentScoreWeight = 2
	session := domain.Session{ID: "legacy", TrustScore: 60, ArgumentScore: 4, PressureScore: 3}

	result := EvaluateResult(session, rules)
	if result.FinalScore != 58 {
		t.Fatalf("unexpected weighted score: %+v", result)
	}
	if !slices.Contains(result.Mistakes, "Избыточное давление") {
		t.Fatalf("pressure mistake is missing: %+v", result.Mistakes)
	}
}

func successfulStructuredSession() domain.Session {
	return domain.Session{
		ID:            "session",
		TrustScore:    60,
		ArgumentScore: 3,
		State: domain.SessionState{
			StructuredMovesUsed: true,
			InterestsExplored:   true,
			EvidencePresented:   true,
			OfferMade:           true,
			OfferAccepted:       true,
		},
	}
}
