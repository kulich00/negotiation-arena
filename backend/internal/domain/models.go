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
	if r.MaxTurns == 0 {
		r.MaxTurns = defaults.MaxTurns
	}
	if r.MinimumTrustForAgreement == 0 {
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

type SPINStage int

const (
	PhaseOpening     NegotiationPhase = "opening"
	PhaseExploration NegotiationPhase = "exploration"
	PhaseBargaining  NegotiationPhase = "bargaining"
	PhaseFinished    NegotiationPhase = "finished"
)

const (
	SPINStageNone SPINStage = iota
	SPINStageSituation
	SPINStageProblem
	SPINStageImplication
	SPINStageNeedPayoff
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
	Phase                 NegotiationPhase `json:"phase"`
	InterestsExplored     bool             `json:"interestsExplored"`
	EvidencePresented     bool             `json:"evidencePresented"`
	SituationExplored     bool             `json:"situationExplored"`
	ProblemIdentified     bool             `json:"problemIdentified"`
	ImplicationsExplored  bool             `json:"implicationsExplored"`
	NeedPayoffEstablished bool             `json:"needPayoffEstablished"`
	SPINStage             SPINStage        `json:"spinStage"`
	BATNADefined          bool             `json:"batnaDefined"`
	OfferMade             bool             `json:"offerMade"`
	OfferAccepted         bool             `json:"offerAccepted"`
	LastOfferID           string           `json:"lastOfferId,omitempty"`
	StructuredMovesUsed   bool             `json:"structuredMovesUsed"`
}

type Result struct {
	SessionID       string          `json:"sessionId"`
	FinalScore      int             `json:"finalScore"`
	Outcome         string          `json:"outcome"`
	Strengths       []string        `json:"strengths"`
	Mistakes        []string        `json:"mistakes"`
	Recommendations []string        `json:"recommendations"`
	Analysis        SessionAnalysis `json:"analysis"`
}

type SessionAnalysis struct {
	AnalyzedTurns           int              `json:"analyzedTurns"`
	Techniques              []TechniqueUsage `json:"techniques"`
	RepeatedRisks           []RepeatedRisk   `json:"repeatedRisks"`
	PriorityRecommendations []string         `json:"priorityRecommendations"`
	BestMove                *BestMoveInsight `json:"bestMove,omitempty"`
}

type TechniqueUsage struct {
	Technique string `json:"technique"`
	Label     string `json:"label"`
	Count     int    `json:"count"`
}

type RepeatedRisk struct {
	Risk  string `json:"risk"`
	Count int    `json:"count"`
}

type BestMoveInsight struct {
	Turn        int    `json:"turn"`
	Technique   string `json:"technique"`
	Label       string `json:"label"`
	Summary     string `json:"summary"`
	ScoreImpact int    `json:"scoreImpact"`
}

type Message struct {
	Sender   string        `json:"sender"`
	Content  string        `json:"content"`
	Analysis *TurnAnalysis `json:"analysis,omitempty"`
}

type TurnAnalysis struct {
	Intent         string   `json:"intent"`
	Technique      string   `json:"technique"`
	TrustDelta     int      `json:"trustDelta"`
	ArgumentDelta  int      `json:"argumentDelta"`
	PressureDelta  int      `json:"pressureDelta"`
	Summary        string   `json:"summary"`
	Strengths      []string `json:"strengths"`
	Risks          []string `json:"risks"`
	Recommendation string   `json:"recommendation"`
}
