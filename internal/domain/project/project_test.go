package project

import (
	"errors"
	"testing"
	"time"
)

func TestNewProjectRejectsBlankTitle(t *testing.T) {
	_, err := NewProject("project-1", "  ", "奇幻", "简介", time.Unix(100, 0))

	if !errors.Is(err, ErrBlankTitle) {
		t.Fatalf("error = %v, want ErrBlankTitle", err)
	}
}

func TestNewProjectRejectsBlankID(t *testing.T) {
	_, err := NewProject(" ", "长夜", "奇幻", "简介", time.Unix(100, 0))

	if !errors.Is(err, ErrBlankID) {
		t.Fatalf("error = %v, want ErrBlankID", err)
	}
}

func TestNewProjectNormalizesTextAndKeepsStableIdentity(t *testing.T) {
	now := time.Date(2026, time.July, 15, 8, 30, 0, 123, time.FixedZone("CST", 8*60*60))

	got, err := NewProject(
		"project-1",
		"  长夜织境  ",
		"  东方奇幻 ",
		" 一名作者与智能体共同追索失落故事。 ",
		now,
	)
	if err != nil {
		t.Fatal(err)
	}

	if got.ID != "project-1" {
		t.Fatalf("ID = %q, want project-1", got.ID)
	}
	if got.Title != "长夜织境" {
		t.Fatalf("Title = %q, want 长夜织境", got.Title)
	}
	if got.Genre != "东方奇幻" {
		t.Fatalf("Genre = %q, want 东方奇幻", got.Genre)
	}
	if got.Summary != "一名作者与智能体共同追索失落故事。" {
		t.Fatalf("Summary = %q", got.Summary)
	}
	if !got.CreatedAt.Equal(now.UTC()) || !got.UpdatedAt.Equal(now.UTC()) {
		t.Fatalf("timestamps = (%v, %v), want %v", got.CreatedAt, got.UpdatedAt, now.UTC())
	}
}
