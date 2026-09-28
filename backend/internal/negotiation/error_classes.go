package negotiation

import "github.com/kulich00/negotiation-arena/backend/internal/domain"

const (
	ErrorEvidenceBeforeInterests         = "evidence_before_interests"
	ErrorSPINProblemBeforeSituation      = "spin_problem_before_situation"
	ErrorSPINImplicationBeforeProblem    = "spin_implication_before_problem"
	ErrorSPINNeedPayoffBeforeImplication = "spin_need_payoff_before_implication"
	ErrorBATNABeforeInterests            = "batna_before_interests"
	ErrorProposalBeforeInterests         = "proposal_before_interests"
	ErrorProposalWithoutEvidence         = "proposal_without_evidence"
	ErrorProposalOutsideLimit            = "proposal_outside_limit"
	ErrorPressureTactic                  = "pressure_tactic"
)

func negotiationError(code, label string, severity domain.ErrorSeverity, message string) domain.NegotiationError {
	return domain.NegotiationError{
		Code: code, Label: label, Severity: severity, Message: message,
	}
}

func addNegotiationError(analysis *domain.TurnAnalysis, item domain.NegotiationError) {
	analysis.Errors = append(analysis.Errors, item)
	analysis.Risks = append(analysis.Risks, item.Message)
}

func errorSeverityRank(severity domain.ErrorSeverity) int {
	switch severity {
	case domain.ErrorSeverityCritical:
		return 4
	case domain.ErrorSeverityHigh:
		return 3
	case domain.ErrorSeverityMedium:
		return 2
	case domain.ErrorSeverityLow:
		return 1
	default:
		return 0
	}
}
