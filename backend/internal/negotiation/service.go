package negotiation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
	"github.com/kulich00/negotiation-arena/backend/internal/llm"
	"github.com/kulich00/negotiation-arena/backend/internal/repository"
)

type Service struct {
	repo     *repository.MemoryRepository
	provider llm.Provider
}

type TurnResult struct {
	Reply   string         `json:"reply"`
	Session domain.Session `json:"session"`
}

func NewService(repo *repository.MemoryRepository, provider llm.Provider) *Service {
	return &Service{repo: repo, provider: provider}
}

func (s *Service) SeedDefaults() {
	s.repo.SaveScenario(domain.Scenario{ID: "salary-negotiation", Title: "Переговоры о зарплате", Sphere: "HR", Topic: "Повышение компенсации", Difficulty: "medium", OpponentRole: "Руководитель отдела", OpponentTone: "Сдержанный", PlayerGoal: "Добиться повышения или согласовать план пересмотра", OpponentGoal: "Сохранить сотрудника в рамках бюджета", InitialMessage: "Вы хотели обсудить вашу компенсацию. Я слушаю."})
	s.repo.SaveScenario(domain.Scenario{ID: "project-deadline", Title: "Перенос срока проекта", Sphere: "Project management", Topic: "Согласование нового дедлайна", Difficulty: "hard", OpponentRole: "Заказчик", OpponentTone: "Требовательный", PlayerGoal: "Согласовать реалистичный срок без потери доверия", OpponentGoal: "Получить результат вовремя и снизить риски", InitialMessage: "Срок уже был подтверждён. Почему я должен соглашаться на перенос?"})
}

func (s *Service) ListScenarios() []domain.Scenario { return s.repo.ListScenarios() }

func (s *Service) CreateScenario(scenario domain.Scenario) domain.Scenario {
	if scenario.ID == "" {
		scenario.ID = slug(scenario.Title) + "-" + newID()[:6]
	}
	s.repo.SaveScenario(scenario)
	return scenario
}

func (s *Service) StartSession(scenarioID string) (domain.Session, error) {
	scenario, err := s.repo.Scenario(scenarioID)
	if err != nil {
		return domain.Session{}, err
	}
	session := domain.Session{ID: newID(), ScenarioID: scenarioID, Status: "active", TrustScore: 50, ArgumentScore: 0, PressureScore: 0, InitialMessage: scenario.InitialMessage, StartedAt: time.Now().UTC()}
	s.repo.SaveSession(session)
	return session, nil
}

func (s *Service) Session(id string) (domain.Session, error) { return s.repo.Session(id) }

func (s *Service) ProcessMessage(ctx context.Context, sessionID, message string) (TurnResult, error) {
	session, err := s.repo.Session(sessionID)
	if err != nil {
		return TurnResult{}, err
	}
	if session.Status != "active" {
		return TurnResult{}, errors.New("session is finished")
	}
	analysis, err := s.provider.Analyze(ctx, llm.AnalysisRequest{Message: message, Turn: session.Turn, TrustScore: session.TrustScore, ArgumentScore: session.ArgumentScore})
	if err != nil {
		return TurnResult{}, err
	}
	session.Turn++
	session.TrustScore = clamp(session.TrustScore + analysis.TrustDelta)
	session.ArgumentScore = clamp(session.ArgumentScore + analysis.ArgumentDelta)
	session.PressureScore = clamp(session.PressureScore + analysis.PressureDelta)
	s.repo.SaveSession(session)
	return TurnResult{Reply: analysis.Reply, Session: session}, nil
}

func (s *Service) Finish(sessionID string) (domain.Result, error) {
	session, err := s.repo.Session(sessionID)
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
	s.repo.SaveSession(session)
	s.repo.SaveResult(result)
	return result, nil
}

func (s *Service) Result(sessionID string) (domain.Result, error) { return s.repo.Result(sessionID) }

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
