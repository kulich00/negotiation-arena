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
	case IntentAskInterest, IntentPresentEvidence, IntentAskSituation, IntentIdentifyProblem, IntentExploreImplication, IntentClarifyNeedPayoff, IntentStateBATNA, IntentAccept, IntentPressure:
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
	case IntentAskInterest:
		evaluation.TrustDelta = 2
		evaluation.State.InterestsExplored = true
		advanceToExploration(&evaluation.State)
	case IntentPresentEvidence:
		evaluation.TrustDelta = 1
		evaluation.ArgumentDelta = 2
		evaluation.State.EvidencePresented = true
		advanceToExploration(&evaluation.State)
	case IntentAskSituation:
		evaluation.TrustDelta = 1
		evaluation.State.SituationExplored = true
		if evaluation.State.SPINStage < domain.SPINStageSituation {
			evaluation.State.SPINStage = domain.SPINStageSituation
		}
		advanceToExploration(&evaluation.State)
	case IntentIdentifyProblem:
		evaluation.ArgumentDelta = 1
		evaluation.State.ProblemIdentified = true
		if state.SPINStage >= domain.SPINStageSituation {
			evaluation.TrustDelta = 1
		}
		if state.SPINStage == domain.SPINStageSituation {
			evaluation.State.SPINStage = domain.SPINStageProblem
		}
		advanceToExploration(&evaluation.State)
	case IntentExploreImplication:
		evaluation.ArgumentDelta = 1
		if state.SPINStage >= domain.SPINStageProblem {
			evaluation.ArgumentDelta = 2
		}
		evaluation.State.ImplicationsExplored = true
		if evaluation.State.SPINStage == domain.SPINStageProblem {
			evaluation.State.SPINStage = domain.SPINStageImplication
		}
		advanceToExploration(&evaluation.State)
	case IntentClarifyNeedPayoff:
		evaluation.TrustDelta = 1
		if state.SPINStage >= domain.SPINStageImplication {
			evaluation.TrustDelta = 2
			evaluation.ArgumentDelta = 1
		}
		evaluation.State.NeedPayoffEstablished = true
		if state.SPINStage == domain.SPINStageImplication {
			evaluation.State.SPINStage = domain.SPINStageNeedPayoff
		}
		advanceToExploration(&evaluation.State)
	case IntentStateBATNA:
		evaluation.ArgumentDelta = 1
		evaluation.State.BATNADefined = true
		advanceToExploration(&evaluation.State)
	case IntentPropose:
		evaluation.TrustDelta = 1
		evaluation.State.OfferMade = true
		evaluation.State.Phase = domain.PhaseBargaining
		evaluation.State.LastOfferQuality = proposalQuality(move.Proposal, rules.Proposal)
		if move.Proposal == nil {
			evaluation.State.LastOfferID = "free_form"
		} else if move.Proposal.AlternativeID != "" {
			evaluation.State.LastOfferID = move.Proposal.AlternativeID
		} else {
			evaluation.State.LastOfferID = fmt.Sprintf("%s:%d", move.Proposal.Kind, move.Proposal.Value)
		}
	case IntentAccept:
		evaluation.TrustDelta = 2
		evaluation.State.OfferAccepted = true
		evaluation.State.Phase = domain.PhaseBargaining
	case IntentPressure:
		evaluation.TrustDelta = -2
		evaluation.PressureDelta = 2
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
