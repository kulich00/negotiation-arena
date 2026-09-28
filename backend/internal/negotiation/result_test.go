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
		outcomeCode    string
	}{
		{
			name: "interests not explored",
			change: func(session *domain.Session) {
				session.State.InterestsExplored = false
			},
			recommendation: "Перед предложением выясните интересы второй стороны",
			outcomeCode:    "fragile_agreement",
		},
		{
			name: "evidence not presented",
			change: func(session *domain.Session) {
				session.State.EvidencePresented = false
			},
			recommendation: "Подкрепите позицию измеримыми фактами",
			outcomeCode:    "fragile_agreement",
		},
		{
			name: "offer not made",
			change: func(session *domain.Session) {
				session.State.OfferMade = false
				session.State.OfferAccepted = false
			},
			recommendation: "Сформулируйте конкретное предложение",
			outcomeCode:    "no_agreement",
		},
		{
			name: "offer not accepted",
			change: func(session *domain.Session) {
				session.State.OfferAccepted = false
			},
			recommendation: "Убедитесь, что итоговое предложение явно принято",
			outcomeCode:    "no_agreement",
		},
		{
			name: "trust below threshold",
			change: func(session *domain.Session) {
				session.TrustScore = rules.MinimumTrustForAgreement - 1
			},
			recommendation: "Сначала укрепите доверие второй стороны",
			outcomeCode:    "fragile_agreement",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			session := successfulStructuredSession()
			test.change(&session)
			result := EvaluateResult(session, rules)
			if result.OutcomeCode != test.outcomeCode {
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

func TestEvaluateResultIncludesSPINAndBATNAFeedback(t *testing.T) {
	rules := domain.DefaultScenarioRules()
	session := successfulStructuredSession()
	session.State.SituationExplored = true
	session.State.ProblemIdentified = true
	session.State.ImplicationsExplored = true
	session.State.NeedPayoffEstablished = true
	session.State.SPINStage = domain.SPINStageNeedPayoff
	session.State.BATNADefined = true

	result := EvaluateResult(session, rules)
	if !slices.Contains(result.Strengths, "Последовательно пройдены все этапы SPIN") {
		t.Fatalf("SPIN strength is missing: %+v", result.Strengths)
	}
	if !slices.Contains(result.Strengths, "Определена альтернатива на случай отсутствия соглашения") {
		t.Fatalf("BATNA strength is missing: %+v", result.Strengths)
	}
}

func TestEvaluateResultDoesNotTreatOutOfOrderSPINAsComplete(t *testing.T) {
	rules := domain.DefaultScenarioRules()
	session := successfulStructuredSession()
	session.State.ProblemIdentified = true
	session.State.ImplicationsExplored = true
	session.State.NeedPayoffEstablished = true

	result := EvaluateResult(session, rules)
	if slices.Contains(result.Strengths, "Последовательно пройдены все этапы SPIN") {
		t.Fatalf("out-of-order SPIN was treated as complete: %+v", result.Strengths)
	}
	if !slices.Contains(result.Recommendations, "Начните SPIN-анализ с уточнения ситуации") {
		t.Fatalf("missing restart recommendation: %+v", result.Recommendations)
	}
}

func TestEvaluateResultRecommendsNextSPINStepAndBATNA(t *testing.T) {
	rules := domain.DefaultScenarioRules()
	session := successfulStructuredSession()
	session.State.SituationExplored = true
	session.State.SPINStage = domain.SPINStageSituation

	result := EvaluateResult(session, rules)
	if !slices.Contains(result.Recommendations, "Продолжите SPIN-анализ формулированием проблемы") {
		t.Fatalf("next SPIN step is missing: %+v", result.Recommendations)
	}
	if !slices.Contains(result.Recommendations, "Заранее определите BATNA и границу приемлемого соглашения") {
		t.Fatalf("BATNA recommendation is missing: %+v", result.Recommendations)
	}
}

func TestEvaluateResultDistinguishesOpponentOutcomes(t *testing.T) {
	rules := domain.DefaultScenarioRules()

	mutualGain := successfulStructuredSession()
	mutualGain.ArgumentScore = 4
	mutualGain.State.LastOfferQuality = domain.OfferQualityPreferred
	result := EvaluateResult(mutualGain, rules)
	if result.OutcomeCode != "mutual_gain" || result.Outcome != "Взаимовыгодное соглашение" {
		t.Fatalf("unexpected mutual-gain result: %+v", result)
	}

	rejected := successfulStructuredSession()
	rejected.State.LastOfferQuality = domain.OfferQualityRejected
	result = EvaluateResult(rejected, rules)
	if result.OutcomeCode != "walk_away" || result.Outcome != "Оппонент отказался от соглашения" {
		t.Fatalf("unexpected rejected-offer result: %+v", result)
	}

	pressured := successfulStructuredSession()
	pressured.PressureScore = rules.MaximumPressureForAgreement + 1
	result = EvaluateResult(pressured, rules)
	if result.OutcomeCode != "walk_away" {
		t.Fatalf("unexpected excessive-pressure result: %+v", result)
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
