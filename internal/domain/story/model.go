package story

import (
	"errors"
	"strings"
	"time"
)

var (
	ErrBlankValue       = errors.New("story value must not be blank")
	ErrInvalidHierarchy = errors.New("invalid outline hierarchy")
	ErrInvalidReference = errors.New("invalid story reference")
	ErrInvalidPosition  = errors.New("story position must be positive")
	ErrReferenced       = errors.New("story entity is still referenced")
)

type OutlineLevel string

const (
	LevelBook    OutlineLevel = "book"
	LevelVolume  OutlineLevel = "volume"
	LevelArc     OutlineLevel = "arc"
	LevelChapter OutlineLevel = "chapter"
	LevelScene   OutlineLevel = "scene"
)

var outlineLevelOrder = map[OutlineLevel]int{
	LevelBook: 0, LevelVolume: 1, LevelArc: 2, LevelChapter: 3, LevelScene: 4,
}

type OutlineNode struct {
	ID        string
	ProjectID string
	ParentID  string
	Level     OutlineLevel
	Title     string
	Summary   string
	Goal      string
	Conflict  string
	Outcome   string
	Position  int
	ChapterID string
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func NewOutlineNode(id, projectID string, parent *OutlineNode, level OutlineLevel, title string, position int, now time.Time) (OutlineNode, error) {
	id, projectID, title = strings.TrimSpace(id), strings.TrimSpace(projectID), strings.TrimSpace(title)
	if id == "" || projectID == "" || title == "" {
		return OutlineNode{}, ErrBlankValue
	}
	if position < 1 {
		return OutlineNode{}, ErrInvalidPosition
	}
	order, valid := outlineLevelOrder[level]
	if !valid {
		return OutlineNode{}, ErrInvalidHierarchy
	}
	parentID := ""
	if parent == nil {
		if order != 0 {
			return OutlineNode{}, ErrInvalidHierarchy
		}
	} else {
		if parent.ProjectID != projectID {
			return OutlineNode{}, ErrInvalidReference
		}
		parentOrder, parentValid := outlineLevelOrder[parent.Level]
		if !parentValid || parentOrder+1 != order || parent.DeletedAt != nil {
			return OutlineNode{}, ErrInvalidHierarchy
		}
		parentID = parent.ID
	}
	now = now.UTC()
	return OutlineNode{ID: id, ProjectID: projectID, ParentID: parentID, Level: level, Title: title, Position: position, CreatedAt: now, UpdatedAt: now}, nil
}

func (n OutlineNode) UpdateContent(
	title string,
	summary string,
	goal string,
	conflict string,
	outcome string,
	now time.Time,
) (OutlineNode, error) {
	title = strings.TrimSpace(title)
	if n.ID == "" || n.ProjectID == "" || n.DeletedAt != nil || title == "" {
		return OutlineNode{}, ErrBlankValue
	}
	n.Title = title
	n.Summary = strings.TrimSpace(summary)
	n.Goal = strings.TrimSpace(goal)
	n.Conflict = strings.TrimSpace(conflict)
	n.Outcome = strings.TrimSpace(outcome)
	n.UpdatedAt = now.UTC()
	return n, nil
}

type StoryLineStatus string

const (
	StoryLinePlanned StoryLineStatus = "planned"
	StoryLineActive  StoryLineStatus = "active"
	StoryLineClosed  StoryLineStatus = "closed"
)

type StoryLine struct {
	ID        string
	ProjectID string
	Name      string
	Summary   string
	Status    StoryLineStatus
	CreatedAt time.Time
	UpdatedAt time.Time
	DeletedAt *time.Time
}

func NewStoryLine(id, projectID, name, summary string, now time.Time) (StoryLine, error) {
	id, projectID, name = strings.TrimSpace(id), strings.TrimSpace(projectID), strings.TrimSpace(name)
	if id == "" || projectID == "" || name == "" {
		return StoryLine{}, ErrBlankValue
	}
	now = now.UTC()
	return StoryLine{ID: id, ProjectID: projectID, Name: name, Summary: strings.TrimSpace(summary), Status: StoryLinePlanned, CreatedAt: now, UpdatedAt: now}, nil
}

type StoryLineLink struct {
	StoryLineID   string
	OutlineNodeID string
	Position      int
	CreatedAt     time.Time
}

func NewStoryLineLink(line StoryLine, outlineNodeID string, position int, now time.Time) (StoryLineLink, error) {
	outlineNodeID = strings.TrimSpace(outlineNodeID)
	if line.ID == "" || line.DeletedAt != nil || outlineNodeID == "" {
		return StoryLineLink{}, ErrInvalidReference
	}
	if position < 1 {
		return StoryLineLink{}, ErrInvalidPosition
	}
	return StoryLineLink{StoryLineID: line.ID, OutlineNodeID: outlineNodeID, Position: position, CreatedAt: now.UTC()}, nil
}

type SceneStatus string

const (
	ScenePlanned SceneStatus = "planned"
	SceneDrafted SceneStatus = "drafted"
	SceneFinal   SceneStatus = "final"
)

type SceneCard struct {
	ID            string
	ProjectID     string
	ChapterID     string
	OutlineNodeID string
	Title         string
	POV           string
	Location      string
	Goal          string
	Conflict      string
	Outcome       string
	Position      int
	Status        SceneStatus
	CreatedAt     time.Time
	UpdatedAt     time.Time
	DeletedAt     *time.Time
}

func NewSceneCard(id, projectID, chapterID, outlineNodeID, title string, position int, now time.Time) (SceneCard, error) {
	id, projectID = strings.TrimSpace(id), strings.TrimSpace(projectID)
	chapterID, outlineNodeID, title = strings.TrimSpace(chapterID), strings.TrimSpace(outlineNodeID), strings.TrimSpace(title)
	if id == "" || projectID == "" || title == "" {
		return SceneCard{}, ErrBlankValue
	}
	if chapterID == "" || outlineNodeID == "" {
		return SceneCard{}, ErrInvalidReference
	}
	if position < 1 {
		return SceneCard{}, ErrInvalidPosition
	}
	now = now.UTC()
	return SceneCard{ID: id, ProjectID: projectID, ChapterID: chapterID, OutlineNodeID: outlineNodeID, Title: title, Position: position, Status: ScenePlanned, CreatedAt: now, UpdatedAt: now}, nil
}
