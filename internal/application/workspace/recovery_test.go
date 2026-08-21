package workspace

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/license"
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
	"github.com/lishen802/novel-studio/internal/platform/projectfs"
	"github.com/lishen802/novel-studio/internal/platform/snapshotfs"
	"github.com/lishen802/novel-studio/internal/testsupport/legacyproject"
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

// TestWorkspaceOpensLegacyProjectAndPreservesDataAfterMigration verifies that
// a v9 .zjproject fixture is opened through the safe staged migration
// coordinator, its business sentinels are preserved, and the recovery
// directory plus backup are created. Reopening the migrated project must not
// run migration or create a second backup.
func TestWorkspaceOpensLegacyProjectAndPreservesDataAfterMigration(t *testing.T) {
	ctx := context.Background()
	fixture := legacyproject.Build(t, t.TempDir())

	service := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Unix(5_000, 0)},
		&sequenceIDs{},
		snapshotfs.Archive{},
	)

	view, err := service.OpenProject(ctx, fixture.PackagePath)
	if err != nil {
		t.Fatalf("OpenProject legacy: %v", err)
	}
	defer service.Close()

	if view.ID != fixture.Expected.Project.ID {
		t.Fatalf("project id = %q, want %q", view.ID, fixture.Expected.Project.ID)
	}
	if view.Title != fixture.Expected.Project.Title {
		t.Fatalf("project title = %q, want %q", view.Title, fixture.Expected.Project.Title)
	}
	if view.Genre != fixture.Expected.Project.Genre {
		t.Fatalf("project genre = %q, want %q", view.Genre, fixture.Expected.Project.Genre)
	}

	// Chapter body and revision preserved after migration.
	chapters, err := service.ListChapters(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(chapters) != 1 || chapters[0].ID != fixture.Expected.Chapter.ID {
		t.Fatalf("chapters = %#v", chapters)
	}
	chapter := chapters[0]
	if chapter.Body != fixture.Expected.Chapter.Body {
		t.Fatalf("chapter body = %q, want %q", chapter.Body, fixture.Expected.Chapter.Body)
	}
	if chapter.Revision != int64(fixture.Expected.Chapter.Revision) {
		t.Fatalf("chapter revision = %d, want %d", chapter.Revision, fixture.Expected.Chapter.Revision)
	}

	// Character data preserved.
	world, err := service.StoryWorld(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(world.Characters) != 1 || world.Characters[0].ID != fixture.Expected.Character.ID {
		t.Fatalf("characters = %#v", world.Characters)
	}
	if world.Characters[0].Name != fixture.Expected.Character.Name {
		t.Fatalf("character name = %q, want %q", world.Characters[0].Name, fixture.Expected.Character.Name)
	}

	// imports/ and recovery/ directories exist for compatibility.
	importsDir := filepath.Join(fixture.PackagePath, "imports")
	if info, err := os.Stat(importsDir); err != nil || !info.IsDir() {
		t.Fatalf("imports directory missing: %v", err)
	}
	recoveryDir := filepath.Join(fixture.PackagePath, "recovery")
	if info, err := os.Stat(recoveryDir); err != nil || !info.IsDir() {
		t.Fatalf("recovery directory missing: %v", err)
	}

	// Backup created and is independently openable at v9.
	backupPath := filepath.Join(recoveryDir, "migration-v09-v20.sqlite")
	if _, err := os.Stat(backupPath); err != nil {
		t.Fatalf("backup missing: %v", err)
	}
	backupHash := fileSHA256(t, backupPath)

	// No stage, original, or journal left behind.
	for _, name := range []string{"migration-stage.sqlite", "migration-original.sqlite", "migration.json"} {
		if _, err := os.Stat(filepath.Join(recoveryDir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("recovery artifact %q should not exist: %v", name, err)
		}
	}

	if err := service.Close(); err != nil {
		t.Fatal(err)
	}

	// Reopen: must not create a second backup or re-run migration.
	reopened := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Unix(6_000, 0)},
		&sequenceIDs{},
		snapshotfs.Archive{},
	)
	defer reopened.Close()
	if _, err := reopened.OpenProject(ctx, fixture.PackagePath); err != nil {
		t.Fatalf("reopen migrated project: %v", err)
	}

	if got := fileSHA256(t, backupPath); got != backupHash {
		t.Fatalf("backup changed on reopen")
	}
	for _, name := range []string{"migration-stage.sqlite", "migration-original.sqlite", "migration.json"} {
		if _, err := os.Stat(filepath.Join(recoveryDir, name)); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("recovery artifact %q should not exist after reopen: %v", name, err)
		}
	}
}

