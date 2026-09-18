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
