package creation

import (
	"context"
	"database/sql"
	"reflect"
	"sync"

	assistantdomain "github.com/lishen802/novel-studio/internal/domain/assistant"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
)

type memoryStore struct {
	samples        map[string]creationdomain.SampleHandoff
	entries        map[string]creationdomain.EntryRevision
	entryHeads     map[string]string
	mu             sync.Mutex
	sessions       map[string]creationdomain.Session
	candidates     map[string]creationdomain.Candidate
	reviews        map[string]creationdomain.CandidateReview
	cursors        map[string]creationdomain.ReviewCursor
	operations     map[string]creationdomain.Operation
	assistants     map[string]assistantdomain.Session
	messages       []assistantdomain.Message
	authorizations map[string]assistantdomain.Authorization
	actions        map[string]assistantdomain.Action
	pending        int
}

func newMemoryStore() *memoryStore {
	return &memoryStore{
		entries: map[string]creationdomain.EntryRevision{}, entryHeads: map[string]string{},
		sessions:       map[string]creationdomain.Session{},
		candidates:     map[string]creationdomain.Candidate{},
		reviews:        map[string]creationdomain.CandidateReview{},
		cursors:        map[string]creationdomain.ReviewCursor{},
		operations:     map[string]creationdomain.Operation{},
		assistants:     map[string]assistantdomain.Session{},
		messages:       []assistantdomain.Message{},
		authorizations: map[string]assistantdomain.Authorization{},
		actions:        map[string]assistantdomain.Action{},
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

func (store *memoryStore) ApplyCandidateChanges(_ context.Context, expected, updates, inserts []creationdomain.Candidate, guards ...creationdomain.CandidateCommitGuard) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, guard := range guards {
		found := false
		for _, session := range store.sessions {
			if session.ID == guard.SessionID && session.ContentVersion == guard.ContentVersion {
				found = true
			}
		}
		project, _ := store.Project(context.Background())
		if !found || project.ID != guard.ProjectID || !project.UpdatedAt.Equal(guard.ProjectUpdatedAt) {
			return creationdomain.ErrSessionVersionConflict
		}
	}
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

func (store *memoryStore) SaveAssistantMessage(_ context.Context, message assistantdomain.Message) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, existing := range store.messages {
		if existing.SessionID == message.SessionID && existing.IdempotencyKey == message.IdempotencyKey {
			return nil
		}
	}
	store.messages = append(store.messages, message)
	return nil
}

func (store *memoryStore) UpdateAssistantMessage(_ context.Context, message assistantdomain.Message) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	for i, existing := range store.messages {
		if existing.ID == message.ID {
			store.messages[i] = message
			return nil
		}
	}
	return ErrSessionNotFound
}

func (store *memoryStore) ListAssistantMessages(_ context.Context, sessionID string) ([]assistantdomain.Message, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	result := make([]assistantdomain.Message, 0)
	for _, message := range store.messages {
		if message.SessionID == sessionID {
			result = append(result, message)
		}
	}
	return result, nil
}

func (store *memoryStore) AssistantMessageByIdempotency(_ context.Context, sessionID, key string) (assistantdomain.Message, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, message := range store.messages {
		if message.SessionID == sessionID && message.IdempotencyKey == key {
			return message, nil
		}
	}
	return assistantdomain.Message{}, ErrSessionNotFound
}

func (store *memoryStore) SaveAuthorization(_ context.Context, auth assistantdomain.Authorization) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.authorizations == nil {
		store.authorizations = map[string]assistantdomain.Authorization{}
	}
	store.authorizations[auth.ID] = auth
	return nil
}

func (store *memoryStore) Authorization(_ context.Context, id string) (assistantdomain.Authorization, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	auth, ok := store.authorizations[id]
	if !ok {
		return assistantdomain.Authorization{}, ErrSessionNotFound
	}
	return auth, nil
}

func (store *memoryStore) AuthorizationByAction(_ context.Context, actionID string) (assistantdomain.Authorization, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	for _, auth := range store.authorizations {
		if auth.ActionID == actionID {
			return auth, nil
		}
	}
	return assistantdomain.Authorization{}, ErrSessionNotFound
}

func (store *memoryStore) ListAuthorizations(_ context.Context, sessionID string) ([]assistantdomain.Authorization, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	result := make([]assistantdomain.Authorization, 0)
	for _, auth := range store.authorizations {
		if auth.SessionID == sessionID {
			result = append(result, auth)
		}
	}
	return result, nil
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

func (store *memoryStore) CurrentEntry(_ context.Context, id string) (creationdomain.EntryRevision, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	entry, ok := store.entries[store.entryHeads[id]]
	if !ok {
		return entry, sql.ErrNoRows
	}
	return entry, nil
}
func (store *memoryStore) Entry(_ context.Context, session, id string) (creationdomain.EntryRevision, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	entry, ok := store.entries[id]
	if !ok || entry.SessionID != session {
		return creationdomain.EntryRevision{}, sql.ErrNoRows
	}
	return entry, nil
}
func (store *memoryStore) AppendEntry(_ context.Context, expected creationdomain.Session, entry creationdomain.EntryRevision) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	if err := entry.Validate(); err != nil {
		return err
	}
	current, ok := store.sessions[expected.ProjectID]
	if !ok || current.ID != expected.ID || current.BranchID != expected.BranchID || current.ContentVersion != expected.ContentVersion || current.RunSlotVersion != expected.RunSlotVersion || current.ActiveRunID != "" || current.ActiveOperationID != "" || entry.SessionID != current.ID {
		return creationdomain.ErrSessionVersionConflict
	}
	parent := store.entries[store.entryHeads[current.ID]]
	if entry.ParentID != parent.ID || entry.Version != parent.Version+1 {
		return creationdomain.ErrSessionVersionConflict
	}
	if _, exists := store.entries[entry.ID]; exists {
		return creationdomain.ErrSessionVersionConflict
	}
	store.entries[entry.ID] = entry
	store.entryHeads[current.ID] = entry.ID
	current.ContentVersion++
	current.Stage = creationdomain.StageIntent
	current.AdvanceMode = creationdomain.ModeStepReview
	current.UpdatedAt = entry.CreatedAt
	store.sessions[current.ProjectID] = current
	return nil
}

