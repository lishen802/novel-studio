package canon

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrBlankValue        = errors.New("canon value must not be blank")
	ErrInvalidStatus     = errors.New("invalid canon status")
	ErrInvalidConfidence = errors.New("canon confidence must be between zero and one")
	ErrInvalidEvidence   = errors.New("invalid canon evidence")
	ErrInvalidReference  = errors.New("invalid canon reference")
	ErrReferenced        = errors.New("canon entity is still referenced")
)

type FactStatus string

const (
	StatusAuthorConfirmed FactStatus = "author_confirmed"
	StatusTextEvidenced   FactStatus = "text_evidenced"
	StatusAIInferred      FactStatus = "ai_inferred"
	StatusConflicted      FactStatus = "conflicted"
	StatusRejected        FactStatus = "rejected"
)

type FactOrigin string

const (
	OriginAuthor FactOrigin = "author"
	OriginText   FactOrigin = "text"
	OriginModel  FactOrigin = "model"
	OriginRule   FactOrigin = "rule"
)

type Fact struct {
	ID                string
	ProjectID         string
	Text              string
	Category          string
	Origin            FactOrigin
	Status            FactStatus
	Confidence        float64
	ValidFromEventID  string
	ValidUntilEventID string
	CreatedBy         string
	ConfirmedBy       string
	CreatedAt         time.Time
	UpdatedAt         time.Time
	DeletedAt         *time.Time
}

func NewFact(id, projectID, text, category string, origin FactOrigin, status FactStatus, confidence float64, validFromEventID, validUntilEventID string, now time.Time) (Fact, error) {
	id, projectID, text, category = strings.TrimSpace(id), strings.TrimSpace(projectID), strings.TrimSpace(text), strings.TrimSpace(category)
	if id == "" || projectID == "" || text == "" || category == "" {
		return Fact{}, ErrBlankValue
	}
	if confidence < 0 || confidence > 1 {
		return Fact{}, ErrInvalidConfidence
	}
	if !validOrigin(origin) || !validFactStatus(status) || (origin == OriginModel && (status == StatusAuthorConfirmed || status == StatusTextEvidenced)) {
		return Fact{}, ErrInvalidStatus
	}
	now = now.UTC()
	createdBy := string(origin)
	confirmedBy := ""
	if status == StatusAuthorConfirmed {
		confirmedBy = "author"
	}
	return Fact{ID: id, ProjectID: projectID, Text: text, Category: category, Origin: origin, Status: status, Confidence: confidence, ValidFromEventID: strings.TrimSpace(validFromEventID), ValidUntilEventID: strings.TrimSpace(validUntilEventID), CreatedBy: createdBy, ConfirmedBy: confirmedBy, CreatedAt: now, UpdatedAt: now}, nil
}

func (f Fact) Update(text, category string, status FactStatus, confidence float64, validFromEventID, validUntilEventID string, now time.Time) (Fact, error) {
	text, category = strings.TrimSpace(text), strings.TrimSpace(category)
	if f.ID == "" || f.ProjectID == "" || f.DeletedAt != nil || text == "" || category == "" {
		return Fact{}, ErrBlankValue
	}
	if confidence < 0 || confidence > 1 {
		return Fact{}, ErrInvalidConfidence
	}
	if !validFactStatus(status) {
		return Fact{}, ErrInvalidStatus
	}
	f.Text, f.Category, f.Status, f.Confidence = text, category, status, confidence
	f.ValidFromEventID, f.ValidUntilEventID = strings.TrimSpace(validFromEventID), strings.TrimSpace(validUntilEventID)
	if status != StatusAuthorConfirmed {
		f.ConfirmedBy = ""
	}
	f.UpdatedAt = now.UTC()
	return f, nil
}

func (f Fact) Confirm(authorID string, now time.Time) (Fact, error) {
	authorID = strings.TrimSpace(authorID)
	if f.ID == "" || f.ProjectID == "" || f.DeletedAt != nil || authorID == "" {
		return Fact{}, ErrBlankValue
	}
	f.Status, f.ConfirmedBy, f.Confidence, f.UpdatedAt = StatusAuthorConfirmed, authorID, 1, now.UTC()
	return f, nil
}

func validOrigin(origin FactOrigin) bool {
	return origin == OriginAuthor || origin == OriginText || origin == OriginModel || origin == OriginRule
}

