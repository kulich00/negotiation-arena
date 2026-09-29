package negotiation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
	"unicode"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
	"github.com/kulich00/negotiation-arena/backend/internal/llm"
	"github.com/kulich00/negotiation-arena/backend/internal/repository"
)

type Service struct {
	repo            repository.Repository
	provider        llm.Provider
	replyGenerator  llm.ReplyGenerator
	moveInterpreter llm.MoveInterpreter
}

type ServiceOption func(*Service)

func WithReplyGenerator(generator llm.ReplyGenerator) ServiceOption {
	return func(service *Service) {
		if generator != nil {
			service.replyGenerator = generator
		}
	}
}

func WithMoveInterpreter(interpreter llm.MoveInterpreter) ServiceOption {
	return func(service *Service) {
		service.moveInterpreter = interpreter
	}
}

type TurnResult struct {
	Reply    string              `json:"reply"`
	Session  domain.Session      `json:"session"`
	Analysis domain.TurnAnalysis `json:"analysis"`
	Result   *domain.Result      `json:"result,omitempty"`
}

func NewService(repo repository.Repository, provider llm.Provider, options ...ServiceOption) *Service {
	service := &Service{
		repo: repo, provider: provider,
		replyGenerator: llm.NewPassthroughReplyGenerator(),
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *Service) SeedDefaults(ctx context.Context) error {
	easyRules := domain.DefaultScenarioRules()
	if err := s.repo.SaveScenario(ctx, domain.Scenario{
		ID: "vendor-introduction", Title: "Первое знакомство с поставщиком", Sphere: "Procurement",
		Topic: "Согласование пробной поставки", Difficulty: DifficultyEasy,
		OpponentRole: "Менеджер поставщика", OpponentTone: "Доброжелательный",
		PlayerGoal: "Согласовать условия пробной поставки", OpponentGoal: "Получить нового постоянного клиента",
		InitialMessage: "Расскажите, какие условия пробной поставки вам подходят.", Rules: easyRules,
	}); err != nil {
		return err
	}
	salaryRules := domain.DefaultScenarioRules()
	salaryRules.Proposal = domain.ProposalConstraint{
		Kind:                    "raise_percent",
		PreferredValue:          6,
		MaximumValue:            10,
		InputMaximumValue:       30,
		AlternativeIDs:          []string{"review_in_3_months"},
		PreferredAlternativeIDs: []string{"review_in_3_months"},
	}
	if err := s.repo.SaveScenario(ctx, domain.Scenario{ID: "salary-negotiation", Title: "Переговоры о зарплате", Sphere: "HR", Topic: "Повышение компенсации", Difficulty: "medium", OpponentRole: "Руководитель отдела", OpponentTone: "Сдержанный", PlayerGoal: "Добиться повышения или согласовать план пересмотра", OpponentGoal: "Сохранить сотрудника в рамках бюджета", InitialMessage: "Вы хотели обсудить вашу компенсацию. Я слушаю.", Rules: salaryRules}); err != nil {
		return err
	}
	deadlineRules := domain.DefaultScenarioRules()
	deadlineRules.MaxTurns = 10
	deadlineRules.MinimumTrustForAgreement = 60
	deadlineRules.MinimumArgumentScoreForAgreement = 3
	deadlineRules.MaximumPressureForAgreement = 2
	deadlineRules.Proposal = domain.ProposalConstraint{
		Kind:                    "extension_days",
		PreferredValue:          7,
		MaximumValue:            14,
		InputMaximumValue:       60,
		AlternativeIDs:          []string{"phased_delivery"},
		PreferredAlternativeIDs: []string{"phased_delivery"},
	}
	deadlineRules.Behavior = domain.OpponentBehavior{
		Mode: OpponentModeDifficult, Emotionality: 2, Volatility: 2,
		InitialPriority: "соблюдение исходного срока",
		PriorityShifts: []domain.OpponentPriorityShift{
			{Turn: 3, Priority: "снижение рисков запуска"},
			{Turn: 6, Priority: "поэтапная поставка результата"},
		},
	}
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
	return s.StartSessionForPlayer(ctx, scenarioID, "")
}

func (s *Service) CreatePlayer(ctx context.Context, displayName string) (domain.PlayerProfile, error) {
	displayName, err := normalizePlayerName(displayName)
	if err != nil {
		return domain.PlayerProfile{}, err
	}
	now := time.Now().UTC()
	player := initialPlayerProfile(newID(), displayName, now)
	return player, s.repo.CreatePlayer(ctx, player)
}

func (s *Service) Player(ctx context.Context, id string) (domain.PlayerProfile, error) {
	return s.repo.Player(ctx, strings.TrimSpace(id))
}

func (s *Service) StartSessionForPlayer(ctx context.Context, scenarioID, playerID string) (domain.Session, error) {
	scenarioID = strings.TrimSpace(scenarioID)
	playerID = strings.TrimSpace(playerID)
	scenario, err := s.repo.Scenario(ctx, scenarioID)
	if err != nil {
		return domain.Session{}, err
	}
	if playerID != "" {
		player, err := s.repo.Player(ctx, playerID)
		if err != nil {
			return domain.Session{}, err
		}
		if !difficultyUnlocked(player.UnlockedDifficulty, scenario.Difficulty) {
			return domain.Session{}, ErrDifficultyLocked
		}
	}
	state := domain.InitialSessionState()
	InitializeOpponentState(&state, scenario.Rules)
	session := domain.Session{ID: newID(), ScenarioID: scenarioID, PlayerID: playerID, Status: domain.SessionStatusActive, TrustScore: 50, ArgumentScore: 0, PressureScore: 0, InitialMessage: scenario.InitialMessage, StartedAt: time.Now().UTC(), State: state}
	return session, s.repo.SaveSession(ctx, session)
}

func (s *Service) Session(ctx context.Context, id string) (domain.Session, error) {
	return s.repo.Session(ctx, id)
}

func (s *Service) ListSessions(ctx context.Context, filter repository.SessionFilter) (domain.SessionPage, error) {
	return s.repo.ListSessions(ctx, filter)
}

func (s *Service) SessionStatistics(ctx context.Context, scenarioID string) (domain.SessionStatistics, error) {
	return s.repo.SessionStatistics(ctx, strings.TrimSpace(scenarioID))
}

func (s *Service) SessionDetail(ctx context.Context, id string) (domain.SessionDetail, error) {
	session, err := s.repo.Session(ctx, id)
	if err != nil {
		return domain.SessionDetail{}, err
	}
	scenario, err := s.repo.Scenario(ctx, session.ScenarioID)
	if err != nil {
		return domain.SessionDetail{}, err
	}
	messages, err := s.repo.Messages(ctx, id)
	if err != nil {
		return domain.SessionDetail{}, err
	}
	checkpoints, err := s.repo.Checkpoints(ctx, id)
	if err != nil {
		return domain.SessionDetail{}, err
	}
	detail := domain.SessionDetail{Session: session, Scenario: scenario, Messages: messages, Checkpoints: checkpoints}
	if session.Status != domain.SessionStatusActive {
		result, err := s.repo.Result(ctx, id)
		if err != nil {
			return domain.SessionDetail{}, err
		}
		detail.Result = &result
	}
	return detail, nil
}
func (s *Service) Messages(ctx context.Context, id string) ([]domain.Message, error) {
	return s.repo.Messages(ctx, id)
}

func (s *Service) Checkpoints(ctx context.Context, id string) ([]domain.TurnCheckpoint, error) {
	return s.repo.Checkpoints(ctx, id)
}

func (s *Service) Achievements() []domain.Achievement {
	return AchievementCatalog()
}

var ErrInvalidFork = errors.New("invalid fork point")

func (s *Service) ForkSession(ctx context.Context, parentID string, turn int) (domain.Session, error) {
	if turn < 0 {
		return domain.Session{}, ErrInvalidFork
	}
	parent, err := s.repo.Session(ctx, parentID)
	if err != nil {
		return domain.Session{}, err
	}
	checkpoints, err := s.repo.Checkpoints(ctx, parentID)
	if err != nil {
		return domain.Session{}, err
	}
	var selected *domain.TurnCheckpoint
	for index := range checkpoints {
		if checkpoints[index].Turn == turn {
			selected = &checkpoints[index]
			break
		}
	}
	if selected == nil {
		return domain.Session{}, ErrInvalidFork
	}
	forkedFromTurn := turn
	child := domain.Session{
		ID: newID(), ScenarioID: parent.ScenarioID, PlayerID: parent.PlayerID, ParentSessionID: parent.ID,
		ForkedFromTurn: &forkedFromTurn, Status: domain.SessionStatusActive, Turn: turn,
		TrustScore: selected.TrustScore, ArgumentScore: selected.ArgumentScore,
		PressureScore: selected.PressureScore, InitialMessage: parent.InitialMessage,
		StartedAt: time.Now().UTC(), State: selected.State,
	}
	child.State.Phase = resumablePhase(child.State)
	if err := s.repo.ForkSession(ctx, parentID, child); err != nil {
		return domain.Session{}, err
	}
	return child, nil
}

func resumablePhase(state domain.SessionState) domain.NegotiationPhase {
	switch {
	case state.OfferMade:
		return domain.PhaseBargaining
	case state.InterestsExplored || state.EvidencePresented || state.SituationExplored || state.ProblemIdentified || state.ImplicationsExplored || state.NeedPayoffEstablished || state.BATNADefined:
		return domain.PhaseExploration
	default:
		return domain.PhaseOpening
	}
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
	if session.Status != domain.SessionStatusActive {
		return TurnResult{}, repository.ErrConflict
	}
	scenario, err := s.repo.Scenario(ctx, session.ScenarioID)
	if err != nil {
		return TurnResult{}, err
	}
	repeatedMessage := s.isRepeatedPlayerMessage(ctx, session.ID, message)
	effectiveMove := move
	interpretationFailed := false
	if move == nil && s.moveInterpreter != nil {
		interpreted, err := s.interpretMove(ctx, message, session, scenario)
		if err != nil {
			interpretationFailed = true
		} else if interpreted != nil {
			effectiveMove = interpreted
		}
	}

	var (
		evaluation MoveEvaluation
		reply      string
		analysis   domain.TurnAnalysis
	)
	if effectiveMove != nil {
		if err := ValidateMove(*effectiveMove, scenario.Rules); err != nil {
			return TurnResult{}, err
		}
		if effectiveMove.Intent == IntentAccept && !session.State.OfferMade {
			return TurnResult{}, fmtInvalidMove("there is no offer to accept")
		}
		if effectiveMove.Intent == IntentAccept && session.State.LastOfferQuality == domain.OfferQualityRejected {
			return TurnResult{}, fmtInvalidMove("the opponent rejected the current offer")
		}
		evaluation = EvaluateMove(*effectiveMove, session.State, scenario.Rules)
		evaluation = ApplyOpponentBehavior(*effectiveMove, session.Turn+1, scenario.Rules, evaluation)
		if repeatedMessage || evaluation.Repeated {
			evaluation = suppressRepeatedRewards(evaluation)
		}
		reply = GenerateOpponentReply(*effectiveMove, session, scenario.Rules, evaluation)
		analysis = AnalyzeStructuredMove(*effectiveMove, session.State, scenario.Rules, evaluation)
	} else {
		providerAnalysis, err := s.provider.Analyze(ctx, llm.AnalysisRequest{Message: message, Turn: session.Turn, TrustScore: session.TrustScore, ArgumentScore: session.ArgumentScore})
		if err != nil {
			return TurnResult{}, err
		}
		reply = providerAnalysis.Reply
		evaluation.TrustDelta = providerAnalysis.TrustDelta
		evaluation.ArgumentDelta = providerAnalysis.ArgumentDelta
		evaluation.PressureDelta = providerAnalysis.PressureDelta
		evaluation.State = session.State
		behaviorMove := PlayerMove{Content: message, Intent: legacyBehaviorIntent(providerAnalysis)}
		evaluation = ApplyOpponentBehavior(behaviorMove, session.Turn+1, scenario.Rules, evaluation)
		reply = difficultOpponentReply(reply, evaluation.OpponentReaction)
		analysis = AnalyzeLegacyMove(providerAnalysis)
		if evaluation.OpponentReaction != nil {
			reaction := *evaluation.OpponentReaction
			analysis.OpponentReaction = &reaction
		}
	}
	if !interpretationFailed {
		reply = s.generateReply(ctx, reply, message, session, scenario, evaluation.State)
	}
	expectedTurn := session.Turn
	session.Turn++
	session.TrustScore = clamp(session.TrustScore + evaluation.TrustDelta)
	session.ArgumentScore = clamp(session.ArgumentScore + evaluation.ArgumentDelta)
	session.PressureScore = clamp(session.PressureScore + evaluation.PressureDelta)
	if effectiveMove != nil || evaluation.OpponentReaction != nil {
		session.State = evaluation.State
	}
	if err := s.repo.ApplyTurn(ctx, session, expectedTurn, message, reply, analysis); err != nil {
		return TurnResult{}, err
	}
	turnResult := TurnResult{Reply: reply, Session: session, Analysis: analysis}
	if session.State.OfferAccepted {
		finishedSession, result, err := s.finalize(ctx, session, scenario, domain.SessionStatusFinished)
		if err != nil {
			return TurnResult{}, err
		}
		turnResult.Session = finishedSession
		turnResult.Result = &result
	}
	return turnResult, nil
}

func (s *Service) interpretMove(ctx context.Context, message string, session domain.Session, scenario domain.Scenario) (*PlayerMove, error) {
	rules := scenario.Rules.WithDefaults()
	interpretation, err := s.moveInterpreter.InterpretMove(ctx, llm.InterpretationRequest{
		Message: message, ScenarioTopic: scenario.Topic, PlayerGoal: scenario.PlayerGoal,
		OpponentRole: scenario.OpponentRole, Phase: string(session.State.Phase),
		OfferMade: session.State.OfferMade, ProposalKind: rules.Proposal.Kind,
		ProposalMaximum:      rules.Proposal.InputMaximumValue,
		ProposalAlternatives: append([]string{}, rules.Proposal.AlternativeIDs...),
		ConversationHistory:  s.recentConversation(ctx, session.ID),
	})
	if err != nil {
		return nil, err
	}
	if interpretation.Relevant != nil && !*interpretation.Relevant {
		return &PlayerMove{Content: message, Intent: IntentNeutral}, nil
	}
	move := PlayerMove{Content: message, Intent: MoveIntent(interpretation.Intent)}
	if move.Intent == IntentPropose && rules.Proposal.Kind != "none" {
		switch {
		case interpretation.AlternativeID != "":
			move.Proposal = &MoveProposal{AlternativeID: interpretation.AlternativeID}
		case interpretation.ProposalValue > 0:
			move.Proposal = &MoveProposal{Kind: rules.Proposal.Kind, Value: interpretation.ProposalValue}
		}
	}
	if err := ValidateMove(move, rules); err != nil {
		return &PlayerMove{Content: message, Intent: IntentNeutral}, nil
	}
	if move.Intent == IntentAccept && (!session.State.OfferMade || session.State.LastOfferQuality == domain.OfferQualityRejected) {
		return &PlayerMove{Content: message, Intent: IntentNeutral}, nil
	}
	return &move, nil
}

func (s *Service) isRepeatedPlayerMessage(ctx context.Context, sessionID, message string) bool {
	normalized := normalizeMessageForComparison(message)
	if normalized == "" {
		return false
	}
	messages, err := s.repo.Messages(ctx, sessionID)
	if err != nil {
		return false
	}
	for index := len(messages) - 1; index >= 0; index-- {
		if messages[index].Sender == "player" && normalizeMessageForComparison(messages[index].Content) == normalized {
			return true
		}
	}
	return false
}

func normalizeMessageForComparison(value string) string {
	var normalized strings.Builder
	spacePending := false
	for _, char := range strings.ToLower(strings.TrimSpace(value)) {
		switch {
		case unicode.IsLetter(char) || unicode.IsDigit(char):
			if spacePending && normalized.Len() > 0 {
				normalized.WriteByte(' ')
			}
			normalized.WriteRune(char)
			spacePending = false
		case unicode.IsSpace(char) || unicode.IsPunct(char):
			spacePending = true
		}
	}
	return normalized.String()
}

func (s *Service) generateReply(ctx context.Context, baseReply, playerMessage string, session domain.Session, scenario domain.Scenario, state domain.SessionState) string {
	history := s.recentConversation(ctx, session.ID)
	reply, err := s.replyGenerator.GenerateReply(ctx, llm.ReplyRequest{
		BaseReply: baseReply, PlayerMessage: playerMessage,
		ScenarioTopic: scenario.Topic, OpponentRole: scenario.OpponentRole,
		OpponentTone: scenario.OpponentTone, OpponentGoal: scenario.OpponentGoal,
		Phase: string(state.Phase), Mood: state.OpponentMood,
		Priority: state.OpponentPriority, Turn: session.Turn + 1, History: history,
	})
	if err != nil || strings.TrimSpace(reply) == "" {
		return baseReply
	}
	return strings.TrimSpace(reply)
}

func (s *Service) recentConversation(ctx context.Context, sessionID string) []llm.ConversationMessage {
	history := make([]llm.ConversationMessage, 0, 8)
	if messages, err := s.repo.Messages(ctx, sessionID); err == nil {
		start := len(messages) - 8
		if start < 0 {
			start = 0
		}
		for _, message := range messages[start:] {
			history = append(history, llm.ConversationMessage{Role: message.Sender, Text: message.Content})
		}
	}
	return history
}

func legacyBehaviorIntent(result llm.AnalysisResult) MoveIntent {
	switch {
	case result.PressureDelta > 0:
		return IntentPressure
	case result.TrustDelta > 0:
		return IntentAskInterest
	default:
		return ""
	}
}

func (s *Service) Finish(ctx context.Context, sessionID string) (domain.Result, error) {
	session, err := s.repo.Session(ctx, sessionID)
	if err != nil {
		return domain.Result{}, err
	}
	if session.Status != domain.SessionStatusActive {
		return s.repo.Result(ctx, sessionID)
	}
	scenario, err := s.repo.Scenario(ctx, session.ScenarioID)
	if err != nil {
		return domain.Result{}, err
	}
	_, result, err := s.finalize(ctx, session, scenario, domain.SessionStatusFinished)
	return result, err
}

func (s *Service) Abandon(ctx context.Context, sessionID string) (domain.Result, error) {
	session, err := s.repo.Session(ctx, sessionID)
	if err != nil {
		return domain.Result{}, err
	}
	if session.Status != domain.SessionStatusActive {
		return s.repo.Result(ctx, sessionID)
	}
	scenario, err := s.repo.Scenario(ctx, session.ScenarioID)
	if err != nil {
		return domain.Result{}, err
	}
	_, result, err := s.finalize(ctx, session, scenario, domain.SessionStatusAbandoned)
	return result, err
}

func (s *Service) finalize(ctx context.Context, session domain.Session, scenario domain.Scenario, status string) (domain.Session, domain.Result, error) {
	messages, err := s.repo.Messages(ctx, session.ID)
	if err != nil {
		return domain.Session{}, domain.Result{}, err
	}
	result := EvaluateResult(session, scenario.Rules)
	result.Analysis = AggregateSessionAnalysis(messages)
	EnrichResultWithSessionAnalysis(&result)
	if status == domain.SessionStatusAbandoned {
		result.OutcomeCode = "abandoned"
		result.Outcome = "Переговоры прерваны игроком"
		result.Recommendations = appendUnique(result.Recommendations, "Перед новой попыткой определите цель, BATNA и последовательность аргументов")
	}
	result.Achievements = EvaluateAchievements(session, result)
	session.Status = status
	session.State.Phase = domain.PhaseFinished
	finishedAt := time.Now().UTC()
	session.FinishedAt = &finishedAt
	if err := s.repo.Finish(ctx, session, result); err != nil {
		if errors.Is(err, repository.ErrConflict) {
			stored, resultErr := s.repo.Result(ctx, session.ID)
			if resultErr == nil {
				storedSession, sessionErr := s.repo.Session(ctx, session.ID)
				if sessionErr == nil {
					session = storedSession
				}
				return session, stored, nil
			}
		}
		return domain.Session{}, domain.Result{}, err
	}
	return session, result, nil
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
