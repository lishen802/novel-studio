package creation

import (
	"context"
	"reflect"
	"sync"

	assistantdomain "github.com/lishen802/novel-studio/internal/domain/assistant"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
)

type memoryStore struct {
	mu         sync.Mutex
	sessions   map[string]creationdomain.Session
	candidates map[string]creationdomain.Candidate
	operations map[string]creationdomain.Operation
	assistants map[string]assistantdomain.Session
	actions    map[string]assistantdomain.Action
	pending    int
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		sessions:   map[string]creationdomain.Session{},
		candidates: map[string]creationdomain.Candidate{},
		operations: map[string]creationdomain.Operation{},
		assistants: map[string]assistantdomain.Session{},
		actions:    map[string]assistantdomain.Action{},
	}
}

func operationKey(projectID, branchID, command, key string) string {
	return projectID + "\x00" + branchID + "\x00" + command + "\x00" + key
}

func (store *memoryStore) ReserveOperation(_ context.Context, operation creationdomain.Operation) (creationdomain.Operation, bool, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	key := operationKey(operation.ProjectID, operation.BranchID, operation.Command, operation.IdempotencyKey)
	if existing, ok := store.operations[key]; ok {
		return existing, false, nil
	}
	store.operations[key] = operation
	return operation, true, nil
}

func (store *memoryStore) OperationByIdempotency(_ context.Context, projectID, branchID, command, key string) (creationdomain.Operation, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	operation, ok := store.operations[operationKey(projectID, branchID, command, key)]
	if !ok {
		return creationdomain.Operation{}, ErrOperationNotFound
	}
	return operation, nil
}

func (store *memoryStore) Operation(_ context.Context, id string) (creationdomain.Operation, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, operation := range store.operations {
		if operation.ID == id {
			return operation, nil
		}
	}
	return creationdomain.Operation{}, ErrOperationNotFound
}

func (store *memoryStore) UpdateOperation(_ context.Context, operation creationdomain.Operation) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	key := operationKey(operation.ProjectID, operation.BranchID, operation.Command, operation.IdempotencyKey)
	if _, ok := store.operations[key]; !ok {
		return ErrOperationNotFound
	}
	store.operations[key] = operation
	return nil
}

// CreateSession 与持久化端口一致：重复初始化只返回原会话。
func (store *memoryStore) CreateSession(_ context.Context, session creationdomain.Session) (creationdomain.Session, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	if existing, ok := store.sessions[session.ProjectID]; ok {
		if existing.BranchID != session.BranchID {
			return creationdomain.Session{}, creationdomain.ErrSessionVersionConflict
		}
		return cloneSession(existing), nil
	}
	session.ContentVersion, session.RunSlotVersion = 1, 0
	store.sessions[session.ProjectID] = cloneSession(session)
	return cloneSession(session), nil
}

// SaveSession 仅更新内容域，保持与 SQLite 相同的版本冲突语义。
func (store *memoryStore) SaveSession(_ context.Context, session creationdomain.Session) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	current, ok := store.sessions[session.ProjectID]
	if !ok || current.ID != session.ID || current.BranchID != session.BranchID || current.ContentVersion != session.ContentVersion {
		return creationdomain.ErrSessionVersionConflict
	}
	session.ContentVersion++
	session.RunSlotVersion = current.RunSlotVersion
	session.ActiveOperationID, session.ActiveRunID, session.BlockingReason = current.ActiveOperationID, current.ActiveRunID, current.BlockingReason
	session.CreatedAt = current.CreatedAt
	if session.UpdatedAt.Before(current.UpdatedAt) {
		session.UpdatedAt = current.UpdatedAt
	}
	store.sessions[session.ProjectID] = cloneSession(session)
	return nil
}

// SaveRunSlot 不让运行状态更新覆盖作者字段，也不接受较旧运行槽快照。
func (store *memoryStore) SaveRunSlot(_ context.Context, session creationdomain.Session) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	current, ok := store.sessions[session.ProjectID]
	if !ok || current.ID != session.ID || current.BranchID != session.BranchID || current.RunSlotVersion != session.RunSlotVersion {
		return creationdomain.ErrSessionVersionConflict
	}
	current.ActiveOperationID, current.ActiveRunID, current.BlockingReason = session.ActiveOperationID, session.ActiveRunID, session.BlockingReason
	current.RunSlotVersion++
	if current.UpdatedAt.Before(session.UpdatedAt) {
		current.UpdatedAt = session.UpdatedAt
	}
	store.sessions[session.ProjectID] = current
	return nil
}

// cloneSession 隔离保留卡切片，防止调用方绕过仓储版本检查修改内存状态。
func cloneSession(session creationdomain.Session) creationdomain.Session {
	session.KeptDirectionIDs = append([]string(nil), session.KeptDirectionIDs...)
	return session
}

func (store *memoryStore) Session(_ context.Context, projectID string) (creationdomain.Session, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	session, ok := store.sessions[projectID]
	if !ok {
		return creationdomain.Session{}, ErrSessionNotFound
	}
	return cloneSession(session), nil
}

func (store *memoryStore) SaveCandidate(_ context.Context, candidate creationdomain.Candidate) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, exists := store.candidates[candidate.ID]; exists {
		return creationdomain.ErrCandidateVersionConflict
	}
	if candidate.RunID != "" && candidate.ArtifactID != "" {
		for _, existing := range store.candidates {
			if existing.RunID == candidate.RunID && existing.ArtifactID == candidate.ArtifactID && existing.Kind == candidate.Kind {
				return creationdomain.ErrCandidateResultExists
			}
		}
	}
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

func (store *memoryStore) ApplyCandidateChanges(_ context.Context, expected, updates, inserts []creationdomain.Candidate) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, candidate := range expected {
		current, ok := store.candidates[candidate.ID]
		if !ok || !reflect.DeepEqual(current, candidate) {
			return creationdomain.ErrCandidateVersionConflict
		}
	}
	for _, candidate := range updates {
		if _, ok := store.candidates[candidate.ID]; !ok {
			return creationdomain.ErrCandidateVersionConflict
		}
	}
	for _, candidate := range inserts {
		if _, ok := store.candidates[candidate.ID]; ok {
			return creationdomain.ErrCandidateVersionConflict
		}
		if candidate.RunID != "" && candidate.ArtifactID != "" {
			for _, existing := range store.candidates {
				if existing.RunID == candidate.RunID && existing.ArtifactID == candidate.ArtifactID && existing.Kind == candidate.Kind {
					return creationdomain.ErrCandidateResultExists
				}
			}
		}
	}
	for _, candidate := range updates {
		store.candidates[candidate.ID] = candidate
	}
	for _, candidate := range inserts {
		store.candidates[candidate.ID] = candidate
	}
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
