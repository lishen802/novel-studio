package creation

import (
	"context"
	"database/sql"
	"errors"

	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
)

var (
	ErrSessionNotFound = errors.New("creation session not found")
	ErrStaleStore      = errors.New("creation store generation changed")
)

type Store interface {
	SaveSession(ctx context.Context, session creationdomain.Session) error
	Session(ctx context.Context, projectID string) (creationdomain.Session, error)
	SaveCandidate(ctx context.Context, candidate creationdomain.Candidate) error
	UpdateCandidate(ctx context.Context, candidate creationdomain.Candidate) error
	Candidate(ctx context.Context, id string) (creationdomain.Candidate, error)
	ListCandidates(ctx context.Context, sessionID string) ([]creationdomain.Candidate, error)
	PendingActionCount(ctx context.Context, projectID string) (int, error)
	Project(ctx context.Context) (projectdomain.Project, error)
	WorkbenchState(ctx context.Context) (projectdomain.WorkbenchState, error)
	ApplyArchive(ctx context.Context, apply ArchiveApply) error
}

type ArchiveApply struct {
	ChangeSet  revisiondomain.ChangeSet
	Session    creationdomain.Session
	Candidates []creationdomain.Candidate
	Workbench  projectdomain.WorkbenchState
}

func isMissingSession(err error) bool {
	return errors.Is(err, ErrSessionNotFound) || errors.Is(err, sql.ErrNoRows)
}
