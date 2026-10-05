package workspace

import (
	"context"
	creationapp "github.com/lishen802/novel-studio/internal/application/creation"
	assistantdomain "github.com/lishen802/novel-studio/internal/domain/assistant"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
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

// Review 在项目代次保护下读取候选逐项审阅关系。
func (store creationStore) Review(ctx context.Context, candidateID string) (review creationdomain.CandidateReview, err error) {
	err = store.guard.access(ctx, false, func() error { review, err = store.inner.Review(ctx, candidateID); return err })
	return
}

// SaveReview 在可写项目保护下按 review_version 写入逐项审阅关系。
func (store creationStore) SaveReview(ctx context.Context, expected, updated creationdomain.CandidateReview) error {
	return store.guard.access(ctx, true, func() error { return store.inner.SaveReview(ctx, expected, updated) })
}

// ReviewCursor 在项目代次保护下读取作者审阅位置。
func (store creationStore) ReviewCursor(ctx context.Context, sessionID string) (cursor creationdomain.ReviewCursor, err error) {
	err = store.guard.access(ctx, false, func() error { cursor, err = store.inner.ReviewCursor(ctx, sessionID); return err })
	return
}

// SaveReviewCursor 在可写项目保护下按 view_version 保存作者审阅位置。
func (store creationStore) SaveReviewCursor(ctx context.Context, cursor creationdomain.ReviewCursor) error {
	return store.guard.access(ctx, true, func() error { return store.inner.SaveReviewCursor(ctx, cursor) })
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

// LatestOperation 在项目代次保护下读取最近一次真实运行回执。
func (store creationStore) LatestOperation(ctx context.Context, projectID, branchID string) (operation creationdomain.Operation, err error) {
	err = store.guard.access(ctx, false, func() error { operation, err = store.inner.LatestOperation(ctx, projectID, branchID); return err })
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

// SaveAssistantMessage 在项目代次保护下保存公开对话消息。
func (store creationStore) SaveAssistantMessage(ctx context.Context, message assistantdomain.Message) error {
	return store.guard.access(ctx, true, func() error { return store.inner.SaveAssistantMessage(ctx, message) })
}

// UpdateAssistantMessage 在项目代次保护下更新讨论运行状态。
func (store creationStore) UpdateAssistantMessage(ctx context.Context, message assistantdomain.Message) error {
	return store.guard.access(ctx, true, func() error { return store.inner.UpdateAssistantMessage(ctx, message) })
}

// ListAssistantMessages 在项目代次保护下恢复公开对话消息。
func (store creationStore) ListAssistantMessages(ctx context.Context, sessionID string) (messages []assistantdomain.Message, err error) {
	err = store.guard.access(ctx, false, func() error { messages, err = store.inner.ListAssistantMessages(ctx, sessionID); return err })
	return
}

// AssistantMessageByIdempotency 在项目代次保护下按幂等键读取消息。
func (store creationStore) AssistantMessageByIdempotency(ctx context.Context, sessionID, key string) (message assistantdomain.Message, err error) {
	err = store.guard.access(ctx, false, func() error {
		message, err = store.inner.AssistantMessageByIdempotency(ctx, sessionID, key)
		return err
	})
	return
}

// SaveAuthorization 在项目代次保护下保存讨论授权。
func (store creationStore) SaveAuthorization(ctx context.Context, auth assistantdomain.Authorization) error {
	return store.guard.access(ctx, true, func() error { return store.inner.SaveAuthorization(ctx, auth) })
}

// Authorization 在项目代次保护下读取讨论授权。
func (store creationStore) Authorization(ctx context.Context, id string) (auth assistantdomain.Authorization, err error) {
	err = store.guard.access(ctx, false, func() error { auth, err = store.inner.Authorization(ctx, id); return err })
	return
}

// AuthorizationByAction 在项目代次保护下按 Action 读取授权。
func (store creationStore) AuthorizationByAction(ctx context.Context, actionID string) (auth assistantdomain.Authorization, err error) {
	err = store.guard.access(ctx, false, func() error { auth, err = store.inner.AuthorizationByAction(ctx, actionID); return err })
	return
}

// ListAuthorizations 在项目代次保护下列出讨论授权。
func (store creationStore) ListAuthorizations(ctx context.Context, sessionID string) (items []assistantdomain.Authorization, err error) {
	err = store.guard.access(ctx, false, func() error { items, err = store.inner.ListAuthorizations(ctx, sessionID); return err })
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

// HasSavedManuscript 在读取历史保护状态前核对活动项目代次，避免跨项目检查。
func (store creationStore) HasSavedManuscript(ctx context.Context) (saved bool, err error) {
	err = store.guard.access(ctx, false, func() error { saved, err = store.inner.HasSavedManuscript(ctx); return err })
	return
}

// FormalCharacterRevisionBase 在项目代次保护下读取正式人物基线。
func (store creationStore) FormalCharacterRevisionBase(ctx context.Context, project, branch, id string) (base asset.CharacterRevisionBase, err error) {
	err = store.guard.access(ctx, false, func() error {
		reader, ok := store.inner.(interface {
			FormalCharacterRevisionBase(context.Context, string, string, string) (asset.CharacterRevisionBase, error)
		})
		if !ok {
			return creationapp.ErrUnavailable
		}
		// 既有创作会话使用 main 别名；正式修订以磁盘分支目录的活动 ID 为权威。
		actual, e := store.guard.workspace.constraintBranchLocked()
		if e != nil {
			return e
		}
		base, err = reader.FormalCharacterRevisionBase(ctx, project, actual, id)
		return err
	})
	return
}
