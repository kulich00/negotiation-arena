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
	IntentAskInterest     MoveIntent = "ask_interest"
	IntentPresentEvidence MoveIntent = "present_evidence"
	IntentPropose         MoveIntent = "propose"
	IntentAccept          MoveIntent = "accept"
	IntentPressure        MoveIntent = "pressure"
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
	TrustDelta    int
	ArgumentDelta int
	PressureDelta int
	State         domain.SessionState
}

var ErrInvalidMove = errors.New("invalid move")
var ErrTurnLimitReached = errors.New("turn limit reached")

const maxMoveContentLength = 4000

func fmtInvalidMove(reason string) error {
	return fmt.Errorf("%w: %s", ErrInvalidMove, reason)
}

func ValidateMove(move PlayerMove, rules domain.ScenarioRules) error {
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
	case IntentAskInterest, IntentPresentEvidence, IntentAccept, IntentPressure:
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
	if proposal.Value < 1 || proposal.Value > rules.Proposal.MaximumValue {
		return fmt.Errorf("%w: proposal value must be between 1 and %d", ErrInvalidMove, rules.Proposal.MaximumValue)
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
func EvaluateMove(move PlayerMove, state domain.SessionState) MoveEvaluation {
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
	case IntentPropose:
		evaluation.TrustDelta = 1
		evaluation.State.OfferMade = true
		evaluation.State.Phase = domain.PhaseBargaining
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

func advanceToExploration(state *domain.SessionState) {
	if state.Phase == domain.PhaseOpening {
		state.Phase = domain.PhaseExploration
	}
}
