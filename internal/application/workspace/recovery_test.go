package workspace

import (
	"context"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/license"
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
	"github.com/lishen802/novel-studio/internal/platform/projectfs"
)

func TestWorkspaceRecoversDraftAcrossCompleteServiceRestart(t *testing.T) {
	ctx := context.Background()
	parent := t.TempDir()
	first := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Unix(1_000, 0)},
		&sequenceIDs{ids: []string{"project-1", "chapter-1"}},
	)

	project, err := first.CreateProject(ctx, parent, "长夜织境", "东方奇幻", "")
	if err != nil {
		t.Fatal(err)
	}
	chapter, err := first.CreateChapter(ctx, "第一章")
	if err != nil {
		t.Fatal(err)
	}
	committed, err := first.CommitChapter(ctx, chapter.ID, "已提交正文 A", chapter.Revision)
	if err != nil {
		t.Fatal(err)
	}
	if committed.Revision != 2 {
		t.Fatalf("committed revision = %d, want 2", committed.Revision)
	}
	if err := first.SaveDraft(ctx, chapter.ID, "中断前草稿 B", committed.Revision); err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}

	second := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Unix(2_000, 0)},
		&sequenceIDs{},
	)
	defer second.Close()
	if _, err := second.OpenProject(ctx, project.Path); err != nil {
		t.Fatal(err)
	}

	recovered, err := second.OpenChapter(ctx, chapter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Body != "已提交正文 A" || recovered.Revision != 2 {
		t.Fatalf("committed chapter changed during recovery: %#v", recovered)
	}
	if !recovered.HasRecoveryDraft || recovered.RecoveryBody != "中断前草稿 B" {
		t.Fatalf("recovery draft = %#v", recovered)
	}

	accepted, err := second.CommitChapter(
		ctx,
		chapter.ID,
		recovered.RecoveryBody,
		recovered.Revision,
	)
	if err != nil {
		t.Fatal(err)
	}
	if accepted.Body != "中断前草稿 B" || accepted.Revision != 3 {
		t.Fatalf("accepted recovery = %#v", accepted)
	}

	afterAccept, err := second.OpenChapter(ctx, chapter.ID)
	if err != nil {
		t.Fatal(err)
	}
	if afterAccept.HasRecoveryDraft || afterAccept.RecoveryBody != "" {
		t.Fatalf("draft was not cleared after acceptance: %#v", afterAccept)
	}
}
