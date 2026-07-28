package workspace

import (
	"context"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/license"
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
	"github.com/lishen802/novel-studio/internal/platform/projectfs"
	"github.com/lishen802/novel-studio/internal/platform/snapshotfs"
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
		snapshotfs.Archive{},
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
		snapshotfs.Archive{},
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

func TestWorkspaceRestoresWholeProjectSnapshotAndKeepsRecoveryPoint(t *testing.T) {
	ctx := context.Background()
	parent := t.TempDir()
	service := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Unix(3_000, 0)},
		&sequenceIDs{ids: []string{
			"project-1",
			"chapter-1",
			"character-1",
			"snapshot-1",
			"snapshot-recovery",
		}},
		snapshotfs.Archive{},
	)
	defer service.Close()

	if _, err := service.CreateProject(ctx, parent, "长夜织境", "东方奇幻", ""); err != nil {
		t.Fatal(err)
	}
	chapter, err := service.CreateChapter(ctx, "第一章")
	if err != nil {
		t.Fatal(err)
	}
	atSnapshot, err := service.CommitChapter(ctx, chapter.ID, "快照正文", chapter.Revision)
	if err != nil {
		t.Fatal(err)
	}
	character, err := service.CreateCharacter(ctx, CharacterInput{
		Name:    "墨老",
		Aliases: []string{},
		Role:    "supporting",
		Status:  "active",
	})
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := service.CreateSnapshot(ctx, "第一幕定稿")
	if err != nil {
		t.Fatal(err)
	}

	if _, err := service.CommitChapter(ctx, chapter.ID, "恢复前的新正文", atSnapshot.Revision); err != nil {
		t.Fatal(err)
	}
	if err := service.ArchiveCharacter(ctx, character.ID); err != nil {
		t.Fatal(err)
	}

	restored, err := service.RestoreSnapshot(ctx, snapshot.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(restored.Chapters) != 1 || restored.Chapters[0].Body != "快照正文" {
		t.Fatalf("restored chapters = %#v", restored.Chapters)
	}
	world, err := service.StoryWorld(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(world.Characters) != 1 || world.Characters[0].ID != character.ID {
		t.Fatalf("restored characters = %#v", world.Characters)
	}
	snapshots, err := service.Snapshots(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshots) != 2 {
		t.Fatalf("snapshots after restore = %#v", snapshots)
	}
	if snapshots[0].Kind != "recovery" && snapshots[1].Kind != "recovery" {
		t.Fatalf("pre-restore recovery point missing: %#v", snapshots)
	}
}
