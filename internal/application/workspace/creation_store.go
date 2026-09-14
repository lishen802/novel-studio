package workspace

import (
	"context"
	creationapp "github.com/lishen802/novel-studio/internal/application/creation"
	assistantdomain "github.com/lishen802/novel-studio/internal/domain/assistant"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
)

// creationStore 为创作会话、候选及回执统一提供短锁与项目代次保护。
// 后台终态必须保留租约上下文；项目排水超时后禁止继续落库。
type creationStore struct {
	inner creationapp.Store
	guard creationKernelStore
}

var _ creationapp.Store = creationStore{}

// CreateSession 在工作区短锁内原子创建或返回已有会话。
func (store creationStore) CreateSession(ctx context.Context, proposed creationdomain.Session) (session creationdomain.Session, err error) {
	err = store.guard.access(ctx, true, func() error { session, err = store.inner.CreateSession(ctx, proposed); return err })
	return
}

// SaveRunSlot 对独立的运行槽更新同样验证工作区代次和租约。
func (store creationStore) SaveRunSlot(ctx context.Context, session creationdomain.Session) error {
	return store.guard.access(ctx, true, func() error { return store.inner.SaveRunSlot(ctx, session) })
}

// SaveSession 在访问创作存储前核对工作区和运行租约。
func (store creationStore) SaveSession(ctx context.Context, session creationdomain.Session) error {
	return store.guard.access(ctx, true, func() error { return store.inner.SaveSession(ctx, session) })
}

// Session 在访问创作存储前核对工作区和运行租约。
func (store creationStore) Session(ctx context.Context, projectID string) (session creationdomain.Session, err error) {
	err = store.guard.access(ctx, false, func() error { session, err = store.inner.Session(ctx, projectID); return err })
	return
}

// SaveCandidate 在访问创作存储前核对工作区和运行租约。
func (store creationStore) SaveCandidate(ctx context.Context, candidate creationdomain.Candidate) error {
	return store.guard.access(ctx, true, func() error { return store.inner.SaveCandidate(ctx, candidate) })
}

// UpdateCandidate 在访问创作存储前核对工作区和运行租约。
func (store creationStore) UpdateCandidate(ctx context.Context, candidate creationdomain.Candidate) error {
	return store.guard.access(ctx, true, func() error { return store.inner.UpdateCandidate(ctx, candidate) })
}

// ApplyCandidateChanges 在同一工作区租约内提交候选换版事务。
func (store creationStore) ApplyCandidateChanges(ctx context.Context, expected, updates, inserts []creationdomain.Candidate, guards ...creationdomain.CandidateCommitGuard) error {
	return store.guard.access(ctx, true, func() error {
		return store.inner.ApplyCandidateChanges(ctx, expected, updates, inserts, guards...)
	})
}

// Candidate 在访问创作存储前核对工作区和运行租约。
func (store creationStore) Candidate(ctx context.Context, id string) (candidate creationdomain.Candidate, err error) {
	err = store.guard.access(ctx, false, func() error { candidate, err = store.inner.Candidate(ctx, id); return err })
	return
}

// ListCandidates 在访问创作存储前核对工作区和运行租约。
func (store creationStore) ListCandidates(ctx context.Context, sessionID string) (candidates []creationdomain.Candidate, err error) {
	err = store.guard.access(ctx, false, func() error { candidates, err = store.inner.ListCandidates(ctx, sessionID); return err })
	return
}

// PendingActionCount 在访问创作存储前核对工作区和运行租约。
func (store creationStore) PendingActionCount(ctx context.Context, projectID string) (count int, err error) {
	err = store.guard.access(ctx, false, func() error { count, err = store.inner.PendingActionCount(ctx, projectID); return err })
	return
}

// Project 在访问创作存储前核对工作区和运行租约。
func (store creationStore) Project(ctx context.Context) (project projectdomain.Project, err error) {
	err = store.guard.access(ctx, false, func() error { project, err = store.inner.Project(ctx); return err })
	return
}

// WorkbenchState 在访问创作存储前核对工作区和运行租约。
func (store creationStore) WorkbenchState(ctx context.Context) (state projectdomain.WorkbenchState, err error) {
	err = store.guard.access(ctx, false, func() error { state, err = store.inner.WorkbenchState(ctx); return err })
	return
}

// ApplyArchive 在访问创作存储前核对工作区和运行租约。
func (store creationStore) ApplyArchive(ctx context.Context, apply creationapp.ArchiveApply) error {
	return store.guard.access(ctx, true, func() error { return store.inner.ApplyArchive(ctx, apply) })
}