// SaveSample 模拟事务内的会话与候选快照校验。
func (store *memoryStore) SaveSample(_ context.Context, s creationdomain.Session, c creationdomain.Candidate, h creationdomain.SampleHandoff) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	current := store.sessions[s.ProjectID]
	if current.ID != s.ID || current.ContentVersion != s.ContentVersion || current.RunSlotVersion != s.RunSlotVersion || current.ActiveRunID != "" || current.ActiveOperationID != "" {
		return creationdomain.ErrSessionVersionConflict
	}
	if !reflect.DeepEqual(store.candidates[c.ID], c) {
		return creationdomain.ErrCandidateVersionConflict
	}
	if err := h.Validate(c); err != nil {
		return err
	}
	if store.samples == nil {
		store.samples = map[string]creationdomain.SampleHandoff{}
	}
	h.Constraints = append([]string{}, h.Constraints...)
	store.samples[s.ID] = h
	current.ContentVersion++
	store.sessions[s.ProjectID] = current
	return nil
}

// LatestSample 返回副本，避免调用方改写已保存的约束。
func (store *memoryStore) LatestSample(_ context.Context, id string) (creationdomain.SampleHandoff, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	h, ok := store.samples[id]
	if !ok {
		return h, sql.ErrNoRows
	}
	h.Constraints = append([]string{}, h.Constraints...)
	return h, nil
}

// Review 返回候选逐项审阅关系的副本；未记录时返回空审阅。
func (store *memoryStore) Review(_ context.Context, candidateID string) (creationdomain.CandidateReview, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	review, ok := store.reviews[candidateID]
	if !ok {
		return creationdomain.CandidateReview{}, nil
	}
	return cloneReview(review), nil
}

// SaveReview 与 SQLite 保持相同的 review_version 条件语义。
func (store *memoryStore) SaveReview(_ context.Context, expected, updated creationdomain.CandidateReview) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	current, ok := store.reviews[updated.CandidateID]
	if !ok {
		if expected.ReviewVersion != 0 {
			return creationdomain.ErrReviewVersionConflict
		}
		updated.ReviewVersion = 1
		store.reviews[updated.CandidateID] = cloneReview(updated)
		return nil
	}
	if current.ReviewVersion != expected.ReviewVersion {
		return creationdomain.ErrReviewVersionConflict
	}
	updated.ReviewVersion = current.ReviewVersion + 1
	updated.CreatedAt = current.CreatedAt
	store.reviews[updated.CandidateID] = cloneReview(updated)
	return nil
}

// ReviewCursor 返回作者审阅位置；未记录时返回 ErrReviewCursorNotFound。
func (store *memoryStore) ReviewCursor(_ context.Context, sessionID string) (creationdomain.ReviewCursor, error) {
	store.mu.Lock()
	defer store.mu.Unlock()
	cursor, ok := store.cursors[sessionID]
	if !ok {
		return creationdomain.ReviewCursor{}, ErrReviewCursorNotFound
	}
	return cursor, nil
}

// SaveReviewCursor 与 SQLite 保持相同的 view_version 条件语义。
func (store *memoryStore) SaveReviewCursor(_ context.Context, cursor creationdomain.ReviewCursor) error {
	store.mu.Lock()
	defer store.mu.Unlock()
	current, ok := store.cursors[cursor.SessionID]
	if !ok {
		cursor.ViewVersion = 1
		store.cursors[cursor.SessionID] = cursor
		return nil
	}
	if current.ViewVersion != cursor.ViewVersion {
		return creationdomain.ErrReviewVersionConflict
	}
	cursor.ViewVersion = current.ViewVersion + 1
	store.cursors[cursor.SessionID] = cursor
	return nil
}

func cloneReview(review creationdomain.CandidateReview) creationdomain.CandidateReview {
	review.Accepted = append([]creationdomain.ReviewRef{}, review.Accepted...)
	review.Rejected = append([]creationdomain.ReviewRef{}, review.Rejected...)
	return review
}

// HasSavedManuscript 内存策划测试仓库不保存正文，其历史安全行为由 SQLite 合同测试覆盖。
func (store *memoryStore) HasSavedManuscript(context.Context) (bool, error) { return false, nil }
