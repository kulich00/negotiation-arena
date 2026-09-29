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
	MaxTurns                         int                `json:"maxTurns"`
	MinimumTrustForAgreement         int                `json:"minimumTrustForAgreement"`
	MinimumArgumentScoreForAgreement int                `json:"minimumArgumentScoreForAgreement"`
	MaximumPressureForAgreement      int                `json:"maximumPressureForAgreement"`
	RequiresInterestExploration      bool               `json:"requiresInterestExploration"`
	RequiresEvidence                 bool               `json:"requiresEvidence"`
	Proposal                         ProposalConstraint `json:"proposal"`
	TrustScoreWeight                 float64            `json:"trustScoreWeight"`
	ArgumentScoreWeight              float64            `json:"argumentScoreWeight"`
	PressureScoreWeight              float64            `json:"pressureScoreWeight"`
	Behavior                         OpponentBehavior   `json:"behavior"`
}

type OpponentBehavior struct {
	Mode            string                  `json:"mode"`
	Emotionality    int                     `json:"emotionality"`
	Volatility      int                     `json:"volatility"`
	InitialPriority string                  `json:"initialPriority"`
	PriorityShifts  []OpponentPriorityShift `json:"priorityShifts"`
}

type OpponentPriorityShift struct {
	Turn     int    `json:"turn"`
	Priority string `json:"priority"`
}

// ProposalConstraint separates client input limits from the opponent's hidden
// preferred and reservation values.
type ProposalConstraint struct {
	Kind                    string   `json:"kind"`
	PreferredValue          int      `json:"preferredValue"`
	MaximumValue            int      `json:"maximumValue"`
	InputMaximumValue       int      `json:"inputMaximumValue"`
	AlternativeIDs          []string `json:"alternativeIds"`
	PreferredAlternativeIDs []string `json:"preferredAlternativeIds"`
}

func DefaultScenarioRules() ScenarioRules {
	return ScenarioRules{
		MaxTurns:                         8,
		MinimumTrustForAgreement:         55,
		MinimumArgumentScoreForAgreement: 2,
		MaximumPressureForAgreement:      4,
		RequiresInterestExploration:      true,
		RequiresEvidence:                 true,
		Proposal:                         ProposalConstraint{Kind: "none", AlternativeIDs: []string{}},
		TrustScoreWeight:                 1,
		ArgumentScoreWeight:              1,
		PressureScoreWeight:              1,
		Behavior:                         OpponentBehavior{Mode: "standard", PriorityShifts: []OpponentPriorityShift{}},
	}
}

