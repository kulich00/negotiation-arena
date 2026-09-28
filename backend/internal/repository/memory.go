package repository

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
)

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("session changed")

type MemoryRepository struct {
	mu            sync.RWMutex
	players       map[string]domain.PlayerProfile
	scenarios     map[string]domain.Scenario
	sessions      map[string]domain.Session
	messages      map[string][]domain.Message
	checkpoints   map[string][]domain.TurnCheckpoint
	results       map[string]domain.Result
	admins        map[string]string
	adminSessions map[string]memoryAdminSession
}

type memoryAdminSession struct {
	email     string
	expiresAt time.Time
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		players:       make(map[string]domain.PlayerProfile),
		scenarios:     make(map[string]domain.Scenario),
		sessions:      make(map[string]domain.Session),
		messages:      make(map[string][]domain.Message),
		checkpoints:   make(map[string][]domain.TurnCheckpoint),
		results:       make(map[string]domain.Result),
		admins:        make(map[string]string),
		adminSessions: make(map[string]memoryAdminSession),
	}
}

func (r *MemoryRepository) CreatePlayer(_ context.Context, player domain.PlayerProfile) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.players[player.ID]; exists {
		return ErrConflict
	}
	r.players[player.ID] = clonePlayer(player)
	return nil
}

func (r *MemoryRepository) Player(_ context.Context, id string) (domain.PlayerProfile, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	player, exists := r.players[id]
	if !exists {
		return domain.PlayerProfile{}, ErrNotFound
	}
	return clonePlayer(player), nil
}

func (r *MemoryRepository) SetAdminPassword(_ context.Context, email, passwordHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.admins[email] = passwordHash
	for tokenHash, session := range r.adminSessions {
		if session.email == email {
			delete(r.adminSessions, tokenHash)
		}
	}
	return nil
}

func (r *MemoryRepository) AdminPasswordHash(_ context.Context, email string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	hash, ok := r.admins[email]
	if !ok {
		return "", ErrNotFound
	}
	return hash, nil
}

func (r *MemoryRepository) SaveAdminSession(_ context.Context, tokenHash, email string, expiresAt time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.admins[email]; !ok {
		return ErrNotFound
	}
	r.adminSessions[tokenHash] = memoryAdminSession{email: email, expiresAt: expiresAt}
	return nil
}

func (r *MemoryRepository) ValidAdminSession(_ context.Context, tokenHash string, now time.Time) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	session, ok := r.adminSessions[tokenHash]
	return ok && now.Before(session.expiresAt), nil
}

func (r *MemoryRepository) DeleteAdminSession(_ context.Context, tokenHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.adminSessions, tokenHash)
	return nil
}

func (r *MemoryRepository) DeleteExpiredAdminSessions(_ context.Context, now time.Time) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for tokenHash, session := range r.adminSessions {
		if !now.Before(session.expiresAt) {
			delete(r.adminSessions, tokenHash)
		}
	}
	return nil
}

func (r *MemoryRepository) ListScenarios(_ context.Context) ([]domain.Scenario, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]domain.Scenario, 0, len(r.scenarios))
	for _, scenario := range r.scenarios {
		items = append(items, cloneScenario(scenario))
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func (r *MemoryRepository) SaveScenario(_ context.Context, scenario domain.Scenario) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	scenario.Rules = scenario.Rules.WithDefaults()
	r.scenarios[scenario.ID] = cloneScenario(scenario)
	return nil
}

func (r *MemoryRepository) CreateScenario(_ context.Context, scenario domain.Scenario) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.scenarios[scenario.ID]; exists {
		return ErrConflict
	}
	scenario.Rules = scenario.Rules.WithDefaults()
	r.scenarios[scenario.ID] = cloneScenario(scenario)
	return nil
}

func (r *MemoryRepository) UpdateScenario(_ context.Context, scenario domain.Scenario) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.scenarios[scenario.ID]; !exists {
		return ErrNotFound
	}
	for _, session := range r.sessions {
		if session.ScenarioID == scenario.ID && session.Status == "active" {
			return ErrConflict
		}
	}
	scenario.Rules = scenario.Rules.WithDefaults()
	r.scenarios[scenario.ID] = cloneScenario(scenario)
	return nil
}

func (r *MemoryRepository) DeleteScenario(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.scenarios[id]; !exists {
		return ErrNotFound
	}
	for _, session := range r.sessions {
		if session.ScenarioID == id {
			return ErrConflict
		}
	}
	delete(r.scenarios, id)
	return nil
}

