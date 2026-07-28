package revision

import (
	"errors"
	"strings"
	"time"
)

var ErrInvalidSnapshotKind = errors.New("invalid snapshot kind")

type RevisionSource string

const (
	RevisionInitial RevisionSource = "initial"
	RevisionAuthor  RevisionSource = "author"
	RevisionAgent   RevisionSource = "agent"
	RevisionRestore RevisionSource = "restore"
)

type ChapterRevision struct {
	ChapterID string
	Revision  int64
	Title     string
	Body      string
	Source    RevisionSource
	Summary   string
	CreatedAt time.Time
}

type SnapshotKind string

const (
	SnapshotNamed    SnapshotKind = "named"
	SnapshotRecovery SnapshotKind = "recovery"
)

type Snapshot struct {
	ID        string
	ProjectID string
	Name      string
	Kind      SnapshotKind
	CreatedAt time.Time
}

func NewSnapshot(
	id string,
	projectID string,
	name string,
	kind SnapshotKind,
	now time.Time,
) (Snapshot, error) {
	id = strings.TrimSpace(id)
	projectID = strings.TrimSpace(projectID)
	name = strings.TrimSpace(name)
	if id == "" || projectID == "" || name == "" {
		return Snapshot{}, ErrBlankValue
	}
	if kind != SnapshotNamed && kind != SnapshotRecovery {
		return Snapshot{}, ErrInvalidSnapshotKind
	}
	return Snapshot{
		ID:        id,
		ProjectID: projectID,
		Name:      name,
		Kind:      kind,
		CreatedAt: now.UTC(),
	}, nil
}