func validFactStatus(status FactStatus) bool {
	switch status {
	case StatusAuthorConfirmed, StatusTextEvidenced, StatusAIInferred, StatusConflicted, StatusRejected:
		return true
	default:
		return false
	}
}

type Evidence struct {
	ID              string
	ProjectID       string
	FactID          string
	ChapterID       string
	Quote           string
	StartOffset     int
	EndOffset       int
	ChapterRevision int
	CreatedAt       time.Time
}

func NewEvidence(id, projectID, factID, chapterID, quote string, startOffset, endOffset, chapterRevision int, now time.Time) (Evidence, error) {
	id, projectID, factID, chapterID, quote = strings.TrimSpace(id), strings.TrimSpace(projectID), strings.TrimSpace(factID), strings.TrimSpace(chapterID), strings.TrimSpace(quote)
	if id == "" || projectID == "" || factID == "" || chapterID == "" || quote == "" {
		return Evidence{}, ErrBlankValue
	}
	if startOffset < 0 || endOffset <= startOffset || chapterRevision < 1 {
		return Evidence{}, ErrInvalidEvidence
	}
	return Evidence{ID: id, ProjectID: projectID, FactID: factID, ChapterID: chapterID, Quote: quote, StartOffset: startOffset, EndOffset: endOffset, ChapterRevision: chapterRevision, CreatedAt: now.UTC()}, nil
}

type ScopeType string

const (
	ScopeCharacter ScopeType = "character"
	ScopeFaction   ScopeType = "faction"
	ScopeLocation  ScopeType = "location"
	ScopeItem      ScopeType = "item"
	ScopeRegion    ScopeType = "region"
	ScopeRule      ScopeType = "rule"
	ScopeEvent     ScopeType = "event"
)

type Scope struct {
	ID         string
	ProjectID  string
	FactID     string
	TargetType ScopeType
	TargetID   string
	CreatedAt  time.Time
}

func NewScope(id, projectID, factID string, targetType ScopeType, targetID string, now time.Time) (Scope, error) {
	id, projectID, factID, targetID = strings.TrimSpace(id), strings.TrimSpace(projectID), strings.TrimSpace(factID), strings.TrimSpace(targetID)
	if id == "" || projectID == "" || factID == "" || targetID == "" {
		return Scope{}, ErrBlankValue
	}
	if !validScopeType(targetType) {
		return Scope{}, ErrInvalidReference
	}
	return Scope{ID: id, ProjectID: projectID, FactID: factID, TargetType: targetType, TargetID: targetID, CreatedAt: now.UTC()}, nil
}

func validScopeType(scopeType ScopeType) bool {
	switch scopeType {
	case ScopeCharacter, ScopeFaction, ScopeLocation, ScopeItem, ScopeRegion, ScopeRule, ScopeEvent:
		return true
	default:
		return false
	}
}

type ConflictStatus string

const (
	ConflictOpen     ConflictStatus = "open"
	ConflictResolved ConflictStatus = "resolved"
)

type Conflict struct {
	ID          string
	ProjectID   string
	FactID      string
	OtherFactID string
	Reason      string
	Status      ConflictStatus
	CreatedAt   time.Time
	ResolvedAt  *time.Time
}

func NewConflict(id, projectID, factID, otherFactID, reason string, now time.Time) (Conflict, error) {
	id, projectID, factID, otherFactID = strings.TrimSpace(id), strings.TrimSpace(projectID), strings.TrimSpace(factID), strings.TrimSpace(otherFactID)
	if id == "" || projectID == "" || factID == "" || otherFactID == "" {
		return Conflict{}, ErrBlankValue
	}
	if factID == otherFactID {
		return Conflict{}, ErrInvalidReference
	}
	return Conflict{ID: id, ProjectID: projectID, FactID: factID, OtherFactID: otherFactID, Reason: strings.TrimSpace(reason), Status: ConflictOpen, CreatedAt: now.UTC()}, nil
}

func (c Conflict) Resolve(now time.Time) (Conflict, error) {
	if c.ID == "" || c.ProjectID == "" || c.FactID == "" || c.OtherFactID == "" {
		return Conflict{}, ErrInvalidReference
	}
	resolved := now.UTC()
	c.Status, c.ResolvedAt = ConflictResolved, &resolved
	return c, nil
}
