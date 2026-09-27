package negotiation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"
	"unicode"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
	"github.com/kulich00/negotiation-arena/backend/internal/llm"
	"github.com/kulich00/negotiation-arena/backend/internal/repository"
)

type Service struct {
	repo     repository.Repository
	provider llm.Provider
}

type TurnResult struct {
	Reply    string              `json:"reply"`
	Session  domain.Session      `json:"session"`
	Analysis domain.TurnAnalysis `json:"analysis"`
}

func NewService(repo repository.Repository, provider llm.Provider) *Service {
	return &Service{repo: repo, provider: provider}
}

func (s *Service) SeedDefaults(ctx context.Context) error {
	salaryRules := domain.DefaultScenarioRules()
	salaryRules.Proposal = domain.ProposalConstraint{Kind: "raise_percent", MaximumValue: 10, AlternativeIDs: []string{"review_in_3_months"}}
	if err := s.repo.SaveScenario(ctx, domain.Scenario{ID: "salary-negotiation", Title: "Переговоры о зарплате", Sphere: "HR", Topic: "Повышение компенсации", Difficulty: "medium", OpponentRole: "Руководитель отдела", OpponentTone: "Сдержанный", PlayerGoal: "Добиться повышения или согласовать план пересмотра", OpponentGoal: "Сохранить сотрудника в рамках бюджета", InitialMessage: "Вы хотели обсудить вашу компенсацию. Я слушаю.", Rules: salaryRules}); err != nil {
		return err
	}
	deadlineRules := domain.DefaultScenarioRules()
	deadlineRules.MaxTurns = 10
	deadlineRules.MinimumTrustForAgreement = 60
	deadlineRules.Proposal = domain.ProposalConstraint{Kind: "extension_days", MaximumValue: 14, AlternativeIDs: []string{"phased_delivery"}}
	return s.repo.SaveScenario(ctx, domain.Scenario{ID: "project-deadline", Title: "Перенос срока проекта", Sphere: "Project management", Topic: "Согласование нового дедлайна", Difficulty: "hard", OpponentRole: "Заказчик", OpponentTone: "Требовательный", PlayerGoal: "Согласовать реалистичный срок без потери доверия", OpponentGoal: "Получить результат вовремя и снизить риски", InitialMessage: "Срок уже был подтверждён. Почему я должен соглашаться на перенос?", Rules: deadlineRules})
}

func (s *Service) ListScenarios(ctx context.Context) ([]domain.Scenario, error) {
	return s.repo.ListScenarios(ctx)
}

func (s *Service) CreateScenario(ctx context.Context, scenario domain.Scenario) (domain.Scenario, error) {
	scenario = normalizeScenario(scenario)
	if scenario.ID == "" {
		scenario.ID = slug(scenario.Title) + "-" + newID()[:6]
	}
	scenario.Rules = scenario.Rules.WithDefaults()
	if err := ValidateScenario(scenario); err != nil {
		return domain.Scenario{}, err
	}
	return scenario, s.repo.CreateScenario(ctx, scenario)
}

func (s *Service) UpdateScenario(ctx context.Context, id string, scenario domain.Scenario) (domain.Scenario, error) {
	id = strings.TrimSpace(id)
	scenario = normalizeScenario(scenario)
	if scenario.ID != "" && scenario.ID != id {
		return domain.Scenario{}, invalidScenario("id must match the request path")
	}
	scenario.ID = id
	scenario.Rules = scenario.Rules.WithDefaults()
	if err := ValidateScenario(scenario); err != nil {
		return domain.Scenario{}, err
	}
	return scenario, s.repo.UpdateScenario(ctx, scenario)
}

func (s *Service) DeleteScenario(ctx context.Context, id string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return invalidScenario("id is required")
	}
	return s.repo.DeleteScenario(ctx, id)
}

func (s *Service) StartSession(ctx context.Context, scenarioID string) (domain.Session, error) {
	scenario, err := s.repo.Scenario(ctx, scenarioID)
	if err != nil {
		return domain.Session{}, err
	}
	session := domain.Session{ID: newID(), ScenarioID: scenarioID, Status: "active", TrustScore: 50, ArgumentScore: 0, PressureScore: 0, InitialMessage: scenario.InitialMessage, StartedAt: time.Now().UTC(), State: domain.InitialSessionState()}
	return session, s.repo.SaveSession(ctx, session)
}

func (s *Service) Session(ctx context.Context, id string) (domain.Session, error) {
	return s.repo.Session(ctx, id)
}
func (s *Service) Messages(ctx context.Context, id string) ([]domain.Message, error) {
	return s.repo.Messages(ctx, id)
}

