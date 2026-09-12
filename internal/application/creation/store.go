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

var (
	ErrSessionNotFound   = errors.New("creation session not found")
	ErrStaleStore        = errors.New("creation store generation changed")
	ErrOperationNotFound = errors.New("creation operation not found")
	ErrOperationConflict = errors.New("creation operation idempotency conflict")
)

type Store interface {
	CurrentEntry(context.Context, string) (creationdomain.EntryRevision, error)
	Entry(context.Context, string, string) (creationdomain.EntryRevision, error)
	AppendEntry(context.Context, creationdomain.Session, creationdomain.EntryRevision) error
	CreateSession(ctx context.Context, session creationdomain.Session) (creationdomain.Session, error)
	// SaveSession 以 ContentVersion 为前置条件，仅写作者内容；成功后内容版本加一。
	SaveSession(ctx context.Context, session creationdomain.Session) error
	// SaveRunSlot 以 RunSlotVersion 为前置条件，仅写运行引用与暂停原因；成功后槽版本加一。
	SaveRunSlot(ctx context.Context, session creationdomain.Session) error
	Session(ctx context.Context, projectID string) (creationdomain.Session, error)
	SaveCandidate(ctx context.Context, candidate creationdomain.Candidate) error
	UpdateCandidate(ctx context.Context, candidate creationdomain.Candidate) error
	// ApplyCandidateChanges 原子校验候选快照并提交状态更新和新版本。
	ApplyCandidateChanges(ctx context.Context, expected, updates, inserts []creationdomain.Candidate) error
	Candidate(ctx context.Context, id string) (creationdomain.Candidate, error)
	ListCandidates(ctx context.Context, sessionID string) ([]creationdomain.Candidate, error)
	PendingActionCount(ctx context.Context, projectID string) (int, error)
	Project(ctx context.Context) (projectdomain.Project, error)
	WorkbenchState(ctx context.Context) (projectdomain.WorkbenchState, error)
	ApplyArchive(ctx context.Context, apply ArchiveApply) error
	// ReserveOperation 以 (project, branch, command, key) 唯一键预留操作回执；
	// 键已存在时返回既有回执与 created=false，不依赖内存缓存即可跨重启去重。
	ReserveOperation(ctx context.Context, operation creationdomain.Operation) (creationdomain.Operation, bool, error)
	Operation(ctx context.Context, id string) (creationdomain.Operation, error)
	OperationByIdempotency(ctx context.Context, projectID, branchID, command, idempotencyKey string) (creationdomain.Operation, error)
	UpdateOperation(ctx context.Context, operation creationdomain.Operation) error
	SaveAssistantSession(ctx context.Context, session assistantdomain.Session) error
	AssistantSession(ctx context.Context, projectID, branchID string) (assistantdomain.Session, error)
	SaveAction(ctx context.Context, action assistantdomain.Action) error
	Action(ctx context.Context, id string) (assistantdomain.Action, error)
	ActionByIdempotency(ctx context.Context, sessionID, key string) (assistantdomain.Action, error)
	ListActions(ctx context.Context, sessionID string) ([]assistantdomain.Action, error)
}

type ArchiveApply struct {
	ChangeSet  revisiondomain.ChangeSet
	Session    creationdomain.Session
	Candidates []creationdomain.Candidate
	// Verify 是确认前置条件的一部分但不会改为 confirmed 的候选（如选中的方向），
	// 事务内按快照校验它们未被替代/废弃/改锁/改质量。
	Verify    []creationdomain.Candidate
	Workbench projectdomain.WorkbenchState
}

func isMissingSession(err error) bool {
	return errors.Is(err, ErrSessionNotFound) || errors.Is(err, sql.ErrNoRows)
}
