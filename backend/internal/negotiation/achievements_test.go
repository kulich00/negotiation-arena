package negotiation

import (
	"reflect"
	"testing"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
)

func TestEvaluateAchievementsForSuccessfulReplay(t *testing.T) {
	result := domain.Result{
		OutcomeCode: "mutual_gain",
		Analysis: domain.SessionAnalysis{
			AnalyzedTurns: 4,
			ErrorClasses:  []domain.ErrorClass{},
		},
	}
	session := domain.Session{
		ParentSessionID: "parent",
		State: domain.SessionState{
			InterestsExplored: true,
			EvidencePresented: true,
			BATNADefined:      true,
			SPINStage:         domain.SPINStageNeedPayoff,
			OpponentMode:      OpponentModeDifficult,
		},
	}

	achievements := EvaluateAchievements(session, result)
	codes := make([]string, 0, len(achievements))
	for _, achievement := range achievements {
		codes = append(codes, achievement.Code)
	}
	want := []string{
		AchievementFirstRound,
		AchievementDealMaker,
		AchievementMutualBenefit,
		AchievementCalmNegotiator,
		AchievementWellPrepared,
		AchievementSPINMaster,
		AchievementCleanRun,
		AchievementSuccessfulReplay,
		AchievementDifficultClient,
	}
	if !reflect.DeepEqual(codes, want) {
		t.Fatalf("unexpected achievements: %v", codes)
	}
}

func TestEvaluateAchievementsDoesNotAwardCleanRunWithErrors(t *testing.T) {
	result := domain.Result{
		OutcomeCode: "walk_away",
		Analysis: domain.SessionAnalysis{
			AnalyzedTurns: 1,
			ErrorClasses:  []domain.ErrorClass{{Code: ErrorPressureTactic, Count: 1}},
		},
	}

	achievements := EvaluateAchievements(domain.Session{PressureScore: 2}, result)
	if len(achievements) != 1 || achievements[0].Code != AchievementFirstRound {
		t.Fatalf("unexpected achievements for failed session: %+v", achievements)
	}
}

func TestAchievementCatalogReturnsCopy(t *testing.T) {
	first := AchievementCatalog()
	first[0].Title = "changed"
	second := AchievementCatalog()
	if second[0].Title == "changed" {
		t.Fatal("catalog was mutated through returned data")
	}
}
