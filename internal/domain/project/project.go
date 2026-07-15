package project

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrBlankID          = errors.New("id must not be blank")
	ErrBlankTitle       = errors.New("title must not be blank")
	ErrInvalidPosition  = errors.New("chapter position must be positive")
	ErrRevisionConflict = errors.New("chapter revision conflict")
)

type Project struct {
	ID        string
	Title     string
	Genre     string
	Summary   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewProject(id, title, genre, summary string, now time.Time) (Project, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return Project{}, ErrBlankID
	}

	title = strings.TrimSpace(title)
	if title == "" {
		return Project{}, ErrBlankTitle
	}

	now = now.UTC()
	return Project{
		ID:        id,
		Title:     title,
		Genre:     strings.TrimSpace(genre),
		Summary:   strings.TrimSpace(summary),
		CreatedAt: now,
		UpdatedAt: now,
	}, nil
}
