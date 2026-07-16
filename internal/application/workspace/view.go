package workspace

import (
	"time"

	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
)

type ProjectView struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Genre   string `json:"genre"`
	Summary string `json:"summary"`
	Path    string `json:"path"`
}

type ChapterView struct {
	ID                string `json:"id"`
	ProjectID         string `json:"projectId"`
	Title             string `json:"title"`
	Position          int    `json:"position"`
	Body              string `json:"body"`
	Revision          int64  `json:"revision"`
	UpdatedAt         string `json:"updatedAt"`
	RecoveryBody      string `json:"recoveryBody,omitempty"`
	RecoveryUpdatedAt string `json:"recoveryUpdatedAt,omitempty"`
	HasRecoveryDraft  bool   `json:"hasRecoveryDraft"`
}

func projectView(project projectdomain.Project, path string) ProjectView {
	return ProjectView{
		ID:      project.ID,
		Title:   project.Title,
		Genre:   project.Genre,
		Summary: project.Summary,
		Path:    path,
	}
}

func chapterView(chapter projectdomain.Chapter) ChapterView {
	return ChapterView{
		ID:        chapter.ID,
		ProjectID: chapter.ProjectID,
		Title:     chapter.Title,
		Position:  chapter.Position,
		Body:      chapter.Body,
		Revision:  chapter.Revision,
		UpdatedAt: chapter.UpdatedAt.UTC().Format(time.RFC3339Nano),
	}
}
