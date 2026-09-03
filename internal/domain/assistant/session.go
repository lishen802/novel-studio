package assistant

import (
	"strings"
	"time"
)

type SessionKind string

const KindCreation SessionKind = "creation"

type Session struct {
	ID        string
	ProjectID string
	BranchID  string
	Kind      SessionKind
	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewSession 为作品分支建立创作对话会话，切换作品必须换会话。
func NewSession(id, projectID, branchID string, now time.Time) (Session, error) {
	id, projectID, branchID = strings.TrimSpace(id), strings.TrimSpace(projectID), strings.TrimSpace(branchID)
	if id == "" || projectID == "" || branchID == "" {
		return Session{}, ErrBlankValue
	}
	now = now.UTC()
	return Session{
		ID: id, ProjectID: projectID, BranchID: branchID, Kind: KindCreation,
		CreatedAt: now, UpdatedAt: now,
	}, nil
}
