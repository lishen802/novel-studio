package revision

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrBlankValue        = errors.New("required value must not be blank")
	ErrInvalidRevision   = errors.New("base revision must be positive")
	ErrNoopOperation     = errors.New("patch operation must change content")
	ErrInvalidTransition = errors.New("invalid change set status transition")
)

type ChangeSetStatus string

const (
	ChangeSetPending    ChangeSetStatus = "pending"
	ChangeSetApplied    ChangeSetStatus = "applied"
	ChangeSetRejected   ChangeSetStatus = "rejected"
	ChangeSetConflicted ChangeSetStatus = "conflicted"
)

type OperationKind string

const OperationReplaceChapterBody OperationKind = "replace_chapter_body"

type PatchOperation struct {
	ID           string
	ChangeSetID  string
	Kind         OperationKind
	TargetID     string
	BaseRevision int64
	BeforeValue  string
	AfterValue   string
	Reason       string
	Status       ChangeSetStatus
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type ChangeSet struct {
	ID         string
	ProjectID  string
	Title      string
	Summary    string
	CreatedBy  string
	Status     ChangeSetStatus
	Operations []PatchOperation
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

func NewChapterBodyChangeSet(
	id string,
	operationID string,
	projectID string,
	chapterID string,
	title string,
	summary string,
	createdBy string,
	baseRevision int64,
	beforeBody string,
	afterBody string,
	reason string,
	now time.Time,
) (ChangeSet, error) {
	id = strings.TrimSpace(id)
	operationID = strings.TrimSpace(operationID)
	projectID = strings.TrimSpace(projectID)
	chapterID = strings.TrimSpace(chapterID)
	title = strings.TrimSpace(title)
	createdBy = strings.TrimSpace(createdBy)
	if id == "" || operationID == "" || projectID == "" || chapterID == "" || title == "" || createdBy == "" {
		return ChangeSet{}, ErrBlankValue
	}
	if baseRevision < 1 {
		return ChangeSet{}, ErrInvalidRevision
	}
	if beforeBody == afterBody {
		return ChangeSet{}, ErrNoopOperation
	}

	now = now.UTC()
	return ChangeSet{
		ID:        id,
		ProjectID: projectID,
		Title:     title,
		Summary:   strings.TrimSpace(summary),
		CreatedBy: createdBy,
		Status:    ChangeSetPending,
		Operations: []PatchOperation{{
			ID:           operationID,
			ChangeSetID:  id,
			Kind:         OperationReplaceChapterBody,
			TargetID:     chapterID,
			BaseRevision: baseRevision,
			BeforeValue:  beforeBody,
			AfterValue:   afterBody,
			Reason:       strings.TrimSpace(reason),
			Status:       ChangeSetPending,
			CreatedAt:    now,
			UpdatedAt:    now,
		}},
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (c ChangeSet) Apply(now time.Time) (ChangeSet, error) {
	if c.Status == ChangeSetApplied {
		return c, nil
	}
	if c.Status != ChangeSetPending {
		return ChangeSet{}, ErrInvalidTransition
	}
	return c.withStatus(ChangeSetApplied, now), nil
}

func (c ChangeSet) Reject(now time.Time) (ChangeSet, error) {
	if c.Status == ChangeSetRejected {
		return c, nil
	}
	if c.Status != ChangeSetPending && c.Status != ChangeSetConflicted {
		return ChangeSet{}, ErrInvalidTransition
	}
	return c.withStatus(ChangeSetRejected, now), nil
}

func (c ChangeSet) Conflict(now time.Time) (ChangeSet, error) {
	if c.Status == ChangeSetConflicted {
		return c, nil
	}
	if c.Status != ChangeSetPending {
		return ChangeSet{}, ErrInvalidTransition
	}
	return c.withStatus(ChangeSetConflicted, now), nil
}

func (c ChangeSet) withStatus(status ChangeSetStatus, now time.Time) ChangeSet {
	now = now.UTC()
	c.Status = status
	c.UpdatedAt = now
	for index := range c.Operations {
		c.Operations[index].Status = status
		c.Operations[index].UpdatedAt = now
	}
	return c
}
