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

// 跨功能验收实际 SQLite 写入：工作稿冲突、重新生成候选、采用、空稿保存及历史恢复不能相互吞稿。
func TestAuthoringFlowPreservesEveryVersion(t *testing.T) {
	store, project, original := createStoreWithChapter(t)
	defer store.Close()
	ctx := context.Background()
	now := time.Unix(500, 0)
	makeCandidate := func(id string, chapter projectdomain.Chapter) revisiondomain.ChangeSet {
		t.Helper()
		candidate, err := revisiondomain.NewChapterBodyChangeSet(id, id+"-op", project.ID, chapter.ID, "正文候选", "", "agent", chapter.Revision, chapter.Body, "AI 候选正文", "", now)
		if err != nil {
			t.Fatal(err)
		}
		if err = store.CreateChangeSet(ctx, candidate); err != nil {
			t.Fatal(err)
		}
		return candidate
	}
	old := makeCandidate("before-author-edit", original)
	if err := store.SaveDraft(ctx, original.ID, "作者自动保存稿", original.Revision); err != nil {
		t.Fatal(err)
	}
	if _, err := store.ApplyChangeSet(ctx, old.ID, now); !errors.Is(err, projectdomain.ErrUnsavedDraft) {
		t.Fatalf("未保护工作稿: %v", err)
	}
	body, _, _, found, err := store.Draft(ctx, original.ID)
	if err != nil || !found || body != "作者自动保存稿" {
		t.Fatalf("工作稿丢失: %q %v", body, err)
	}
	author, err := original.Commit(body, original.Revision, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.CommitChapter(ctx, author, original); err != nil {
		t.Fatal(err)
	}
	if _, err = store.ApplyChangeSet(ctx, old.ID, now); err == nil {
		t.Fatal("旧基线候选不能覆盖作者新版本")
	}
	fresh := makeCandidate("after-author-edit", author)
	if _, err = store.ApplyChangeSet(ctx, fresh.ID, now); err != nil {
		t.Fatal(err)
	}
	applied, err := store.Chapter(ctx, original.ID)
	if err != nil || applied.Body != "AI 候选正文" {
		t.Fatalf("采用候选: %#v %v", applied, err)
	}
	if err = store.SaveDraft(ctx, original.ID, "", applied.Revision); err != nil {
		t.Fatal(err)
	}
	empty, err := applied.Commit("", applied.Revision, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = store.CommitChapter(ctx, empty, applied); err != nil {
		t.Fatal(err)
	}
	restored, err := store.RestoreChapterRevision(ctx, original.ID, author.Revision, empty.Revision, now)
	if err != nil || restored.Body != author.Body || restored.Revision != empty.Revision+1 {
		t.Fatalf("恢复作者版本: %#v %v", restored, err)
	}
	history, err := store.ListChapterRevisions(ctx, original.ID)
	if err != nil {
		t.Fatal(err)
	}
	retained := map[int64]string{}
	for _, item := range history {
		retained[item.Revision] = item.Body
	}
	for version, want := range map[int64]string{original.Revision: original.Body, author.Revision: author.Body, applied.Revision: applied.Body, empty.Revision: "", restored.Revision: author.Body} {
		got, exists := retained[version]
		if !exists || got != want {
			t.Fatalf("第 %d 版丢失: %q, want %q", version, got, want)
		}
	}
}
