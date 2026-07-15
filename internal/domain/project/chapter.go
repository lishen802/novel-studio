package project

import (
	"strings"
	"time"
)

type Chapter struct {
	ID        string
	ProjectID string
	Title     string
	Position  int
	Body      string
	Revision  int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewChapter(id, projectID, title string, position int, now time.Time) (Chapter, error) {
	id = strings.TrimSpace(id)
	projectID = strings.TrimSpace(projectID)
	if id == "" || projectID == "" {
		return Chapter{}, ErrBlankID
	}

	title = strings.TrimSpace(title)
	if title == "" {
		return Chapter{}, ErrBlankTitle
	}
	if position < 1 {
		return Chapter{}, ErrInvalidPosition
	}

	now = now.UTC()
	return Chapter{
		ID:        id,
		ProjectID: projectID,
		Title:     title,
		Position:  position,
		Revision:  1,
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}

func (c Chapter) Commit(body string, expectedRevision int64, now time.Time) (Chapter, error) {
	if c.Revision != expectedRevision {
		return Chapter{}, ErrRevisionConflict
	}

	c.Body = body
	c.Revision++
	c.UpdatedAt = now.UTC()
	return c, nil
}
