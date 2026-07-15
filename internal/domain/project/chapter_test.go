package project

import (
	"errors"
	"testing"
	"time"
)

func TestNewChapterStartsAtRevisionOne(t *testing.T) {
	now := time.Unix(100, 0)

	chapter, err := NewChapter("chapter-1", "project-1", "  第一章  ", 1, now)
	if err != nil {
		t.Fatal(err)
	}

	if chapter.Title != "第一章" {
		t.Fatalf("Title = %q, want 第一章", chapter.Title)
	}
	if chapter.Position != 1 {
		t.Fatalf("Position = %d, want 1", chapter.Position)
	}
	if chapter.Body != "" {
		t.Fatalf("Body = %q, want empty", chapter.Body)
	}
	if chapter.Revision != 1 {
		t.Fatalf("Revision = %d, want 1", chapter.Revision)
	}
	if !chapter.CreatedAt.Equal(now.UTC()) || !chapter.UpdatedAt.Equal(now.UTC()) {
		t.Fatalf("timestamps = (%v, %v), want %v", chapter.CreatedAt, chapter.UpdatedAt, now.UTC())
	}
}

func TestNewChapterRejectsBlankTitle(t *testing.T) {
	_, err := NewChapter("chapter-1", "project-1", " ", 1, time.Unix(100, 0))

	if !errors.Is(err, ErrBlankTitle) {
		t.Fatalf("error = %v, want ErrBlankTitle", err)
	}
}

func TestNewChapterRejectsInvalidIdentityAndPosition(t *testing.T) {
	tests := []struct {
		name      string
		chapterID string
		projectID string
		position  int
		want      error
	}{
		{name: "blank chapter ID", chapterID: " ", projectID: "project-1", position: 1, want: ErrBlankID},
		{name: "blank project ID", chapterID: "chapter-1", projectID: " ", position: 1, want: ErrBlankID},
		{name: "zero position", chapterID: "chapter-1", projectID: "project-1", position: 0, want: ErrInvalidPosition},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewChapter(tt.chapterID, tt.projectID, "第一章", tt.position, time.Unix(100, 0))
			if !errors.Is(err, tt.want) {
				t.Fatalf("error = %v, want %v", err, tt.want)
			}
		})
	}
}

func TestChapterCommitRequiresExpectedRevision(t *testing.T) {
	chapter, err := NewChapter("chapter-1", "project-1", "第一章", 1, time.Unix(100, 0))
	if err != nil {
		t.Fatal(err)
	}

	updated, err := chapter.Commit("第一版正文\n\n保留段落空白。", 1, time.Unix(101, 0))
	if err != nil {
		t.Fatal(err)
	}
	if updated.Body != "第一版正文\n\n保留段落空白。" {
		t.Fatalf("Body = %q", updated.Body)
	}
	if updated.Revision != 2 {
		t.Fatalf("revision = %d, want 2", updated.Revision)
	}
	if !updated.UpdatedAt.Equal(time.Unix(101, 0).UTC()) {
		t.Fatalf("UpdatedAt = %v", updated.UpdatedAt)
	}
	if !updated.CreatedAt.Equal(chapter.CreatedAt) {
		t.Fatalf("CreatedAt changed from %v to %v", chapter.CreatedAt, updated.CreatedAt)
	}

	_, err = updated.Commit("过期窗口覆盖", 1, time.Unix(102, 0))
	if !errors.Is(err, ErrRevisionConflict) {
		t.Fatalf("error = %v, want ErrRevisionConflict", err)
	}
}
