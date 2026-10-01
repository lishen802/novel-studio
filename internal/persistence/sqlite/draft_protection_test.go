package sqlite

import (
	"context"
	"errors"
	"testing"
	"time"

	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
	revisiondomain "github.com/lishen802/novel-studio/internal/domain/revision"
)

func TestApplyCandidatePreservesAuthorDraft(t *testing.T) {
	for _, partial := range []bool{false, true} {
		t.Run(map[bool]string{false: "整组", true: "单项"}[partial], func(t *testing.T) {
			store, project, chapter := createStoreWithChapter(t)
			defer store.Close()
			ctx := context.Background()
			change, err := revisiondomain.NewChapterBodyChangeSet("change", "operation", project.ID, chapter.ID, "候选", "", "agent", chapter.Revision, chapter.Body, "模型正文", "", time.Unix(200, 0))
			if err != nil {
				t.Fatal(err)
			}
			if err = store.CreateChangeSet(ctx, change); err != nil {
				t.Fatal(err)
			}
			if err = store.SaveDraft(ctx, chapter.ID, "作者的新内容", chapter.Revision); err != nil {
				t.Fatal(err)
			}
			if partial {
				_, err = store.ApplyChangeOperation(ctx, change.ID, "operation", time.Unix(201, 0))
			} else {
				_, err = store.ApplyChangeSet(ctx, change.ID, time.Unix(201, 0))
			}
			if !errors.Is(err, projectdomain.ErrUnsavedDraft) {
				t.Fatalf("apply error = %v", err)
			}
			body, revision, _, found, err := store.Draft(ctx, chapter.ID)
			if err != nil || !found || body != "作者的新内容" || revision != chapter.Revision {
				t.Fatalf("draft = %q, %d, %v, %v", body, revision, found, err)
			}
			current, err := store.Chapter(ctx, chapter.ID)
			if err != nil || current.Body != chapter.Body || current.Revision != chapter.Revision {
				t.Fatalf("chapter mutated: %#v %v", current, err)
			}
			pending, err := store.ChangeSet(ctx, change.ID)
			if err != nil || string(pending.Status) != "pending" {
				t.Fatalf("candidate not retained: %#v %v", pending, err)
			}
		})
	}
}
func TestRestoreRevisionPreservesAuthorDraft(t *testing.T) {
	store, _, chapter := createStoreWithChapter(t)
	defer store.Close()
	ctx := context.Background()
	next, _ := chapter.Commit("已存版本", chapter.Revision, time.Unix(200, 0))
	if err := store.CommitChapter(ctx, next, chapter); err != nil {
		t.Fatal(err)
	}
	// 空白也是作者有意保留的工作稿。
	if err := store.SaveDraft(ctx, chapter.ID, "", next.Revision); err != nil {
		t.Fatal(err)
	}
	_, err := store.RestoreChapterRevision(ctx, chapter.ID, chapter.Revision, next.Revision, time.Unix(300, 0))
	if !errors.Is(err, projectdomain.ErrUnsavedDraft) {
		t.Fatalf("restore error = %v", err)
	}
	body, _, _, found, err := store.Draft(ctx, chapter.ID)
	if err != nil || !found || body != "" {
		t.Fatalf("draft lost: %q %v %v", body, found, err)
	}
}
