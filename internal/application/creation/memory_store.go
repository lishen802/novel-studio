package creation

import (
	"context"
	"sync"

	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

type memoryStore struct {
	mu         sync.Mutex
	sessions   map[string]creationdomain.Session
	candidates map[string]creationdomain.Candidate
	pending    int
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		sessions:   map[string]creationdomain.Session{},
		candidates: map[string]creationdomain.Candidate{},
	}
}

func (store *memoryStore) SaveSession(_ context.Context, session creationdomain.Session) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.sessions[session.ProjectID] = session
	return nil
}

func (store *memoryStore) Session(_ context.Context, projectID string) (creationdomain.Session, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	session, ok := store.sessions[projectID]
	if !ok {
		return creationdomain.Session{}, ErrSessionNotFound
	}
	return session, nil
}

func (store *memoryStore) SaveCandidate(_ context.Context, candidate creationdomain.Candidate) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.candidates[candidate.ID] = candidate
	return nil
}

func (store *memoryStore) UpdateCandidate(_ context.Context, candidate creationdomain.Candidate) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, ok := store.candidates[candidate.ID]; !ok {
		return ErrSessionNotFound
	}
	store.candidates[candidate.ID] = candidate
	return nil
}

func (store *memoryStore) Candidate(_ context.Context, id string) (creationdomain.Candidate, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	candidate, ok := store.candidates[id]
	if !ok {
		return creationdomain.Candidate{}, ErrSessionNotFound
	}
	return candidate, nil
}

func (store *memoryStore) ListCandidates(_ context.Context, sessionID string) ([]creationdomain.Candidate, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	result := make([]creationdomain.Candidate, 0)
	for _, candidate := range store.candidates {
		if candidate.SessionID == sessionID {
			result = append(result, candidate)
		}
	}
	return result, nil
}

func (store *memoryStore) PendingActionCount(_ context.Context, _ string) (int, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	return store.pending, nil
}
