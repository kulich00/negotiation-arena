package repository

import (
	"context"
	"errors"
	"sort"
	"sync"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
)

var ErrNotFound = errors.New("not found")
var ErrConflict = errors.New("session changed")

type MemoryRepository struct {
	mu        sync.RWMutex
	scenarios map[string]domain.Scenario
	sessions  map[string]domain.Session
	messages  map[string][]domain.Message
	results   map[string]domain.Result
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		scenarios: make(map[string]domain.Scenario),
		sessions:  make(map[string]domain.Session),
		messages:  make(map[string][]domain.Message),
		results:   make(map[string]domain.Result),
	}
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
	if session.State.Phase == "" {
		session.State = domain.InitialSessionState()
	}
	r.sessions[session.ID] = session
	r.messages[session.ID] = []domain.Message{{Sender: "opponent", Content: session.InitialMessage}}
	return nil
}

func (r *MemoryRepository) Session(_ context.Context, id string) (domain.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.sessions[id]
	if !ok {
		return domain.Session{}, ErrNotFound
	}
	return item, nil
}

func (r *MemoryRepository) ApplyTurn(_ context.Context, session domain.Session, expectedTurn int, message, reply string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.sessions[session.ID]
	if !ok {
		return ErrNotFound
	}
	if current.Status != "active" || current.Turn != expectedTurn {
		return ErrConflict
	}
	r.sessions[session.ID] = session
	r.messages[session.ID] = append(r.messages[session.ID], domain.Message{Sender: "player", Content: message}, domain.Message{Sender: "opponent", Content: reply})
	return nil
}

func (r *MemoryRepository) Messages(_ context.Context, id string) ([]domain.Message, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if _, ok := r.sessions[id]; !ok {
		return nil, ErrNotFound
	}
	return append([]domain.Message{}, r.messages[id]...), nil
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
	r.sessions[session.ID] = session
	r.results[result.SessionID] = result
	return nil
}

func (r *MemoryRepository) Result(_ context.Context, sessionID string) (domain.Result, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.results[sessionID]
	if !ok {
		return domain.Result{}, ErrNotFound
	}
	return item, nil
}

func cloneScenario(s domain.Scenario) domain.Scenario {
	s.Rules.Proposal.AlternativeIDs = append([]string{}, s.Rules.Proposal.AlternativeIDs...)
	return s
}
