package domain

import "time"

type Scenario struct {
	ID             string        `json:"id"`
	Title          string        `json:"title"`
	Sphere         string        `json:"sphere"`
	Topic          string        `json:"topic"`
	Difficulty     string        `json:"difficulty"`
	OpponentRole   string        `json:"opponentRole"`
	OpponentTone   string        `json:"opponentTone"`
	PlayerGoal     string        `json:"playerGoal"`
	OpponentGoal   string        `json:"opponentGoal"`
	InitialMessage string        `json:"initialMessage"`
	Rules          ScenarioRules `json:"rules"`
}

type ScenarioRules struct {
	MaxTurns                    int                `json:"maxTurns"`
	MinimumTrustForAgreement    int                `json:"minimumTrustForAgreement"`
	RequiresInterestExploration bool               `json:"requiresInterestExploration"`
	RequiresEvidence            bool               `json:"requiresEvidence"`
	Proposal                    ProposalConstraint `json:"proposal"`
	TrustScoreWeight            float64            `json:"trustScoreWeight"`
	ArgumentScoreWeight         float64            `json:"argumentScoreWeight"`
	PressureScoreWeight         float64            `json:"pressureScoreWeight"`
}

// ProposalConstraint describes the numeric concession and named alternatives
// the opponent may accept. The engine will interpret these rules in a later step.
type ProposalConstraint struct {
	Kind           string   `json:"kind"`
	MaximumValue   int      `json:"maximumValue"`
	AlternativeIDs []string `json:"alternativeIds"`
}

func DefaultScenarioRules() ScenarioRules {
	return ScenarioRules{
		MaxTurns:                    8,
		MinimumTrustForAgreement:    55,
		RequiresInterestExploration: true,
		RequiresEvidence:            true,
		Proposal:                    ProposalConstraint{Kind: "none", AlternativeIDs: []string{}},
		TrustScoreWeight:            1,
		ArgumentScoreWeight:         1,
		PressureScoreWeight:         1,
	}
}

func (r ScenarioRules) WithDefaults() ScenarioRules {
	defaults := DefaultScenarioRules()
	if r.MaxTurns == 0 && r.MinimumTrustForAgreement == 0 && !r.RequiresInterestExploration && !r.RequiresEvidence && r.Proposal.Kind == "" && r.TrustScoreWeight == 0 && r.ArgumentScoreWeight == 0 && r.PressureScoreWeight == 0 {
		return defaults
	}
	if r.MaxTurns <= 0 {
		r.MaxTurns = defaults.MaxTurns
	}
	if r.MinimumTrustForAgreement <= 0 {
		r.MinimumTrustForAgreement = defaults.MinimumTrustForAgreement
	}
	if r.Proposal.Kind == "" {
		r.Proposal.Kind = defaults.Proposal.Kind
	}
	if r.Proposal.AlternativeIDs == nil {
		r.Proposal.AlternativeIDs = []string{}
	}
	if r.TrustScoreWeight == 0 {
		r.TrustScoreWeight = defaults.TrustScoreWeight
	}
	if r.ArgumentScoreWeight == 0 {
		r.ArgumentScoreWeight = defaults.ArgumentScoreWeight
	}
	if r.PressureScoreWeight == 0 {
		r.PressureScoreWeight = defaults.PressureScoreWeight
	}
	return r
}

type NegotiationPhase string

const (
	PhaseOpening     NegotiationPhase = "opening"
	PhaseExploration NegotiationPhase = "exploration"
	PhaseBargaining  NegotiationPhase = "bargaining"
	PhaseFinished    NegotiationPhase = "finished"
)

func InitialSessionState() SessionState {
	return SessionState{Phase: PhaseOpening}
}

type Session struct {
	ID             string       `json:"id"`
	ScenarioID     string       `json:"scenarioId"`
	Status         string       `json:"status"`
	Turn           int          `json:"turn"`
	TrustScore     int          `json:"trustScore"`
	ArgumentScore  int          `json:"argumentScore"`
	PressureScore  int          `json:"pressureScore"`
	InitialMessage string       `json:"initialMessage,omitempty"`
	StartedAt      time.Time    `json:"startedAt"`
	State          SessionState `json:"state"`
}

type SessionState struct {
	Phase             NegotiationPhase `json:"phase"`
	InterestsExplored bool             `json:"interestsExplored"`
	EvidencePresented bool             `json:"evidencePresented"`
	OfferMade         bool             `json:"offerMade"`
	OfferAccepted     bool             `json:"offerAccepted"`
	LastOfferID       string           `json:"lastOfferId,omitempty"`
}

type Result struct {
	SessionID       string   `json:"sessionId"`
	FinalScore      int      `json:"finalScore"`
	Outcome         string   `json:"outcome"`
	Strengths       []string `json:"strengths"`
	Mistakes        []string `json:"mistakes"`
	Recommendations []string `json:"recommendations"`
}

type Message struct {
	Sender  string `json:"sender"`
	Content string `json:"content"`
}