func (r ScenarioRules) WithDefaults() ScenarioRules {
	defaults := DefaultScenarioRules()
	if r.MaxTurns == 0 && r.MinimumTrustForAgreement == 0 && r.MinimumArgumentScoreForAgreement == 0 && r.MaximumPressureForAgreement == 0 && !r.RequiresInterestExploration && !r.RequiresEvidence && r.Proposal.Kind == "" && r.TrustScoreWeight == 0 && r.ArgumentScoreWeight == 0 && r.PressureScoreWeight == 0 && r.Behavior.Mode == "" {
		return defaults
	}
	if r.MaxTurns == 0 {
		r.MaxTurns = defaults.MaxTurns
	}
	if r.MinimumTrustForAgreement == 0 {
		r.MinimumTrustForAgreement = defaults.MinimumTrustForAgreement
	}
	if r.MinimumArgumentScoreForAgreement == 0 {
		r.MinimumArgumentScoreForAgreement = defaults.MinimumArgumentScoreForAgreement
	}
	if r.MaximumPressureForAgreement == 0 {
		r.MaximumPressureForAgreement = defaults.MaximumPressureForAgreement
	}
	if r.Proposal.Kind == "" {
		r.Proposal.Kind = defaults.Proposal.Kind
	}
	if r.Proposal.AlternativeIDs == nil {
		r.Proposal.AlternativeIDs = []string{}
	}
	if r.Proposal.PreferredAlternativeIDs == nil {
		r.Proposal.PreferredAlternativeIDs = []string{}
	}
	if r.Proposal.Kind != "none" {
		if r.Proposal.PreferredValue == 0 {
			r.Proposal.PreferredValue = r.Proposal.MaximumValue
		}
		if r.Proposal.InputMaximumValue == 0 {
			r.Proposal.InputMaximumValue = r.Proposal.MaximumValue
		}
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
	if r.Behavior.Mode == "" {
		r.Behavior.Mode = defaults.Behavior.Mode
	}
	if r.Behavior.PriorityShifts == nil {
		r.Behavior.PriorityShifts = []OpponentPriorityShift{}
	}
	return r
}

type NegotiationPhase string

type SPINStage int

type OfferQuality string

const (
	SessionStatusActive    = "active"
	SessionStatusFinished  = "finished"
	SessionStatusAbandoned = "abandoned"
)

const (
	PhaseOpening     NegotiationPhase = "opening"
	PhaseExploration NegotiationPhase = "exploration"
	PhaseBargaining  NegotiationPhase = "bargaining"
	PhaseFinished    NegotiationPhase = "finished"
)

const (
	OfferQualityPreferred  OfferQuality = "preferred"
	OfferQualityAcceptable OfferQuality = "acceptable"
	OfferQualityRejected   OfferQuality = "rejected"
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
	ID              string       `json:"id"`
	ScenarioID      string       `json:"scenarioId"`
	PlayerID        string       `json:"playerId,omitempty"`
	ParentSessionID string       `json:"parentSessionId,omitempty"`
	ForkedFromTurn  *int         `json:"forkedFromTurn,omitempty"`
	Status          string       `json:"status"`
	Turn            int          `json:"turn"`
	TrustScore      int          `json:"trustScore"`
	ArgumentScore   int          `json:"argumentScore"`
	PressureScore   int          `json:"pressureScore"`
	InitialMessage  string       `json:"initialMessage,omitempty"`
	StartedAt       time.Time    `json:"startedAt"`
	FinishedAt      *time.Time   `json:"finishedAt,omitempty"`
	State           SessionState `json:"state"`
}

type SessionSummary struct {
	ID              string     `json:"id"`
	ScenarioID      string     `json:"scenarioId"`
	ScenarioTitle   string     `json:"scenarioTitle"`
	PlayerID        string     `json:"playerId,omitempty"`
	ParentSessionID string     `json:"parentSessionId,omitempty"`
	ForkedFromTurn  *int       `json:"forkedFromTurn,omitempty"`
	Status          string     `json:"status"`
	Turn            int        `json:"turn"`
	FinalScore      *int       `json:"finalScore,omitempty"`
	OutcomeCode     string     `json:"outcomeCode,omitempty"`
	StartedAt       time.Time  `json:"startedAt"`
	FinishedAt      *time.Time `json:"finishedAt,omitempty"`
}

type SessionPage struct {
	Items  []SessionSummary `json:"items"`
	Total  int              `json:"total"`
	Limit  int              `json:"limit"`
	Offset int              `json:"offset"`
}

type OutcomeCount struct {
	OutcomeCode string `json:"outcomeCode"`
	Count       int    `json:"count"`
}

type SessionStatistics struct {
	Total             int            `json:"total"`
	Active            int            `json:"active"`
	Finished          int            `json:"finished"`
	Abandoned         int            `json:"abandoned"`
	AverageFinalScore float64        `json:"averageFinalScore"`
	Outcomes          []OutcomeCount `json:"outcomes"`
}

type SessionDetail struct {
	Session     Session          `json:"session"`
	Scenario    Scenario         `json:"scenario"`
	Messages    []Message        `json:"messages"`
	Checkpoints []TurnCheckpoint `json:"checkpoints"`
	Result      *Result          `json:"result,omitempty"`
}

type TurnCheckpoint struct {
	Turn          int          `json:"turn"`
	TrustScore    int          `json:"trustScore"`
	ArgumentScore int          `json:"argumentScore"`
	PressureScore int          `json:"pressureScore"`
	State         SessionState `json:"state"`
	CreatedAt     time.Time    `json:"createdAt"`
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
	LastOfferQuality      OfferQuality     `json:"lastOfferQuality,omitempty"`
	StructuredMovesUsed   bool             `json:"structuredMovesUsed"`
	OpponentMode          string           `json:"opponentMode,omitempty"`
	OpponentMood          string           `json:"opponentMood,omitempty"`
	OpponentPriority      string           `json:"opponentPriority,omitempty"`
	AppliedPriorityShifts int              `json:"appliedPriorityShifts"`
}

type OpponentReaction struct {
	Mood            string `json:"mood"`
	Priority        string `json:"priority,omitempty"`
	PriorityChanged bool   `json:"priorityChanged"`
	TrustDelta      int    `json:"trustDelta"`
}

type Result struct {
	SessionID       string          `json:"sessionId"`
	FinalScore      int             `json:"finalScore"`
	OutcomeCode     string          `json:"outcomeCode"`
	Outcome         string          `json:"outcome"`
	Strengths       []string        `json:"strengths"`
	Mistakes        []string        `json:"mistakes"`
	Recommendations []string        `json:"recommendations"`
	Achievements    []Achievement   `json:"achievements"`
	Analysis        SessionAnalysis `json:"analysis"`
}

type Achievement struct {
	Code        string `json:"code"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type UnlockedAchievement struct {
	Achievement
	UnlockedAt time.Time `json:"unlockedAt"`
}

type PlayerProfile struct {
	ID                 string                `json:"id"`
	DisplayName        string                `json:"displayName"`
	CompletedSessions  int                   `json:"completedSessions"`
	SuccessfulSessions int                   `json:"successfulSessions"`
	CurrentWinStreak   int                   `json:"currentWinStreak"`
	BestWinStreak      int                   `json:"bestWinStreak"`
	UnlockedDifficulty string                `json:"unlockedDifficulty"`
	Achievements       []UnlockedAchievement `json:"achievements"`
	CreatedAt          time.Time             `json:"createdAt"`
	UpdatedAt          time.Time             `json:"updatedAt"`
}

type SessionAnalysis struct {
	AnalyzedTurns           int              `json:"analyzedTurns"`
	Techniques              []TechniqueUsage `json:"techniques"`
	ErrorClasses            []ErrorClass     `json:"errorClasses"`
	RepeatedRisks           []RepeatedRisk   `json:"repeatedRisks"`
	PriorityRecommendations []string         `json:"priorityRecommendations"`
	BestMove                *BestMoveInsight `json:"bestMove,omitempty"`
}

type ErrorSeverity string

const (
	ErrorSeverityLow      ErrorSeverity = "low"
	ErrorSeverityMedium   ErrorSeverity = "medium"
	ErrorSeverityHigh     ErrorSeverity = "high"
	ErrorSeverityCritical ErrorSeverity = "critical"
)

// NegotiationError is a stable, machine-readable classification of a mistake
// detected in one player move.
type NegotiationError struct {
	Code     string        `json:"code"`
	Label    string        `json:"label"`
	Severity ErrorSeverity `json:"severity"`
	Message  string        `json:"message"`
}

// ErrorClass aggregates one error code across a completed session. Turns are
// one-based player move numbers and can be used as retry points in the UI.
type ErrorClass struct {
	Code     string        `json:"code"`
	Label    string        `json:"label"`
	Severity ErrorSeverity `json:"severity"`
	Count    int           `json:"count"`
	Turns    []int         `json:"turns"`
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
	Sender    string        `json:"sender"`
	Content   string        `json:"content"`
	Analysis  *TurnAnalysis `json:"analysis,omitempty"`
	CreatedAt time.Time     `json:"createdAt"`
}

type TurnAnalysis struct {
	Intent               string             `json:"intent"`
	Technique            string             `json:"technique"`
	InterpretationSource string             `json:"interpretationSource,omitempty"`
	ReplySource          string             `json:"replySource,omitempty"`
	TrustDelta           int                `json:"trustDelta"`
	ArgumentDelta        int                `json:"argumentDelta"`
	PressureDelta        int                `json:"pressureDelta"`
	Summary              string             `json:"summary"`
	Strengths            []string           `json:"strengths"`
	Risks                []string           `json:"risks"`
	Errors               []NegotiationError `json:"errors"`
	OpponentReaction     *OpponentReaction  `json:"opponentReaction,omitempty"`
	Recommendation       string             `json:"recommendation"`
}
