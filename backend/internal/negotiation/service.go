package negotiation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"strings"
	"time"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
	"github.com/kulich00/negotiation-arena/backend/internal/llm"
	"github.com/kulich00/negotiation-arena/backend/internal/repository"
)

type Service struct {
	repo     repository.Repository
	provider llm.Provider
}

type TurnResult struct {
	Reply   string         `json:"reply"`
	Session domain.Session `json:"session"`
}

func NewService(repo repository.Repository, provider llm.Provider) *Service {
	return &Service{repo: repo, provider: provider}
}

func (s *Service) SeedDefaults(ctx context.Context) error {
	if err := s.repo.SaveScenario(ctx, domain.Scenario{ID: "salary-negotiation", Title: "Переговоры о зарплате", Sphere: "HR", Topic: "Повышение компенсации", Difficulty: "medium", OpponentRole: "Руководитель отдела", OpponentTone: "Сдержанный", PlayerGoal: "Добиться повышения или согласовать план пересмотра", OpponentGoal: "Сохранить сотрудника в рамках бюджета", InitialMessage: "Вы хотели обсудить вашу компенсацию. Я слушаю."}); err != nil {
		return err
	}
	return s.repo.SaveScenario(ctx, domain.Scenario{ID: "project-deadline", Title: "Перенос срока проекта", Sphere: "Project management", Topic: "Согласование нового дедлайна", Difficulty: "hard", OpponentRole: "Заказчик", OpponentTone: "Требовательный", PlayerGoal: "Согласовать реалистичный срок без потери доверия", OpponentGoal: "Получить результат вовремя и снизить риски", InitialMessage: "Срок уже был подтверждён. Почему я должен соглашаться на перенос?"})
}

func (s *Service) ListScenarios(ctx context.Context) ([]domain.Scenario, error) {
	return s.repo.ListScenarios(ctx)
}

func (s *Service) CreateScenario(ctx context.Context, scenario domain.Scenario) (domain.Scenario, error) {
	if scenario.ID == "" {
		scenario.ID = slug(scenario.Title) + "-" + newID()[:6]
	}
	return scenario, s.repo.SaveScenario(ctx, scenario)
}

func (s *Service) StartSession(ctx context.Context, scenarioID string) (domain.Session, error) {
	scenario, err := s.repo.Scenario(ctx, scenarioID)
	if err != nil {
		return domain.Session{}, err
	}
	session := domain.Session{ID: newID(), ScenarioID: scenarioID, Status: "active", TrustScore: 50, ArgumentScore: 0, PressureScore: 0, InitialMessage: scenario.InitialMessage, StartedAt: time.Now().UTC()}
	return session, s.repo.SaveSession(ctx, session)
}

func (s *Service) Session(ctx context.Context, id string) (domain.Session, error) {
	return s.repo.Session(ctx, id)
}
func (s *Service) Messages(ctx context.Context, id string) ([]domain.Message, error) {
	return s.repo.Messages(ctx, id)
}

func (s *Service) ProcessMessage(ctx context.Context, sessionID, message string) (TurnResult, error) {
	session, err := s.repo.Session(ctx, sessionID)
	if err != nil {
		return TurnResult{}, err
	}
	if session.Status != "active" {
		return TurnResult{}, repository.ErrConflict
	}
	analysis, err := s.provider.Analyze(ctx, llm.AnalysisRequest{Message: message, Turn: session.Turn, TrustScore: session.TrustScore, ArgumentScore: session.ArgumentScore})
	if err != nil {
		return TurnResult{}, err
	}
	expectedTurn := session.Turn
	session.Turn++
	session.TrustScore = clamp(session.TrustScore + analysis.TrustDelta)
	session.ArgumentScore = clamp(session.ArgumentScore + analysis.ArgumentDelta)
	session.PressureScore = clamp(session.PressureScore + analysis.PressureDelta)
	if err := s.repo.ApplyTurn(ctx, session, expectedTurn, message, analysis.Reply); err != nil {
		return TurnResult{}, err
	}
	return TurnResult{Reply: analysis.Reply, Session: session}, nil
}

func (s *Service) Finish(ctx context.Context, sessionID string) (domain.Result, error) {
	session, err := s.repo.Session(ctx, sessionID)
	if err != nil {
		return domain.Result{}, err
	}
	score := clamp(session.TrustScore + session.ArgumentScore*5 - session.PressureScore*4)
	result := domain.Result{SessionID: sessionID, FinalScore: score, Outcome: "Переговоры требуют доработки", Strengths: []string{}, Mistakes: []string{}, Recommendations: []string{}}
	if session.ArgumentScore > 2 {
		result.Strengths = append(result.Strengths, "Аргументы опирались на факты")
	} else {
		result.Recommendations = append(result.Recommendations, "Добавьте измеримые результаты и факты")
	}
	if session.TrustScore >= 53 {
		result.Strengths = append(result.Strengths, "Удалось сохранить доверие")
	} else {
		result.Recommendations = append(result.Recommendations, "Задавайте больше открытых вопросов")
	}
	if session.PressureScore > 2 {
		result.Mistakes = append(result.Mistakes, "Избыточное давление")
	}
	if score >= 70 {
		result.Outcome = "Выгодное соглашение"
	} else if score >= 50 {
		result.Outcome = "Компромисс"
	}
	session.Status = "finished"
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
	return strings.Trim(strings.ReplaceAll(strings.ToLower(value), " ", "-"), "-")
}
