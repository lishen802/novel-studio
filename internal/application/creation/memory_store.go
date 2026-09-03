package creation

import (
	"context"
	"sync"

	assistantdomain "github.com/lishen802/novel-studio/internal/domain/assistant"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
)

type memoryStore struct {
	mu         sync.Mutex
	sessions   map[string]creationdomain.Session
	candidates map[string]creationdomain.Candidate
	assistants map[string]assistantdomain.Session
	actions    map[string]assistantdomain.Action
	pending    int
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		sessions:   map[string]creationdomain.Session{},
		candidates: map[string]creationdomain.Candidate{},
		assistants: map[string]assistantdomain.Session{},
		actions:    map[string]assistantdomain.Action{},
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

func (store *memoryStore) Project(context.Context) (projectdomain.Project, error) {
	return projectdomain.Project{ID: "project-1", Title: "长夜织境", Genre: "东方奇幻", Summary: "简介"}, nil
}

func (store *memoryStore) WorkbenchState(context.Context) (projectdomain.WorkbenchState, error) {
	return projectdomain.WorkbenchState{Stage: projectdomain.StageIdea, ArchiveStatus: "not_generated"}, nil
}

func (store *memoryStore) ApplyArchive(context.Context, ArchiveApply) error {
	return ErrUnavailable
}

func assistantSessionKey(projectID, branchID string) string {
	return projectID + "\x00" + branchID
}

func (store *memoryStore) SaveAssistantSession(_ context.Context, session assistantdomain.Session) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.assistants[assistantSessionKey(session.ProjectID, session.BranchID)] = session
	return nil
}

func (store *memoryStore) AssistantSession(_ context.Context, projectID, branchID string) (assistantdomain.Session, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	session, ok := store.assistants[assistantSessionKey(projectID, branchID)]
	if !ok {
		return assistantdomain.Session{}, ErrSessionNotFound
	}
	return session, nil
}

func (store *memoryStore) SaveAction(_ context.Context, action assistantdomain.Action) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.actions[action.ID] = action
	pending := 0
	for _, item := range store.actions {
		if item.State == assistantdomain.StatePendingConfirmation {
			pending++
		}
	}
	store.pending = pending
	return nil
}

func (store *memoryStore) Action(_ context.Context, id string) (assistantdomain.Action, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	action, ok := store.actions[id]
	if !ok {
		return assistantdomain.Action{}, ErrSessionNotFound
	}
	return action, nil
}

func (store *memoryStore) ActionByIdempotency(_ context.Context, sessionID, key string) (assistantdomain.Action, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, action := range store.actions {
		if action.SessionID == sessionID && action.IdempotencyKey == key {
			return action, nil
		}
	}
	return assistantdomain.Action{}, ErrSessionNotFound
}

func (store *memoryStore) ListActions(_ context.Context, sessionID string) ([]assistantdomain.Action, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	result := make([]assistantdomain.Action, 0)
	for _, action := range store.actions {
		if action.SessionID == sessionID {
			result = append(result, action)
		}
	}
	return result, nil
}