// TestWorkspaceKeepsCurrentProjectWhenLegacyOpenFailsWithDamagedJournal
// verifies that failing to open a legacy project with a damaged migration
// journal does not replace the currently active project.
func TestWorkspaceKeepsCurrentProjectWhenLegacyOpenFailsWithDamagedJournal(t *testing.T) {
	ctx := context.Background()
	parent := t.TempDir()

	service := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Unix(7_000, 0)},
		&sequenceIDs{ids: []string{"current-project", "current-chapter"}},
		snapshotfs.Archive{},
	)
	defer service.Close()

	// Open a current project that stays active.
	current, err := service.CreateProject(ctx, parent, "当前项目", "奇幻", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateChapter(ctx, "当前章节"); err != nil {
		t.Fatal(err)
	}

	// Build a legacy fixture and damage its recovery journal.
	fixture := legacyproject.Build(t, t.TempDir())
	recoveryDir := filepath.Join(fixture.PackagePath, "recovery")
	if err := os.MkdirAll(recoveryDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(recoveryDir, "migration.json"), []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}

	// Opening the damaged legacy project must fail with a recovery error.
	_, err = service.OpenProject(ctx, fixture.PackagePath)
	if !errors.Is(err, sqlite.ErrDatabaseMigrationRecovery) {
		t.Fatalf("error = %v, want ErrDatabaseMigrationRecovery", err)
	}

	// The current project is still active and readable.
	chapters, err := service.ListChapters(ctx)
	if err != nil {
		t.Fatalf("current project not active after failed open: %v", err)
	}
	if len(chapters) != 1 || chapters[0].Title != "当前章节" {
		t.Fatalf("current chapters = %#v", chapters)
	}

	// The current project path is unchanged.
	currentPath := current.Path
	if _, err := os.Stat(filepath.Join(currentPath, "project.db")); err != nil {
		t.Fatalf("current project database missing: %v", err)
	}
}

// TestWorkspaceKeepsCurrentProjectWhenLegacyOpenFailsWithFutureSchema
// verifies that opening a database with a future schema does not replace the
// currently active project and returns ErrDatabaseTooNew.
func TestWorkspaceKeepsCurrentProjectWhenLegacyOpenFailsWithFutureSchema(t *testing.T) {
	ctx := context.Background()
	parent := t.TempDir()

	service := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Unix(8_000, 0)},
		&sequenceIDs{ids: []string{"current-project-2", "current-chapter-2"}},
		snapshotfs.Archive{},
	)
	defer service.Close()

	current, err := service.CreateProject(ctx, parent, "当前项目二", "奇幻", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateChapter(ctx, "当前章节二"); err != nil {
		t.Fatal(err)
	}

	// Create a future-schema database by inserting a v999 goose_db_version row.
	futureParent := t.TempDir()
	futureService := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Unix(8_100, 0)},
		&sequenceIDs{ids: []string{"future-project"}},
		snapshotfs.Archive{},
	)
	futureView, err := futureService.CreateProject(ctx, futureParent, "未来项目", "", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := futureService.Close(); err != nil {
		t.Fatal(err)
	}
	futureDB, err := sqlite.OpenDatabase(ctx, filepath.Join(futureView.Path, "project.db"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := futureDB.Exec(`INSERT INTO goose_db_version (version_id, is_applied, tstamp) VALUES (999, 1, CURRENT_TIMESTAMP)`); err != nil {
		t.Fatal(err)
	}
	if err := futureDB.Close(); err != nil {
		t.Fatal(err)
	}

	_, err = service.OpenProject(ctx, futureView.Path)
	if !errors.Is(err, sqlite.ErrDatabaseTooNew) {
		t.Fatalf("error = %v, want ErrDatabaseTooNew", err)
	}

	// The current project is still active.
	chapters, err := service.ListChapters(ctx)
	if err != nil {
		t.Fatalf("current project not active after future-schema open: %v", err)
	}
	if len(chapters) != 1 || chapters[0].Title != "当前章节二" {
		t.Fatalf("current chapters = %#v", chapters)
	}

	if _, err := os.Stat(filepath.Join(current.Path, "project.db")); err != nil {
		t.Fatalf("current project database missing: %v", err)
	}
}
