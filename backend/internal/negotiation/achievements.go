package negotiation

import "github.com/kulich00/negotiation-arena/backend/internal/domain"

const (
	AchievementFirstRound       = "first_round"
	AchievementDealMaker        = "deal_maker"
	AchievementMutualBenefit    = "mutual_benefit"
	AchievementCalmNegotiator   = "calm_negotiator"
	AchievementWellPrepared     = "well_prepared"
	AchievementSPINMaster       = "spin_master"
	AchievementCleanRun         = "clean_run"
	AchievementSuccessfulReplay = "successful_replay"
	AchievementDifficultClient  = "difficult_client"
)

var achievementCatalog = []domain.Achievement{
	{Code: AchievementFirstRound, Title: "Первый раунд", Description: "Завершить переговорную сессию хотя бы с одним ходом."},
	{Code: AchievementDealMaker, Title: "Сделка заключена", Description: "Достичь устойчивого соглашения."},
	{Code: AchievementMutualBenefit, Title: "Взаимная выгода", Description: "Завершить переговоры исходом mutual_gain."},
	{Code: AchievementCalmNegotiator, Title: "Холодная голова", Description: "Достичь устойчивого соглашения без давления."},
	{Code: AchievementWellPrepared, Title: "Подготовленный переговорщик", Description: "Исследовать интересы, привести доказательства и определить BATNA."},
	{Code: AchievementSPINMaster, Title: "Мастер SPIN", Description: "Последовательно пройти все четыре этапа SPIN."},
	{Code: AchievementCleanRun, Title: "Чистая партия", Description: "Завершить сессию без классифицированных ошибок."},
	{Code: AchievementSuccessfulReplay, Title: "Работа над ошибками", Description: "Достичь устойчивого соглашения в дочерней ветке сессии."},
	{Code: AchievementDifficultClient, Title: "Крепкие нервы", Description: "Достичь устойчивого соглашения со сложным клиентом."},
}

func AchievementCatalog() []domain.Achievement {
	return append([]domain.Achievement{}, achievementCatalog...)
}

func EvaluateAchievements(session domain.Session, result domain.Result) []domain.Achievement {
	unlocked := make(map[string]bool)
	successfulAgreement := result.OutcomeCode == "mutual_gain" ||
		result.OutcomeCode == "advantageous_agreement" ||
		result.OutcomeCode == "compromise"

	if result.Analysis.AnalyzedTurns > 0 {
		unlocked[AchievementFirstRound] = true
	}
	if successfulAgreement {
		unlocked[AchievementDealMaker] = true
	}
	if result.OutcomeCode == "mutual_gain" {
		unlocked[AchievementMutualBenefit] = true
	}
	if successfulAgreement && session.PressureScore == 0 {
		unlocked[AchievementCalmNegotiator] = true
	}
	if session.State.InterestsExplored && session.State.EvidencePresented && session.State.BATNADefined {
		unlocked[AchievementWellPrepared] = true
	}
	if session.State.SPINStage == domain.SPINStageNeedPayoff {
		unlocked[AchievementSPINMaster] = true
	}
	if result.Analysis.AnalyzedTurns > 0 && len(result.Analysis.ErrorClasses) == 0 {
		unlocked[AchievementCleanRun] = true
	}
	if session.ParentSessionID != "" && successfulAgreement {
		unlocked[AchievementSuccessfulReplay] = true
	}
	if session.State.OpponentMode == OpponentModeDifficult && successfulAgreement {
		unlocked[AchievementDifficultClient] = true
	}

	items := make([]domain.Achievement, 0, len(unlocked))
	for _, achievement := range achievementCatalog {
		if unlocked[achievement.Code] {
			items = append(items, achievement)
		}
	}
	return items
}
