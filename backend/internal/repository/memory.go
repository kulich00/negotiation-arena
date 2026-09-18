package repository

import (
	"errors"
	"sync"

	"github.com/kulich00/negotiation-arena/backend/internal/domain"
)

var ErrNotFound = errors.New("not found")

type MemoryRepository struct {
	mu        sync.RWMutex
	scenarios map[string]domain.Scenario
	sessions  map[string]domain.Session
	results   map[string]domain.Result
}

func NewMemoryRepository() *MemoryRepository {
	return &MemoryRepository{
		scenarios: make(map[string]domain.Scenario),
		sessions:  make(map[string]domain.Session),
		results:   make(map[string]domain.Result),
	}
}

func (r *MemoryRepository) ListScenarios() []domain.Scenario {
	r.mu.RLock()
	defer r.mu.RUnlock()
	items := make([]domain.Scenario, 0, len(r.scenarios))
	for _, scenario := range r.scenarios {
		items = append(items, scenario)
	}
	return items
}

func (r *MemoryRepository) SaveScenario(scenario domain.Scenario) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scenarios[scenario.ID] = scenario
}

func (r *MemoryRepository) Scenario(id string) (domain.Scenario, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.scenarios[id]
	if !ok {
		return domain.Scenario{}, ErrNotFound
	}
	return item, nil
}

func (r *MemoryRepository) SaveSession(session domain.Session) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sessions[session.ID] = session
}

func (r *MemoryRepository) Session(id string) (domain.Session, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.sessions[id]
	if !ok {
		return domain.Session{}, ErrNotFound
	}
	return item, nil
}

func (r *MemoryRepository) SaveResult(result domain.Result) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.results[result.SessionID] = result
}

func (r *MemoryRepository) Result(sessionID string) (domain.Result, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	item, ok := r.results[sessionID]
	if !ok {
		return domain.Result{}, ErrNotFound
	}
	return item, nil
}