// ReserveOperation 在访问创作存储前核对工作区和运行租约。
func (store creationStore) ReserveOperation(ctx context.Context, operation creationdomain.Operation) (reserved creationdomain.Operation, created bool, err error) {
	err = store.guard.access(ctx, true, func() error { reserved, created, err = store.inner.ReserveOperation(ctx, operation); return err })
	return
}

// Operation 在访问创作存储前核对工作区和运行租约。
func (store creationStore) Operation(ctx context.Context, id string) (operation creationdomain.Operation, err error) {
	err = store.guard.access(ctx, false, func() error { operation, err = store.inner.Operation(ctx, id); return err })
	return
}

// OperationByIdempotency 在访问创作存储前核对工作区和运行租约。
func (store creationStore) OperationByIdempotency(ctx context.Context, projectID, branchID, command, key string) (operation creationdomain.Operation, err error) {
	err = store.guard.access(ctx, false, func() error {
		operation, err = store.inner.OperationByIdempotency(ctx, projectID, branchID, command, key)
		return err
	})
	return
}

// UpdateOperation 在访问创作存储前核对工作区和运行租约。
func (store creationStore) UpdateOperation(ctx context.Context, operation creationdomain.Operation) error {
	return store.guard.access(ctx, true, func() error { return store.inner.UpdateOperation(ctx, operation) })
}

// SaveAssistantSession 在访问创作存储前核对工作区和运行租约。
func (store creationStore) SaveAssistantSession(ctx context.Context, session assistantdomain.Session) error {
	return store.guard.access(ctx, true, func() error { return store.inner.SaveAssistantSession(ctx, session) })
}

// AssistantSession 在访问创作存储前核对工作区和运行租约。
func (store creationStore) AssistantSession(ctx context.Context, projectID, branchID string) (session assistantdomain.Session, err error) {
	err = store.guard.access(ctx, false, func() error { session, err = store.inner.AssistantSession(ctx, projectID, branchID); return err })
	return
}

// SaveAction 在访问创作存储前核对工作区和运行租约。
func (store creationStore) SaveAction(ctx context.Context, action assistantdomain.Action) error {
	return store.guard.access(ctx, true, func() error { return store.inner.SaveAction(ctx, action) })
}

// Action 在访问创作存储前核对工作区和运行租约。
func (store creationStore) Action(ctx context.Context, id string) (action assistantdomain.Action, err error) {
	err = store.guard.access(ctx, false, func() error { action, err = store.inner.Action(ctx, id); return err })
	return
}

// ActionByIdempotency 在访问创作存储前核对工作区和运行租约。
func (store creationStore) ActionByIdempotency(ctx context.Context, sessionID, key string) (action assistantdomain.Action, err error) {
	err = store.guard.access(ctx, false, func() error { action, err = store.inner.ActionByIdempotency(ctx, sessionID, key); return err })
	return
}

// ListActions 在访问创作存储前核对工作区和运行租约。
func (store creationStore) ListActions(ctx context.Context, sessionID string) (actions []assistantdomain.Action, err error) {
	err = store.guard.access(ctx, false, func() error { actions, err = store.inner.ListActions(ctx, sessionID); return err })
	return
}

// CurrentEntry 在项目代次保护下读取当前作者来源。
func (store creationStore) CurrentEntry(ctx context.Context, id string) (entry creationdomain.EntryRevision, err error) {
	err = store.guard.access(ctx, false, func() error { entry, err = store.inner.CurrentEntry(ctx, id); return err })
	return
}

// Entry 在项目代次保护下读取来源历史。
func (store creationStore) Entry(ctx context.Context, session, id string) (entry creationdomain.EntryRevision, err error) {
	err = store.guard.access(ctx, false, func() error { entry, err = store.inner.Entry(ctx, session, id); return err })
	return
}

// AppendEntry 在可写项目保护下原子保存作者来源。
func (store creationStore) AppendEntry(ctx context.Context, session creationdomain.Session, entry creationdomain.EntryRevision) error {
	return store.guard.access(ctx, true, func() error { return store.inner.AppendEntry(ctx, session, entry) })
}

// SaveSample 在可写项目保护下保存试写准备。
func (store creationStore) SaveSample(ctx context.Context, s creationdomain.Session, c creationdomain.Candidate, h creationdomain.SampleHandoff) error {
	return store.guard.access(ctx, true, func() error { return store.inner.SaveSample(ctx, s, c, h) })
}

// LatestSample 在项目代次保护下读取试写准备。
func (store creationStore) LatestSample(ctx context.Context, id string) (h creationdomain.SampleHandoff, err error) {
	err = store.guard.access(ctx, false, func() error { h, err = store.inner.LatestSample(ctx, id); return err })
	return
}