func (s *Service) ProcessMessage(ctx context.Context, sessionID, message string) (TurnResult, error) {
	message = strings.TrimSpace(message)
	if err := validateMoveContent(message); err != nil {
		return TurnResult{}, err
	}
	return s.processTurn(ctx, sessionID, message, nil)
}

func (s *Service) ProcessMove(ctx context.Context, sessionID string, move PlayerMove) (TurnResult, error) {
	move.Content = strings.TrimSpace(move.Content)
	return s.processTurn(ctx, sessionID, move.Content, &move)
}

func (s *Service) processTurn(ctx context.Context, sessionID, message string, move *PlayerMove) (TurnResult, error) {
	session, err := s.repo.Session(ctx, sessionID)
	if err != nil {
		return TurnResult{}, err
	}
	if session.Status != "active" {
		return TurnResult{}, repository.ErrConflict
	}
	scenario, err := s.repo.Scenario(ctx, session.ScenarioID)
	if err != nil {
		return TurnResult{}, err
	}
	if session.Turn >= scenario.Rules.MaxTurns {
		return TurnResult{}, ErrTurnLimitReached
	}

	var (
		evaluation MoveEvaluation
		reply      string
		analysis   domain.TurnAnalysis
	)
	if move != nil {
		if err := ValidateMove(*move, scenario.Rules); err != nil {
			return TurnResult{}, err
		}
		if move.Intent == IntentAccept && !session.State.OfferMade {
			return TurnResult{}, fmtInvalidMove("there is no offer to accept")
		}
		evaluation = EvaluateMove(*move, session.State)
		reply = GenerateOpponentReply(*move, session, scenario.Rules, evaluation)
		analysis = AnalyzeStructuredMove(*move, session.State, scenario.Rules, evaluation)
	} else {
		providerAnalysis, err := s.provider.Analyze(ctx, llm.AnalysisRequest{Message: message, Turn: session.Turn, TrustScore: session.TrustScore, ArgumentScore: session.ArgumentScore})
		if err != nil {
			return TurnResult{}, err
		}
		reply = providerAnalysis.Reply
		evaluation.TrustDelta = providerAnalysis.TrustDelta
		evaluation.ArgumentDelta = providerAnalysis.ArgumentDelta
		evaluation.PressureDelta = providerAnalysis.PressureDelta
		analysis = AnalyzeLegacyMove(providerAnalysis)
	}
	expectedTurn := session.Turn
	session.Turn++
	session.TrustScore = clamp(session.TrustScore + evaluation.TrustDelta)
	session.ArgumentScore = clamp(session.ArgumentScore + evaluation.ArgumentDelta)
	session.PressureScore = clamp(session.PressureScore + evaluation.PressureDelta)
	if move != nil {
		session.State = evaluation.State
	}
	if err := s.repo.ApplyTurn(ctx, session, expectedTurn, message, reply, analysis); err != nil {
		return TurnResult{}, err
	}
	return TurnResult{Reply: reply, Session: session, Analysis: analysis}, nil
}

func (s *Service) Finish(ctx context.Context, sessionID string) (domain.Result, error) {
	session, err := s.repo.Session(ctx, sessionID)
	if err != nil {
		return domain.Result{}, err
	}
	if session.Status != "active" {
		return domain.Result{}, repository.ErrConflict
	}
	scenario, err := s.repo.Scenario(ctx, session.ScenarioID)
	if err != nil {
		return domain.Result{}, err
	}
	messages, err := s.repo.Messages(ctx, sessionID)
	if err != nil {
		return domain.Result{}, err
	}
	result := EvaluateResult(session, scenario.Rules)
	result.Analysis = AggregateSessionAnalysis(messages)
	EnrichResultWithSessionAnalysis(&result)
	session.Status = "finished"
	session.State.Phase = domain.PhaseFinished
	if err := s.repo.Finish(ctx, session, result); err != nil {
		return domain.Result{}, err
	}
	return result, nil
}

func (s *Service) Result(ctx context.Context, sessionID string) (domain.Result, error) {
	return s.repo.Result(ctx, sessionID)
}

func newID() string {
	data := make([]byte, 16)
	_, _ = rand.Read(data)
	return hex.EncodeToString(data)
}
func clamp(value int) int {
	if value < 0 {
		return 0
	}
	if value > 100 {
		return 100
	}
	return value
}
func slug(value string) string {
	var result strings.Builder
	separator := false
	for _, char := range strings.ToLower(value) {
		if unicode.IsLetter(char) || unicode.IsDigit(char) {
			result.WriteRune(char)
			separator = false
		} else if result.Len() > 0 && !separator {
			result.WriteByte('-')
			separator = true
		}
	}
	value = strings.Trim(result.String(), "-")
	if value == "" {
		return "scenario"
	}
	return value
}
