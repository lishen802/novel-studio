package creation

import (
	"context"
	"database/sql"
	"errors"

	assistantdomain "github.com/lishen802/novel-studio/internal/domain/assistant"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
)

// SQLBackend 是 SQLite 创作表的最小持久化端口，供工作区把活动项目 store 适配进来。
type SQLBackend interface {
	SaveCreationSample(context.Context, creationdomain.Session, creationdomain.Candidate, creationdomain.SampleHandoff) error
	LatestCreationSample(context.Context, string) (creationdomain.SampleHandoff, error)
	CurrentCreationEntry(context.Context, string) (creationdomain.EntryRevision, error)
	CreationEntry(context.Context, string, string) (creationdomain.EntryRevision, error)
	AppendCreationEntry(context.Context, creationdomain.Session, creationdomain.EntryRevision) error
	CreateCreationSession(context.Context, creationdomain.Session) (creationdomain.Session, error)
	SaveCreationSession(context.Context, creationdomain.Session) error
	SaveCreationRunSlot(context.Context, creationdomain.Session) error
	CreationSession(context.Context, string) (creationdomain.Session, error)
	SaveCreationCandidate(context.Context, creationdomain.Candidate) error
	UpdateCreationCandidate(context.Context, creationdomain.Candidate) error
	ApplyCreationCandidateChanges(context.Context, []creationdomain.Candidate, []creationdomain.Candidate, []creationdomain.Candidate, ...creationdomain.CandidateCommitGuard) error
	CreationCandidate(context.Context, string) (creationdomain.Candidate, error)
	ListCreationCandidates(context.Context, string) ([]creationdomain.Candidate, error)
	PendingAssistantActionCount(context.Context, string) (int, error)
	Project(context.Context) (projectdomain.Project, error)
	WorkbenchState(context.Context) (projectdomain.WorkbenchState, error)
	ApplyCreationArchive(context.Context, revisiondomain.ChangeSet, creationdomain.Session, []creationdomain.Candidate, []creationdomain.Candidate, projectdomain.WorkbenchState) error
	ReserveCreationOperation(context.Context, creationdomain.Operation) (creationdomain.Operation, bool, error)
	CreationOperation(context.Context, string) (creationdomain.Operation, error)
	CreationOperationByIdempotency(context.Context, string, string, string, string) (creationdomain.Operation, error)
	UpdateCreationOperation(context.Context, creationdomain.Operation) error
	SaveAssistantSessionRecord(context.Context, assistantdomain.Session) error
	AssistantSession(context.Context, string, string) (assistantdomain.Session, error)
	SaveAssistantMessage(context.Context, assistantdomain.Message) error
	ListAssistantMessages(context.Context, string) ([]assistantdomain.Message, error)
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

func (store sqliteStore) CreateSession(ctx context.Context, session creationdomain.Session) (creationdomain.Session, error) {
	return store.inner.CreateCreationSession(ctx, session)
}

func (store sqliteStore) SaveRunSlot(ctx context.Context, session creationdomain.Session) error {
	return store.inner.SaveCreationRunSlot(ctx, session)
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

func (store sqliteStore) ApplyCandidateChanges(ctx context.Context, expected, updates, inserts []creationdomain.Candidate, guards ...creationdomain.CandidateCommitGuard) error {
	return store.inner.ApplyCreationCandidateChanges(ctx, expected, updates, inserts, guards...)
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
	return store.inner.ApplyCreationArchive(ctx, apply.ChangeSet, apply.Session, apply.Candidates, apply.Verify, apply.Workbench)
}

func (store sqliteStore) ReserveOperation(ctx context.Context, operation creationdomain.Operation) (creationdomain.Operation, bool, error) {
	return store.inner.ReserveCreationOperation(ctx, operation)
}

func (store sqliteStore) Operation(ctx context.Context, id string) (creationdomain.Operation, error) {
	operation, err := store.inner.CreationOperation(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return creationdomain.Operation{}, ErrOperationNotFound
	}
	return operation, err
}

func (store sqliteStore) OperationByIdempotency(ctx context.Context, projectID, branchID, command, idempotencyKey string) (creationdomain.Operation, error) {
	operation, err := store.inner.CreationOperationByIdempotency(ctx, projectID, branchID, command, idempotencyKey)
	if errors.Is(err, sql.ErrNoRows) {
		return creationdomain.Operation{}, ErrOperationNotFound
	}
	return operation, err
}

func (store sqliteStore) UpdateOperation(ctx context.Context, operation creationdomain.Operation) error {
	return store.inner.UpdateCreationOperation(ctx, operation)
}

func (store sqliteStore) SaveAssistantSession(ctx context.Context, session assistantdomain.Session) error {
	return store.inner.SaveAssistantSessionRecord(ctx, session)
}

func (store sqliteStore) AssistantSession(ctx context.Context, projectID, branchID string) (assistantdomain.Session, error) {
	return store.inner.AssistantSession(ctx, projectID, branchID)
}

func (store sqliteStore) SaveAssistantMessage(ctx context.Context, message assistantdomain.Message) error {
	return store.inner.SaveAssistantMessage(ctx, message)
}

func (store sqliteStore) ListAssistantMessages(ctx context.Context, sessionID string) ([]assistantdomain.Message, error) {
	return store.inner.ListAssistantMessages(ctx, sessionID)
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

func (store sqliteStore) CurrentEntry(ctx context.Context, id string) (creationdomain.EntryRevision, error) {
	return store.inner.CurrentCreationEntry(ctx, id)
}
func (store sqliteStore) Entry(ctx context.Context, session, id string) (creationdomain.EntryRevision, error) {
	return store.inner.CreationEntry(ctx, session, id)
}
func (store sqliteStore) AppendEntry(ctx context.Context, session creationdomain.Session, entry creationdomain.EntryRevision) error {
	return store.inner.AppendCreationEntry(ctx, session, entry)
}

// SaveSample 原子保存来源快照与试写准备。
func (store sqliteStore) SaveSample(ctx context.Context, s creationdomain.Session, c creationdomain.Candidate, h creationdomain.SampleHandoff) error {
	return store.inner.SaveCreationSample(ctx, s, c, h)
}

// LatestSample 读取最近的试写准备。
func (store sqliteStore) LatestSample(ctx context.Context, id string) (creationdomain.SampleHandoff, error) {
	return store.inner.LatestCreationSample(ctx, id)
}
