package workspace

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
)

type ManuscriptExportView struct {
	Path         string `json:"path"`
	ChapterCount int    `json:"chapterCount"`
	DraftCount   int    `json:"draftCount"`
}

type manuscriptExporter interface {
	WriteManuscript(directory, extension, body string) (string, error)
}

// ExportManuscript 导出章节顺序下的当前工作稿，包含空白草稿，且不改变正文版本。
func (s *Service) ExportManuscript(ctx context.Context, format string) (ManuscriptExportView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.requireEditableLocked(); err != nil {
		return ManuscriptExportView{}, err
	}
	extension := "txt"
	switch format {
	case "plain":
	case "markdown":
		extension = "md"
	default:
		return ManuscriptExportView{}, errors.New("unsupported manuscript format")
	}
	exporter, ok := s.packages.(manuscriptExporter)
	if !ok {
		return ManuscriptExportView{}, errors.New("manuscript export unavailable")
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return ManuscriptExportView{}, err
	}
	chapters, err := s.active.store.ListChapters(ctx)
	if err != nil {
		return ManuscriptExportView{}, err
	}
	sort.SliceStable(chapters, func(i, j int) bool { return chapters[i].Position < chapters[j].Position })
	var text strings.Builder
	if format == "markdown" {
		text.WriteString("# ")
	}
	text.WriteString(strings.ReplaceAll(project.Title, "\n", " "))
	text.WriteString("\n\n")
	draftCount := 0
	for _, chapter := range chapters {
		body := chapter.Body
		draft, base, _, found, err := s.active.store.Draft(ctx, chapter.ID)
		if err != nil {
			return ManuscriptExportView{}, err
		}
		if found {
			if base != chapter.Revision {
				return ManuscriptExportView{}, projectdomain.ErrRevisionConflict
			}
			body = draft
			if draft != chapter.Body {
				draftCount++
			}
		}
		if format == "markdown" {
			text.WriteString("## ")
		}
		text.WriteString(strings.ReplaceAll(chapter.Title, "\n", " "))
		text.WriteString("\n\n")
		text.WriteString(body)
		text.WriteString("\n\n")
	}
	if err := ctx.Err(); err != nil {
		return ManuscriptExportView{}, err
	}
	path, err := exporter.WriteManuscript(s.active.packageInfo.Exports, extension, text.String())
	if err != nil {
		return ManuscriptExportView{}, fmt.Errorf("export manuscript: %w", s.handleFilesystemWriteFailureLocked(err))
	}
	return ManuscriptExportView{Path: path, ChapterCount: len(chapters), DraftCount: draftCount}, nil
}
