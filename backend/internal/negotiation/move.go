package negotiation

import (
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
)

type MoveIntent string

const (
	IntentNeutral            MoveIntent = "neutral"
	IntentAskInterest        MoveIntent = "ask_interest"
	IntentPresentEvidence    MoveIntent = "present_evidence"
	IntentAskSituation       MoveIntent = "ask_situation"
	IntentIdentifyProblem    MoveIntent = "identify_problem"
	IntentExploreImplication MoveIntent = "explore_implication"
	IntentClarifyNeedPayoff  MoveIntent = "clarify_need_payoff"
	IntentStateBATNA         MoveIntent = "state_batna"
	IntentPropose            MoveIntent = "propose"
	IntentAccept             MoveIntent = "accept"
	IntentPressure           MoveIntent = "pressure"
)

type PlayerMove struct {
	Content  string        `json:"content"`
	Intent   MoveIntent    `json:"intent"`
	Proposal *MoveProposal `json:"proposal,omitempty"`
}

type MoveProposal struct {
	Kind          string `json:"kind"`
	Value         int    `json:"value"`
	AlternativeID string `json:"alternativeId"`
}

type MoveEvaluation struct {
	TrustDelta       int
	ArgumentDelta    int
	PressureDelta    int
	Repeated         bool
	State            domain.SessionState
	OpponentReaction *domain.OpponentReaction
}

var ErrInvalidMove = errors.New("invalid move")
var ErrTurnLimitReached = errors.New("turn limit reached")

const maxMoveContentLength = 4000

func fmtInvalidMove(reason string) error {
	return fmt.Errorf("%w: %s", ErrInvalidMove, reason)
}

func ValidateMove(move PlayerMove, rules domain.ScenarioRules) error {
	rules = rules.WithDefaults()
	if err := validateMoveContent(move.Content); err != nil {
		return err
	}

	switch move.Intent {
	case IntentPropose:
		freeFormProposal := rules.Proposal.Kind == "none" && len(rules.Proposal.AlternativeIDs) == 0
		if move.Proposal == nil && !freeFormProposal {
			return fmt.Errorf("%w: proposal is required", ErrInvalidMove)
		}
		if move.Proposal != nil && freeFormProposal {
			return fmt.Errorf("%w: proposal details are not supported by this scenario", ErrInvalidMove)
		}
	case IntentNeutral, IntentAskInterest, IntentPresentEvidence, IntentAskSituation, IntentIdentifyProblem, IntentExploreImplication, IntentClarifyNeedPayoff, IntentStateBATNA, IntentAccept, IntentPressure:
		if move.Proposal != nil {
			return fmt.Errorf("%w: proposal is only allowed for propose", ErrInvalidMove)
		}
	default:
		return fmt.Errorf("%w: unknown intent", ErrInvalidMove)
	}

	if move.Intent != IntentPropose {
		return nil
	}
	if move.Proposal == nil {
		return nil
	}

	proposal := move.Proposal
	if proposal.AlternativeID != "" {
		if proposal.Kind != "" || proposal.Value != 0 {
			return fmt.Errorf("%w: alternative cannot contain kind or value", ErrInvalidMove)
		}
		for _, allowed := range rules.Proposal.AlternativeIDs {
			if proposal.AlternativeID == allowed {
				return nil
			}
		}
		return fmt.Errorf("%w: unknown alternative", ErrInvalidMove)
	}

	if proposal.Kind != rules.Proposal.Kind {
		return fmt.Errorf("%w: proposal kind must be %q", ErrInvalidMove, rules.Proposal.Kind)
	}
	if proposal.Value < 1 || proposal.Value > rules.Proposal.InputMaximumValue {
		return fmt.Errorf("%w: proposal value must be between 1 and %d", ErrInvalidMove, rules.Proposal.InputMaximumValue)
	}
	return nil
}

func validateMoveContent(content string) error {
	if strings.TrimSpace(content) == "" {
		return fmt.Errorf("%w: content is required", ErrInvalidMove)
	}
	if utf8.RuneCountInString(content) > maxMoveContentLength {
		return fmt.Errorf("%w: content must not exceed %d characters", ErrInvalidMove, maxMoveContentLength)
	}
	return nil
}

