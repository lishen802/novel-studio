package timeline

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrBlankValue       = errors.New("timeline value must not be blank")
	ErrInvalidOrder     = errors.New("invalid timeline order")
	ErrInvalidStatus    = errors.New("invalid timeline status")
	ErrInvalidReference = errors.New("invalid timeline reference")
	ErrReferenced       = errors.New("timeline entity is still referenced")
)

type EventStatus string

const (
	EventPlanned   EventStatus = "planned"
	EventOccurred  EventStatus = "occurred"
	EventUncertain EventStatus = "uncertain"
)

type StoryEvent struct {
	ID             string
	ProjectID      string
	Title          string
	Summary        string
	Cause          string
	Result         string
	StoryTime      string
	StoryOrder     int64
	NarrativeOrder int
	Status         EventStatus
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
}

func NewStoryEvent(id, projectID, title, storyTime string, storyOrder int64, narrativeOrder int, status EventStatus, now time.Time) (StoryEvent, error) {
	id, projectID, title = strings.TrimSpace(id), strings.TrimSpace(projectID), strings.TrimSpace(title)
	if id == "" || projectID == "" || title == "" {
		return StoryEvent{}, ErrBlankValue
	}
	if storyOrder < 0 || narrativeOrder < 1 {
		return StoryEvent{}, ErrInvalidOrder
	}
	if !validEventStatus(status) {
		return StoryEvent{}, ErrInvalidStatus
	}
	now = now.UTC()
	return StoryEvent{ID: id, ProjectID: projectID, Title: title, StoryTime: strings.TrimSpace(storyTime), StoryOrder: storyOrder, NarrativeOrder: narrativeOrder, Status: status, CreatedAt: now, UpdatedAt: now}, nil
}

func (e StoryEvent) Update(title, summary, cause, result, storyTime string, storyOrder int64, narrativeOrder int, status EventStatus, now time.Time) (StoryEvent, error) {
	title = strings.TrimSpace(title)
	if e.ID == "" || e.ProjectID == "" || e.DeletedAt != nil || title == "" {
		return StoryEvent{}, ErrBlankValue
	}
	if storyOrder < 0 || narrativeOrder < 1 {
		return StoryEvent{}, ErrInvalidOrder
	}
	if !validEventStatus(status) {
		return StoryEvent{}, ErrInvalidStatus
	}
	e.Title, e.Summary, e.Cause, e.Result = title, strings.TrimSpace(summary), strings.TrimSpace(cause), strings.TrimSpace(result)
	e.StoryTime, e.StoryOrder, e.NarrativeOrder, e.Status = strings.TrimSpace(storyTime), storyOrder, narrativeOrder, status
	e.UpdatedAt = now.UTC()
	return e, nil
}

func validEventStatus(status EventStatus) bool {
	return status == EventPlanned || status == EventOccurred || status == EventUncertain
}

type EventTargetType string

const (
	TargetChapter   EventTargetType = "chapter"
	TargetCharacter EventTargetType = "character"
	TargetFaction   EventTargetType = "faction"
	TargetLocation  EventTargetType = "location"
	TargetItem      EventTargetType = "item"
)

type EventLink struct {
	ProjectID  string
	EventID    string
	TargetType EventTargetType
	TargetID   string
	Role       string
	CreatedAt  time.Time
}

func NewEventLink(event StoryEvent, targetType EventTargetType, targetID, role string, now time.Time) (EventLink, error) {
	targetID = strings.TrimSpace(targetID)
	if event.ID == "" || event.ProjectID == "" || event.DeletedAt != nil || targetID == "" || !validTargetType(targetType) {
		return EventLink{}, ErrInvalidReference
	}
	return EventLink{ProjectID: event.ProjectID, EventID: event.ID, TargetType: targetType, TargetID: targetID, Role: strings.TrimSpace(role), CreatedAt: now.UTC()}, nil
}

func validTargetType(targetType EventTargetType) bool {
	switch targetType {
	case TargetChapter, TargetCharacter, TargetFaction, TargetLocation, TargetItem:
		return true
	default:
		return false
	}
}
