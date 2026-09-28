package negotiation

import "github.com/kulich00/negotiation-arena/backend/internal/domain"

const (
	OpponentModeStandard  = "standard"
	OpponentModeDifficult = "difficult"

	OpponentMoodNeutral   = "neutral"
	OpponentMoodGuarded   = "guarded"
	OpponentMoodIrritated = "irritated"
	OpponentMoodReceptive = "receptive"
	OpponentMoodUncertain = "uncertain"
)

func InitializeOpponentState(state *domain.SessionState, rules domain.ScenarioRules) {
	rules = rules.WithDefaults()
	state.OpponentMode = rules.Behavior.Mode
	state.OpponentMood = OpponentMoodNeutral
	state.OpponentPriority = rules.Behavior.InitialPriority
	state.AppliedPriorityShifts = 0
}

// ApplyOpponentBehavior adds deterministic reactions on top of the core move
// evaluation. The turn is one-based, so checkpoint restoration produces the
// same reaction sequence after a fork.
func ApplyOpponentBehavior(move PlayerMove, turn int, rules domain.ScenarioRules, evaluation MoveEvaluation) MoveEvaluation {
	rules = rules.WithDefaults()
	behavior := rules.Behavior
	if behavior.Mode != OpponentModeDifficult {
		return evaluation
	}
	if evaluation.State.OpponentMode == "" {
		InitializeOpponentState(&evaluation.State, rules)
	}

	previousMood := evaluation.State.OpponentMood
	priorityChanged := false
	for evaluation.State.AppliedPriorityShifts < len(behavior.PriorityShifts) {
		shift := behavior.PriorityShifts[evaluation.State.AppliedPriorityShifts]
		if shift.Turn > turn {
			break
		}
		evaluation.State.OpponentPriority = shift.Priority
		evaluation.State.AppliedPriorityShifts++
		priorityChanged = true
	}

	mood := OpponentMoodNeutral
	behaviorTrustDelta := 0
	switch {
	case move.Intent == IntentPressure:
		mood = OpponentMoodIrritated
		behaviorTrustDelta = -behavior.Emotionality
	case isDeescalatingMove(move.Intent) && (previousMood == OpponentMoodIrritated || previousMood == OpponentMoodGuarded || previousMood == OpponentMoodUncertain):
		mood = OpponentMoodReceptive
		behaviorTrustDelta = 1
	case priorityChanged:
		mood = OpponentMoodUncertain
		behaviorTrustDelta = -1
	case volatileTurn(turn, behavior.Volatility):
		mood = OpponentMoodGuarded
		behaviorTrustDelta = -1
	}

	evaluation.State.OpponentMood = mood
	evaluation.TrustDelta += behaviorTrustDelta
	reaction := domain.OpponentReaction{
		Mood: mood, Priority: evaluation.State.OpponentPriority,
		PriorityChanged: priorityChanged, TrustDelta: behaviorTrustDelta,
	}
	evaluation.OpponentReaction = &reaction
	return evaluation
}

func isDeescalatingMove(intent MoveIntent) bool {
	switch intent {
	case IntentAskInterest, IntentAskSituation, IntentIdentifyProblem, IntentClarifyNeedPayoff:
		return true
	default:
		return false
	}
}

func volatileTurn(turn, volatility int) bool {
	if volatility < 1 {
		return false
	}
	period := 5 - volatility
	return period > 0 && turn%period == 0
}
