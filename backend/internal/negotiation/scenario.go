package negotiation

import (
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
)

var ErrInvalidScenario = errors.New("invalid scenario")

func ValidateScenario(scenario domain.Scenario) error {
	scenario = normalizeScenario(scenario)
	scenario.Rules = scenario.Rules.WithDefaults()
	required := []struct {
		name  string
		value string
		limit int
	}{
		{"title", scenario.Title, 120},
		{"sphere", scenario.Sphere, 80},
		{"topic", scenario.Topic, 300},
		{"opponentRole", scenario.OpponentRole, 120},
		{"opponentTone", scenario.OpponentTone, 200},
		{"playerGoal", scenario.PlayerGoal, 1000},
		{"opponentGoal", scenario.OpponentGoal, 1000},
		{"initialMessage", scenario.InitialMessage, 2000},
	}
	for _, field := range required {
		if field.value == "" {
			return invalidScenario("%s is required", field.name)
		}
		if utf8.RuneCountInString(field.value) > field.limit {
			return invalidScenario("%s must not exceed %d characters", field.name, field.limit)
		}
	}

	if scenario.ID == "" || utf8.RuneCountInString(scenario.ID) > 128 || !validScenarioID(scenario.ID) {
		return invalidScenario("id contains unsupported characters or is too long")
	}
	switch scenario.Difficulty {
	case "easy", "medium", "hard":
	default:
		return invalidScenario("difficulty must be easy, medium, or hard")
	}

	rules := scenario.Rules
	if rules.MaxTurns < 1 || rules.MaxTurns > 50 {
		return invalidScenario("maxTurns must be between 1 and 50")
	}
	if rules.MinimumTrustForAgreement < 1 || rules.MinimumTrustForAgreement > 100 {
		return invalidScenario("minimumTrustForAgreement must be between 1 and 100")
	}
	if rules.MinimumArgumentScoreForAgreement < 1 || rules.MinimumArgumentScoreForAgreement > 100 {
		return invalidScenario("minimumArgumentScoreForAgreement must be between 1 and 100")
	}
	if rules.MaximumPressureForAgreement < 1 || rules.MaximumPressureForAgreement > 100 {
		return invalidScenario("maximumPressureForAgreement must be between 1 and 100")
	}
	if err := validateWeight("trustScoreWeight", rules.TrustScoreWeight); err != nil {
		return err
	}
	if err := validateWeight("argumentScoreWeight", rules.ArgumentScoreWeight); err != nil {
		return err
	}
	if err := validateWeight("pressureScoreWeight", rules.PressureScoreWeight); err != nil {
		return err
	}
	if err := validateProposalConstraint(rules.Proposal); err != nil {
		return err
	}
	return validateOpponentBehavior(rules.Behavior, rules.MaxTurns)
}

func normalizeScenario(scenario domain.Scenario) domain.Scenario {
	scenario.ID = strings.TrimSpace(scenario.ID)
	scenario.Title = strings.TrimSpace(scenario.Title)
	scenario.Sphere = strings.TrimSpace(scenario.Sphere)
	scenario.Topic = strings.TrimSpace(scenario.Topic)
	scenario.Difficulty = strings.ToLower(strings.TrimSpace(scenario.Difficulty))
	scenario.OpponentRole = strings.TrimSpace(scenario.OpponentRole)
	scenario.OpponentTone = strings.TrimSpace(scenario.OpponentTone)
	scenario.PlayerGoal = strings.TrimSpace(scenario.PlayerGoal)
	scenario.OpponentGoal = strings.TrimSpace(scenario.OpponentGoal)
	scenario.InitialMessage = strings.TrimSpace(scenario.InitialMessage)
	scenario.Rules.Proposal.Kind = strings.ToLower(strings.TrimSpace(scenario.Rules.Proposal.Kind))
	alternatives := make([]string, len(scenario.Rules.Proposal.AlternativeIDs))
	for index, alternativeID := range scenario.Rules.Proposal.AlternativeIDs {
		alternatives[index] = strings.TrimSpace(alternativeID)
	}
	scenario.Rules.Proposal.AlternativeIDs = alternatives
	preferredAlternatives := make([]string, len(scenario.Rules.Proposal.PreferredAlternativeIDs))
	for index, alternativeID := range scenario.Rules.Proposal.PreferredAlternativeIDs {
		preferredAlternatives[index] = strings.TrimSpace(alternativeID)
	}
	scenario.Rules.Proposal.PreferredAlternativeIDs = preferredAlternatives
	scenario.Rules.Behavior.Mode = strings.ToLower(strings.TrimSpace(scenario.Rules.Behavior.Mode))
	scenario.Rules.Behavior.InitialPriority = strings.TrimSpace(scenario.Rules.Behavior.InitialPriority)
	priorityShifts := make([]domain.OpponentPriorityShift, len(scenario.Rules.Behavior.PriorityShifts))
	for index, shift := range scenario.Rules.Behavior.PriorityShifts {
		shift.Priority = strings.TrimSpace(shift.Priority)
		priorityShifts[index] = shift
	}
	scenario.Rules.Behavior.PriorityShifts = priorityShifts
	return scenario
}