func (r *MemoryRepository) Scenario(_ context.Context, id string) (domain.Scenario, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.scenarios[id]
	if !ok {
		return domain.Scenario{}, ErrNotFound
	}
	return cloneScenario(item), nil
}

func (r *MemoryRepository) SaveSession(_ context.Context, session domain.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if session.PlayerID != "" {
		if _, exists := r.players[session.PlayerID]; !exists {
			return ErrNotFound
		}
	}
	if session.State.Phase == "" {
		session.State = domain.InitialSessionState()
	}
	r.sessions[session.ID] = cloneSession(session)
	r.messages[session.ID] = []domain.Message{{Sender: "opponent", Content: session.InitialMessage}}
	r.checkpoints[session.ID] = []domain.TurnCheckpoint{checkpointFromSession(session, session.StartedAt)}
	return nil
}

func (r *MemoryRepository) ForkSession(_ context.Context, parentID string, child domain.Session) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.sessions[child.ID]; exists {
		return ErrConflict
	}
	if _, exists := r.sessions[parentID]; !exists {
		return ErrNotFound
	}
	checkpointIndex := -1
	for index, checkpoint := range r.checkpoints[parentID] {
		if checkpoint.Turn == child.Turn {
			checkpointIndex = index
			break
		}
	}
	messageCount := 1 + 2*child.Turn
	if checkpointIndex < 0 || messageCount > len(r.messages[parentID]) {
		return ErrNotFound
	}
	r.sessions[child.ID] = cloneSession(child)
	r.messages[child.ID] = cloneMessages(r.messages[parentID][:messageCount])
	r.checkpoints[child.ID] = append([]domain.TurnCheckpoint{}, r.checkpoints[parentID][:checkpointIndex+1]...)
	last := &r.checkpoints[child.ID][checkpointIndex]
	last.TrustScore = child.TrustScore
	last.ArgumentScore = child.ArgumentScore
	last.PressureScore = child.PressureScore
	last.State = child.State
	return nil
}

func (r *MemoryRepository) Session(_ context.Context, id string) (domain.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.sessions[id]
	if !ok {
		return domain.Session{}, ErrNotFound
	}
	return cloneSession(item), nil
}

func (r *MemoryRepository) ApplyTurn(_ context.Context, session domain.Session, expectedTurn int, message, reply string, analysis domain.TurnAnalysis) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.sessions[session.ID]
	if !ok {
		return ErrNotFound
	}
	if current.Status != "active" || current.Turn != expectedTurn {
		return ErrConflict
	}
	r.sessions[session.ID] = cloneSession(session)
	analysis = cloneTurnAnalysis(analysis)
	r.messages[session.ID] = append(r.messages[session.ID], domain.Message{Sender: "player", Content: message, Analysis: &analysis}, domain.Message{Sender: "opponent", Content: reply})
	r.checkpoints[session.ID] = append(r.checkpoints[session.ID], checkpointFromSession(session, time.Now().UTC()))
	return nil
}

func (r *MemoryRepository) Messages(_ context.Context, id string) ([]domain.Message, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.sessions[id]; !ok {
		return nil, ErrNotFound
	}
	items := make([]domain.Message, len(r.messages[id]))
	for index, message := range r.messages[id] {
		items[index] = message
		if message.Analysis != nil {
			analysis := cloneTurnAnalysis(*message.Analysis)
			items[index].Analysis = &analysis
		}
	}
	return items, nil
}

func (r *MemoryRepository) Checkpoints(_ context.Context, id string) ([]domain.TurnCheckpoint, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.sessions[id]; !ok {
		return nil, ErrNotFound
	}
	return append([]domain.TurnCheckpoint{}, r.checkpoints[id]...), nil
}

func (r *MemoryRepository) Finish(_ context.Context, session domain.Session, result domain.Result) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.sessions[session.ID]
	if !ok {
		return ErrNotFound
	}
	if current.Status != "active" || current.Turn != session.Turn {
		return ErrConflict
	}
	if session.PlayerID != "" {
		player, exists := r.players[session.PlayerID]
		if !exists {
			return ErrNotFound
		}
		scenario, exists := r.scenarios[session.ScenarioID]
		if !exists {
			return ErrNotFound
		}
		updatePlayerProgress(&player, result, session.FinishedAt, scenario.Difficulty)
		r.players[player.ID] = clonePlayer(player)
	}
	r.sessions[session.ID] = cloneSession(session)
	r.results[result.SessionID] = cloneResult(result)
	return nil
}

