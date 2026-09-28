package llm

import "context"

type AnalysisRequest struct {
	Message       string
	Turn          int
	TrustScore    int
	ArgumentScore int
}

type AnalysisResult struct {
	Reply             string
	TrustDelta        int
	ArgumentDelta     int
	PressureDelta     int
	DetectedStrengths []string
}

type Provider interface {
	Analyze(context.Context, AnalysisRequest) (AnalysisResult, error)
}

// ConversationMessage is a previous public message supplied to a reply
// generator as context. It deliberately contains no scoring data.
type ConversationMessage struct {
	Role string `json:"role"`
	Text string `json:"text"`
}

// ReplyRequest contains the deterministic decision made by the negotiation
// engine and the context needed to phrase it naturally.
type ReplyRequest struct {
	BaseReply     string                `json:"baseReply"`
	PlayerMessage string                `json:"playerMessage"`
	ScenarioTopic string                `json:"scenarioTopic"`
	OpponentRole  string                `json:"opponentRole"`
	OpponentTone  string                `json:"opponentTone"`
	OpponentGoal  string                `json:"opponentGoal"`
	Phase         string                `json:"phase"`
	Mood          string                `json:"mood,omitempty"`
	Priority      string                `json:"priority,omitempty"`
	Turn          int                   `json:"turn"`
	History       []ConversationMessage `json:"history,omitempty"`
}

// ReplyGenerator may improve only the wording of BaseReply. The negotiation
// service remains the source of truth for state, scores and outcomes.
type ReplyGenerator interface {
	GenerateReply(context.Context, ReplyRequest) (string, error)
}

type PassthroughReplyGenerator struct{}

func NewPassthroughReplyGenerator() PassthroughReplyGenerator {
	return PassthroughReplyGenerator{}
}

func (PassthroughReplyGenerator) GenerateReply(_ context.Context, request ReplyRequest) (string, error) {
	return request.BaseReply, nil
}