// EvaluateMove applies deterministic score and state changes. Opponent replies
// for structured moves are selected separately by GenerateOpponentReply.
func EvaluateMove(move PlayerMove, state domain.SessionState, rules domain.ScenarioRules) MoveEvaluation {
	rules = rules.WithDefaults()
	evaluation := MoveEvaluation{State: state}
	evaluation.State.StructuredMovesUsed = true

	switch move.Intent {
	case IntentNeutral:
		// A neutral or unclear phrase advances the turn without rewarding or
		// penalizing it. Hostile phrases are classified separately as pressure.
	case IntentAskInterest:
		if state.InterestsExplored {
			evaluation.Repeated = true
		} else {
			evaluation.TrustDelta = 2
		}
		evaluation.State.InterestsExplored = true
		advanceToExploration(&evaluation.State)
	case IntentPresentEvidence:
		switch {
		case state.EvidencePresented:
			// A genuinely new fact may still strengthen the case, but repeating
			// evidence no longer builds trust indefinitely.
			evaluation.ArgumentDelta = 1
		case !rules.RequiresInterestExploration || state.InterestsExplored:
			evaluation.TrustDelta = 1
			evaluation.ArgumentDelta = 2
		default:
			// A fact without a connection to the opponent's interests has some
			// argumentative value, but does not build trust yet.
			evaluation.ArgumentDelta = 1
		}
		evaluation.State.EvidencePresented = true
		advanceToExploration(&evaluation.State)
	case IntentAskSituation:
		if state.SituationExplored {
			evaluation.Repeated = true
		} else {
			evaluation.TrustDelta = 1
		}
		evaluation.State.SituationExplored = true
		if evaluation.State.SPINStage < domain.SPINStageSituation {
			evaluation.State.SPINStage = domain.SPINStageSituation
		}
		advanceToExploration(&evaluation.State)
	case IntentIdentifyProblem:
		evaluation.State.ProblemIdentified = true
		if state.ProblemIdentified {
			evaluation.Repeated = true
		} else if state.SPINStage >= domain.SPINStageSituation {
			evaluation.ArgumentDelta = 1
			evaluation.TrustDelta = 1
		}
		if state.SPINStage == domain.SPINStageSituation {
			evaluation.State.SPINStage = domain.SPINStageProblem
		}
		advanceToExploration(&evaluation.State)
	case IntentExploreImplication:
		if state.ImplicationsExplored {
			evaluation.Repeated = true
		} else if state.SPINStage >= domain.SPINStageProblem {
			evaluation.ArgumentDelta = 2
		}
		evaluation.State.ImplicationsExplored = true
		if evaluation.State.SPINStage == domain.SPINStageProblem {
			evaluation.State.SPINStage = domain.SPINStageImplication
		}
		advanceToExploration(&evaluation.State)
	case IntentClarifyNeedPayoff:
		if state.NeedPayoffEstablished {
			evaluation.Repeated = true
		} else if state.SPINStage >= domain.SPINStageImplication {
			evaluation.TrustDelta = 2
			evaluation.ArgumentDelta = 1
		}
		evaluation.State.NeedPayoffEstablished = true
		if state.SPINStage == domain.SPINStageImplication {
			evaluation.State.SPINStage = domain.SPINStageNeedPayoff
		}
		advanceToExploration(&evaluation.State)
	case IntentStateBATNA:
		if state.BATNADefined {
			evaluation.Repeated = true
		} else if state.InterestsExplored {
			evaluation.ArgumentDelta = 1
		}
		evaluation.State.BATNADefined = true
		advanceToExploration(&evaluation.State)
	case IntentPropose:
		evaluation.State.OfferMade = true
		evaluation.State.Phase = domain.PhaseBargaining
		evaluation.State.LastOfferQuality = proposalQuality(move.Proposal, rules.Proposal)
		prerequisitesMet := (!rules.RequiresInterestExploration || state.InterestsExplored) &&
			(!rules.RequiresEvidence || state.EvidencePresented)
		switch {
		case evaluation.State.LastOfferQuality == domain.OfferQualityRejected:
			evaluation.TrustDelta = -2
		case prerequisitesMet:
			evaluation.TrustDelta = 1
		}
		if move.Proposal == nil {
			evaluation.State.LastOfferID = "free_form"
		} else if move.Proposal.AlternativeID != "" {
			evaluation.State.LastOfferID = move.Proposal.AlternativeID
		} else {
			evaluation.State.LastOfferID = fmt.Sprintf("%s:%d", move.Proposal.Kind, move.Proposal.Value)
		}
		if state.OfferMade && state.LastOfferID == evaluation.State.LastOfferID {
			evaluation.Repeated = true
		}
	case IntentAccept:
		evaluation.TrustDelta = 2
		evaluation.State.OfferAccepted = true
		evaluation.State.Phase = domain.PhaseBargaining
	case IntentPressure:
		evaluation.TrustDelta = -2
		evaluation.PressureDelta = 2
	}

	if evaluation.Repeated {
		return suppressRepeatedRewards(evaluation)
	}
	return evaluation
}

func suppressRepeatedRewards(evaluation MoveEvaluation) MoveEvaluation {
	evaluation.Repeated = true
	if evaluation.TrustDelta > 0 {
		evaluation.TrustDelta = 0
	}
	if evaluation.ArgumentDelta > 0 {
		evaluation.ArgumentDelta = 0
	}
	if evaluation.OpponentReaction != nil && evaluation.OpponentReaction.TrustDelta > 0 {
		evaluation.OpponentReaction.TrustDelta = 0
	}
	return evaluation
}

func proposalQuality(proposal *MoveProposal, constraint domain.ProposalConstraint) domain.OfferQuality {
	if proposal == nil {
		return domain.OfferQualityAcceptable
	}
	if proposal.AlternativeID != "" {
		for _, preferred := range constraint.PreferredAlternativeIDs {
			if proposal.AlternativeID == preferred {
				return domain.OfferQualityPreferred
			}
		}
		return domain.OfferQualityAcceptable
	}
	if proposal.Value <= constraint.PreferredValue {
		return domain.OfferQualityPreferred
	}
	if proposal.Value <= constraint.MaximumValue {
		return domain.OfferQualityAcceptable
	}
	return domain.OfferQualityRejected
}

func advanceToExploration(state *domain.SessionState) {
	if state.Phase == domain.PhaseOpening {
		state.Phase = domain.PhaseExploration
	}
}