func (r *MemoryRepository) Result(_ context.Context, sessionID string) (domain.Result, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.results[sessionID]
	if !ok {
		return domain.Result{}, ErrNotFound
	}
	return cloneResult(item), nil
}

func (r *MemoryRepository) ListSessions(_ context.Context, filter SessionFilter) (domain.SessionPage, error) {
	filter = filter.WithDefaults()
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]domain.SessionSummary, 0, len(r.sessions))
	for _, session := range r.sessions {
		if filter.Status != "" && session.Status != filter.Status {
			continue
		}
		if filter.ScenarioID != "" && session.ScenarioID != filter.ScenarioID {
			continue
		}
		summary := domain.SessionSummary{
			ID: session.ID, ScenarioID: session.ScenarioID, Status: session.Status,
			PlayerID:        session.PlayerID,
			ParentSessionID: session.ParentSessionID, ForkedFromTurn: cloneInt(session.ForkedFromTurn),
			Turn: session.Turn, StartedAt: session.StartedAt, FinishedAt: cloneTime(session.FinishedAt),
		}
		if scenario, ok := r.scenarios[session.ScenarioID]; ok {
			summary.ScenarioTitle = scenario.Title
		}
		if result, ok := r.results[session.ID]; ok {
			score := result.FinalScore
			summary.FinalScore = &score
			summary.OutcomeCode = result.OutcomeCode
		}
		items = append(items, summary)
	}
	sort.Slice(items, func(i, j int) bool {
		if !items[i].StartedAt.Equal(items[j].StartedAt) {
			return items[i].StartedAt.After(items[j].StartedAt)
		}
		return items[i].ID > items[j].ID
	})
	total := len(items)
	start := filter.Offset
	if start > total {
		start = total
	}
	end := start + filter.Limit
	if end > total {
		end = total
	}
	pageItems := append([]domain.SessionSummary{}, items[start:end]...)
	return domain.SessionPage{Items: pageItems, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (r *MemoryRepository) SessionStatistics(_ context.Context, scenarioID string) (domain.SessionStatistics, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	statistics := domain.SessionStatistics{Outcomes: []domain.OutcomeCount{}}
	outcomes := make(map[string]int)
	totalScore := 0
	scoredSessions := 0
	for _, session := range r.sessions {
		if scenarioID != "" && session.ScenarioID != scenarioID {
			continue
		}
		statistics.Total++
		switch session.Status {
		case domain.SessionStatusActive:
			statistics.Active++
		case domain.SessionStatusFinished:
			statistics.Finished++
		case domain.SessionStatusAbandoned:
			statistics.Abandoned++
		}
		if result, ok := r.results[session.ID]; ok {
			totalScore += result.FinalScore
			scoredSessions++
			outcomes[result.OutcomeCode]++
		}
	}
	if scoredSessions > 0 {
		statistics.AverageFinalScore = float64(totalScore) / float64(scoredSessions)
	}
	for outcomeCode, count := range outcomes {
		statistics.Outcomes = append(statistics.Outcomes, domain.OutcomeCount{OutcomeCode: outcomeCode, Count: count})
	}
	sort.Slice(statistics.Outcomes, func(i, j int) bool {
		return statistics.Outcomes[i].OutcomeCode < statistics.Outcomes[j].OutcomeCode
	})
	return statistics, nil
}

func cloneScenario(s domain.Scenario) domain.Scenario {
	s.Rules.Proposal.AlternativeIDs = append([]string{}, s.Rules.Proposal.AlternativeIDs...)
	s.Rules.Proposal.PreferredAlternativeIDs = append([]string{}, s.Rules.Proposal.PreferredAlternativeIDs...)
	s.Rules.Behavior.PriorityShifts = append([]domain.OpponentPriorityShift{}, s.Rules.Behavior.PriorityShifts...)
	return s
}

func clonePlayer(player domain.PlayerProfile) domain.PlayerProfile {
	player.Achievements = append([]domain.UnlockedAchievement{}, player.Achievements...)
	return player
}

func cloneSession(session domain.Session) domain.Session {
	session.FinishedAt = cloneTime(session.FinishedAt)
	session.ForkedFromTurn = cloneInt(session.ForkedFromTurn)
	return session
}

func cloneTime(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	cloned := *value
	return &cloned
}

func cloneMessages(messages []domain.Message) []domain.Message {
	items := make([]domain.Message, len(messages))
	for index, message := range messages {
		items[index] = message
		if message.Analysis != nil {
			analysis := cloneTurnAnalysis(*message.Analysis)
			items[index].Analysis = &analysis
		}
	}
	return items
}

func checkpointFromSession(session domain.Session, createdAt time.Time) domain.TurnCheckpoint {
	return domain.TurnCheckpoint{
		Turn: session.Turn, TrustScore: session.TrustScore, ArgumentScore: session.ArgumentScore,
		PressureScore: session.PressureScore, State: session.State, CreatedAt: createdAt,
	}
}

func cloneTurnAnalysis(analysis domain.TurnAnalysis) domain.TurnAnalysis {
	analysis.Strengths = append([]string{}, analysis.Strengths...)
	analysis.Risks = append([]string{}, analysis.Risks...)
	analysis.Errors = append([]domain.NegotiationError{}, analysis.Errors...)
	if analysis.OpponentReaction != nil {
		reaction := *analysis.OpponentReaction
		analysis.OpponentReaction = &reaction
	}
	return analysis
}

func cloneResult(result domain.Result) domain.Result {
	result.Strengths = append([]string{}, result.Strengths...)
	result.Mistakes = append([]string{}, result.Mistakes...)
	result.Recommendations = append([]string{}, result.Recommendations...)
	result.Achievements = append([]domain.Achievement{}, result.Achievements...)
	result.Analysis.Techniques = append([]domain.TechniqueUsage{}, result.Analysis.Techniques...)
	result.Analysis.ErrorClasses = append([]domain.ErrorClass{}, result.Analysis.ErrorClasses...)
	for index := range result.Analysis.ErrorClasses {
		result.Analysis.ErrorClasses[index].Turns = append([]int{}, result.Analysis.ErrorClasses[index].Turns...)
	}
	result.Analysis.RepeatedRisks = append([]domain.RepeatedRisk{}, result.Analysis.RepeatedRisks...)
	result.Analysis.PriorityRecommendations = append([]string{}, result.Analysis.PriorityRecommendations...)
	if result.Analysis.BestMove != nil {
		bestMove := *result.Analysis.BestMove
		result.Analysis.BestMove = &bestMove
	}
	return result
}

func updatePlayerProgress(player *domain.PlayerProfile, result domain.Result, finishedAt *time.Time, completedDifficulty string) {
	now := time.Now().UTC()
	if finishedAt != nil {
		now = *finishedAt
	}
	player.CompletedSessions++
	if successfulOutcome(result.OutcomeCode) {
		player.SuccessfulSessions++
		player.CurrentWinStreak++
		if player.CurrentWinStreak > player.BestWinStreak {
			player.BestWinStreak = player.CurrentWinStreak
		}
	} else {
		player.CurrentWinStreak = 0
	}
	player.UnlockedDifficulty = unlockedDifficulty(
		player.UnlockedDifficulty,
		player.SuccessfulSessions,
		completedDifficulty,
		result.FinalScore,
		successfulOutcome(result.OutcomeCode),
	)
	player.UpdatedAt = now

	existing := make(map[string]bool, len(player.Achievements))
	for _, achievement := range player.Achievements {
		existing[achievement.Code] = true
	}
	for _, achievement := range result.Achievements {
		if existing[achievement.Code] {
			continue
		}
		player.Achievements = append(player.Achievements, domain.UnlockedAchievement{
			Achievement: achievement,
			UnlockedAt:  now,
		})
		existing[achievement.Code] = true
	}
}

func successfulOutcome(code string) bool {
	return code == "mutual_gain" || code == "advantageous_agreement" || code == "compromise"
}

func unlockedDifficulty(current string, successfulSessions int, completedDifficulty string, finalScore int, successful bool) string {
	target := "easy"
	if successfulSessions >= 5 {
		target = "hard"
	} else if successfulSessions >= 2 {
		target = "medium"
	}
	if successful && finalScore >= 100 {
		target = higherDifficulty(target, nextDifficulty(completedDifficulty))
	}
	return higherDifficulty(current, target)
}

func nextDifficulty(completed string) string {
	switch completed {
	case "easy":
		return "medium"
	case "medium", "hard":
		return "hard"
	default:
		return "easy"
	}
}

func higherDifficulty(first, second string) string {
	if difficultyRank(second) > difficultyRank(first) {
		return second
	}
	if difficultyRank(first) == 0 {
		return "easy"
	}
	return first
}

func difficultyRank(difficulty string) int {
	switch difficulty {
	case "easy":
		return 1
	case "medium":
		return 2
	case "hard":
		return 3
	default:
		return 0
	}
}
