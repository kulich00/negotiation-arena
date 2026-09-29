package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

const maxArenaModelResponseBytes = 64 << 10

type ArenaModelConfig struct {
	BaseURL       string
	Client        *http.Client
	MinConfidence float64
	Fallback      MoveInterpreter
	Logger        *slog.Logger
	Observer      Observer
}

type ArenaModelInterpreter struct {
	endpoint      string
	client        *http.Client
	minConfidence float64
	fallback      MoveInterpreter
	logger        *slog.Logger
	observer      Observer
}

type arenaModelResponse struct {
	Intent        string  `json:"intent"`
	ProposalValue int     `json:"proposalValue"`
	AlternativeID string  `json:"alternativeId"`
	Relevant      bool    `json:"relevant"`
	Confidence    float64 `json:"confidence"`
	ModelVersion  string  `json:"modelVersion"`
}

func NewArenaModelInterpreter(config ArenaModelConfig) (*ArenaModelInterpreter, error) {
	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	parsed, err := url.Parse(baseURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" || parsed.User != nil {
		return nil, errors.New("invalid Arena model URL")
	}
	if config.Client == nil {
		config.Client = http.DefaultClient
	}
	if math.IsNaN(config.MinConfidence) || math.IsInf(config.MinConfidence, 0) {
		return nil, errors.New("Arena model minimum confidence must be finite")
	}
	if config.MinConfidence <= 0 {
		config.MinConfidence = 0.55
	}
	if config.MinConfidence > 1 {
		return nil, errors.New("Arena model minimum confidence must not exceed 1")
	}
	if config.Fallback == nil {
		config.Fallback = RuleBasedMoveInterpreter{}
	}
	return &ArenaModelInterpreter{
		endpoint: baseURL + "/v1/interpret", client: config.Client,
		minConfidence: config.MinConfidence, fallback: config.Fallback,
		logger: config.Logger, observer: config.Observer,
	}, nil
}

func (interpreter *ArenaModelInterpreter) InterpretMove(ctx context.Context, request InterpretationRequest) (MoveInterpretation, error) {
	local, localErr := (RuleBasedMoveInterpreter{}).InterpretMove(ctx, request)
	if localErr == nil && local.Intent != "neutral" {
		if interpreter.observer != nil {
			interpreter.observer.ObserveLLM("arena_model_interpretation", "local")
		}
		return local, nil
	}
	result, confidence, err := interpreter.interpretRemote(ctx, request)
	if err == nil && confidence >= interpreter.minConfidence {
		if interpreter.observer != nil {
			interpreter.observer.ObserveLLM("arena_model_interpretation", "success")
		}
		return result, nil
	}
	if interpreter.observer != nil {
		interpreter.observer.ObserveLLM("arena_model_interpretation", "fallback")
	}
	if err != nil && interpreter.logger != nil {
		interpreter.logger.WarnContext(ctx, "Arena model interpretation failed; using rules", "error", err)
	}
	return interpreter.fallback.InterpretMove(ctx, request)
}

func (interpreter *ArenaModelInterpreter) interpretRemote(ctx context.Context, request InterpretationRequest) (MoveInterpretation, float64, error) {
	body, err := json.Marshal(request)
	if err != nil {
		return MoveInterpretation{}, 0, fmt.Errorf("encode Arena model request: %w", err)
	}
	httpRequest, err := http.NewRequestWithContext(ctx, http.MethodPost, interpreter.endpoint, bytes.NewReader(body))
	if err != nil {
		return MoveInterpretation{}, 0, fmt.Errorf("create Arena model request: %w", err)
	}
	httpRequest.Header.Set("Content-Type", "application/json")
	response, err := interpreter.client.Do(httpRequest)
	if err != nil {
		return MoveInterpretation{}, 0, fmt.Errorf("call Arena model: %w", err)
	}
	defer response.Body.Close()
	responseBody, err := io.ReadAll(io.LimitReader(response.Body, maxArenaModelResponseBytes+1))
	if err != nil {
		return MoveInterpretation{}, 0, fmt.Errorf("read Arena model response: %w", err)
	}
	if len(responseBody) > maxArenaModelResponseBytes {
		return MoveInterpretation{}, 0, errors.New("Arena model response is too large")
	}
	if response.StatusCode != http.StatusOK {
		return MoveInterpretation{}, 0, fmt.Errorf("Arena model returned HTTP %d", response.StatusCode)
	}
	var decoded arenaModelResponse
	decoder := json.NewDecoder(bytes.NewReader(responseBody))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&decoded); err != nil {
		return MoveInterpretation{}, 0, fmt.Errorf("decode Arena model response: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return MoveInterpretation{}, 0, errors.New("Arena model returned trailing JSON data")
	}
	decoded.Intent = strings.TrimSpace(decoded.Intent)
	decoded.AlternativeID = strings.TrimSpace(decoded.AlternativeID)
	if _, allowed := geminiMoveIntents[decoded.Intent]; !allowed {
		return MoveInterpretation{}, 0, fmt.Errorf("Arena model returned unsupported intent %q", decoded.Intent)
	}
	if decoded.Confidence < 0 || decoded.Confidence > 1 || decoded.ProposalValue < 0 {
		return MoveInterpretation{}, 0, errors.New("Arena model returned invalid numeric values")
	}
	if decoded.ProposalValue > 0 && request.ProposalMaximum > 0 && decoded.ProposalValue > request.ProposalMaximum {
		return MoveInterpretation{}, 0, errors.New("Arena model proposal exceeds input limit")
	}
	if decoded.AlternativeID != "" && !slices.Contains(request.ProposalAlternatives, decoded.AlternativeID) {
		return MoveInterpretation{}, 0, errors.New("Arena model returned an unknown alternative")
	}
	relevant := decoded.Relevant
	return MoveInterpretation{
		Intent: decoded.Intent, ProposalValue: decoded.ProposalValue,
		AlternativeID: decoded.AlternativeID, Relevant: &relevant, Source: "arena_model",
	}, decoded.Confidence, nil
}
