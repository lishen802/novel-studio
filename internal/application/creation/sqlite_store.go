package creation

import (
	"context"

	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
)

type sqliteStore struct {
	inner *sqlite.Store
}

func NewSQLiteStore(store *sqlite.Store) Store {
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
