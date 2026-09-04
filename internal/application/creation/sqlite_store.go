package creation

import (
	"context"

	assistantdomain "github.com/lishen802/novel-studio/internal/domain/assistant"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
)

// SQLBackend 是 SQLite 创作表的最小持久化端口，供工作区把活动项目 store 适配进来。
type SQLBackend interface {
	SaveCreationSession(context.Context, creationdomain.Session) error
	CreationSession(context.Context, string) (creationdomain.Session, error)
	SaveCreationCandidate(context.Context, creationdomain.Candidate) error
	UpdateCreationCandidate(context.Context, creationdomain.Candidate) error
	CreationCandidate(context.Context, string) (creationdomain.Candidate, error)
	ListCreationCandidates(context.Context, string) ([]creationdomain.Candidate, error)
	PendingAssistantActionCount(context.Context, string) (int, error)
	Project(context.Context) (projectdomain.Project, error)
	WorkbenchState(context.Context) (projectdomain.WorkbenchState, error)
	ApplyCreationArchive(context.Context, revisiondomain.ChangeSet, creationdomain.Session, []creationdomain.Candidate, projectdomain.WorkbenchState) error
	SaveAssistantSessionRecord(context.Context, assistantdomain.Session) error
	AssistantSession(context.Context, string, string) (assistantdomain.Session, error)
	SaveAssistantAction(context.Context, assistantdomain.Action) error
	AssistantAction(context.Context, string) (assistantdomain.Action, error)
	AssistantActionByIdempotency(context.Context, string, string) (assistantdomain.Action, error)
	ListAssistantActions(context.Context, string) ([]assistantdomain.Action, error)
}

type sqliteStore struct {
	inner SQLBackend
}

func NewSQLiteStore(store SQLBackend) Store {
	return sqliteStore{inner: store}
}

func (store sqliteStore) SaveSession(ctx context.Context, session creationdomain.Session) error {
	return store.inner.SaveCreationSession(ctx, session)
}

func (store sqliteStore) Session(ctx context.Context, projectID string) (creationdomain.Session, error) {
	return store.inner.CreationSession(ctx, projectID)
}

func (store sqliteStore) SaveCandidate(ctx context.Context, candidate creationdomain.Candidate) error {
	return store.inner.SaveCreationCandidate(ctx, candidate)
}

func (store sqliteStore) UpdateCandidate(ctx context.Context, candidate creationdomain.Candidate) error {
	return store.inner.UpdateCreationCandidate(ctx, candidate)
}

func (store sqliteStore) Candidate(ctx context.Context, id string) (creationdomain.Candidate, error) {
	return store.inner.CreationCandidate(ctx, id)
}

func (store sqliteStore) ListCandidates(ctx context.Context, sessionID string) ([]creationdomain.Candidate, error) {
	return store.inner.ListCreationCandidates(ctx, sessionID)
}

func (store sqliteStore) PendingActionCount(ctx context.Context, projectID string) (int, error) {
	return store.inner.PendingAssistantActionCount(ctx, projectID)
}

func (store sqliteStore) Project(ctx context.Context) (projectdomain.Project, error) {
	return store.inner.Project(ctx)
}

func (store sqliteStore) WorkbenchState(ctx context.Context) (projectdomain.WorkbenchState, error) {
	return store.inner.WorkbenchState(ctx)
}

func (store sqliteStore) ApplyArchive(ctx context.Context, apply ArchiveApply) error {
	return store.inner.ApplyCreationArchive(ctx, apply.ChangeSet, apply.Session, apply.Candidates, apply.Workbench)
}

func (store sqliteStore) SaveAssistantSession(ctx context.Context, session assistantdomain.Session) error {
	return store.inner.SaveAssistantSessionRecord(ctx, session)
}

func (store sqliteStore) AssistantSession(ctx context.Context, projectID, branchID string) (assistantdomain.Session, error) {
	return store.inner.AssistantSession(ctx, projectID, branchID)
}

func (store sqliteStore) SaveAction(ctx context.Context, action assistantdomain.Action) error {
	return store.inner.SaveAssistantAction(ctx, action)
}

func (store sqliteStore) Action(ctx context.Context, id string) (assistantdomain.Action, error) {
	return store.inner.AssistantAction(ctx, id)
}

func (store sqliteStore) ActionByIdempotency(ctx context.Context, sessionID, key string) (assistantdomain.Action, error) {
	return store.inner.AssistantActionByIdempotency(ctx, sessionID, key)
}

func (store sqliteStore) ListActions(ctx context.Context, sessionID string) ([]assistantdomain.Action, error) {
	return store.inner.ListAssistantActions(ctx, sessionID)
}