func validateOpponentBehavior(behavior domain.OpponentBehavior, maxTurns int) error {
	switch behavior.Mode {
	case OpponentModeStandard:
		if behavior.Emotionality != 0 || behavior.Volatility != 0 || behavior.InitialPriority != "" || len(behavior.PriorityShifts) != 0 {
			return invalidScenario("standard behavior cannot define difficult-client settings")
		}
		return nil
	case OpponentModeDifficult:
	default:
		return invalidScenario("behavior mode must be standard or difficult")
	}
	if behavior.Emotionality < 1 || behavior.Emotionality > 3 {
		return invalidScenario("behavior emotionality must be between 1 and 3")
	}
	if behavior.Volatility < 1 || behavior.Volatility > 3 {
		return invalidScenario("behavior volatility must be between 1 and 3")
	}
	if behavior.InitialPriority == "" || utf8.RuneCountInString(behavior.InitialPriority) > 200 {
		return invalidScenario("behavior initialPriority is required and must not exceed 200 characters")
	}
	if len(behavior.PriorityShifts) > 10 {
		return invalidScenario("no more than 10 behavior priority shifts are allowed")
	}
	previousTurn := 0
	for _, shift := range behavior.PriorityShifts {
		if shift.Turn <= previousTurn || shift.Turn > maxTurns {
			return invalidScenario("behavior priority shift turns must be unique, increasing, and within maxTurns")
		}
		if shift.Priority == "" || utf8.RuneCountInString(shift.Priority) > 200 {
			return invalidScenario("behavior priority shift is required and must not exceed 200 characters")
		}
		previousTurn = shift.Turn
	}
	return nil
}

func validateWeight(name string, value float64) error {
	if math.IsNaN(value) || math.IsInf(value, 0) || value <= 0 || value > 10 {
		return invalidScenario("%s must be greater than 0 and at most 10", name)
	}
	return nil
}

func validateProposalConstraint(proposal domain.ProposalConstraint) error {
	if !validMachineID(proposal.Kind) {
		return invalidScenario("proposal kind is invalid")
	}
	if proposal.Kind == "none" {
		if proposal.PreferredValue != 0 || proposal.MaximumValue != 0 || proposal.InputMaximumValue != 0 {
			return invalidScenario("numeric proposal values must be 0 when proposal kind is none")
		}
	} else {
		if proposal.PreferredValue < 1 || proposal.PreferredValue > proposal.MaximumValue {
			return invalidScenario("preferredValue must be between 1 and maximumValue")
		}
		if proposal.MaximumValue < 1 || proposal.MaximumValue > proposal.InputMaximumValue {
			return invalidScenario("maximumValue must be between 1 and inputMaximumValue")
		}
		if proposal.InputMaximumValue < 1 || proposal.InputMaximumValue > 1_000_000 {
			return invalidScenario("inputMaximumValue must be between 1 and 1000000")
		}
	}
	if len(proposal.AlternativeIDs) > 20 {
		return invalidScenario("no more than 20 proposal alternatives are allowed")
	}
	seen := make(map[string]struct{}, len(proposal.AlternativeIDs))
	for _, alternativeID := range proposal.AlternativeIDs {
		if !validMachineID(alternativeID) || alternativeID == "none" {
			return invalidScenario("proposal alternative id is invalid")
		}
		if _, exists := seen[alternativeID]; exists {
			return invalidScenario("proposal alternative ids must be unique")
		}
		seen[alternativeID] = struct{}{}
	}
	preferredSeen := make(map[string]struct{}, len(proposal.PreferredAlternativeIDs))
	for _, alternativeID := range proposal.PreferredAlternativeIDs {
		if _, exists := seen[alternativeID]; !exists {
			return invalidScenario("preferred proposal alternatives must be allowed alternatives")
		}
		if _, exists := preferredSeen[alternativeID]; exists {
			return invalidScenario("preferred proposal alternative ids must be unique")
		}
		preferredSeen[alternativeID] = struct{}{}
	}
	return nil
}

func validScenarioID(value string) bool {
	for _, char := range value {
		if !unicode.IsLetter(char) && !unicode.IsDigit(char) && char != '-' && char != '_' {
			return false
		}
	}
	return value != ""
}

func validMachineID(value string) bool {
	if value == "" || len(value) > 64 {
		return false
	}
	for index, char := range value {
		if char >= 'a' && char <= 'z' || index > 0 && char >= '0' && char <= '9' || index > 0 && char == '_' {
			continue
		}
		return false
	}
	return true
}

func invalidScenario(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidScenario, fmt.Sprintf(format, args...))
}
