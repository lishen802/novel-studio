package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"github.com/pressly/goose/v3"
	"io/fs"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

func TestCreationEntryRevisionCASAndHistory(t *testing.T) {
	ctx := context.Background()
	store, project := createTestStore(t)
	defer store.Close()
	now := time.Unix(700, 0).UTC()
	session, _ := creationdomain.NewSession("entry-session", project.ID, "main", now)
	session, err := store.CreateCreationSession(ctx, session)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := creationdomain.NewEntryRevision("entry-1", session.ID, "", 1, creationdomain.EntryAuthorPlan, "原文😀\n不能裁剪。", now)
	if err := store.AppendCreationEntry(ctx, session, first); err != nil {
		t.Fatal(err)
	}
	stale := session
	session, err = store.CreationSession(ctx, project.ID)
	if err != nil || session.ContentVersion != stale.ContentVersion+1 {
		t.Fatalf("内容 CAS: %+v %v", session, err)
	}
	second, _ := first.Revise("entry-2", creationdomain.EntryIdea, first.Material, now.Add(time.Second))
	if err := store.AppendCreationEntry(ctx, stale, second); !errors.Is(err, creationdomain.ErrSessionVersionConflict) {
		t.Fatalf("过期页面: %v", err)
	}
	if _, err := store.CreationEntry(ctx, session.ID, second.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("失败写入遗留版本: %v", err)
	}
	if err := store.AppendCreationEntry(ctx, session, second); err != nil {
		t.Fatal(err)
	}
	var sequence int
	var databaseName, databasePath string
	if err := store.db.QueryRowContext(ctx, "PRAGMA database_list").Scan(&sequence, &databaseName, &databasePath); err != nil {
		t.Fatal(err)
	}
	reopened, err := sql.Open("sqlite", databasePath)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	restarted := &Store{db: reopened}
	head, err := restarted.CurrentCreationEntry(ctx, session.ID)
	if err != nil || head.ID != second.ID {
		t.Fatalf("当前起点: %+v %v", head, err)
	}
	old, err := store.CreationEntry(ctx, session.ID, first.ID)
	if err != nil || old.Material != first.Material || old.Kind != creationdomain.EntryAuthorPlan {
		t.Fatalf("旧来源丢失: %+v %v", old, err)
	}
	if _, err := store.CreationEntry(ctx, "other-session", first.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("跨会话读取: %v", err)
	}
}

func TestCreationEntryFailureAndActiveRunDoNotChangeHead(t *testing.T) {
	ctx := context.Background()
	store, project := createTestStore(t)
	defer store.Close()
	now := time.Unix(701, 0).UTC()
	session, _ := creationdomain.NewSession("entry-failure", project.ID, "main", now)
	session, err := store.CreateCreationSession(ctx, session)
	if err != nil {
		t.Fatal(err)
	}
	first, _ := creationdomain.NewEntryRevision("entry-fail", session.ID, "", 1, creationdomain.EntryAuthorPlan, "完整原文", now)
	_, err = store.db.ExecContext(ctx, `CREATE TRIGGER fail_entry_head BEFORE INSERT ON creation_entry_heads BEGIN SELECT RAISE(ABORT,'injected'); END`)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AppendCreationEntry(ctx, session, first); err == nil {
		t.Fatal("预期故障")
	}
	if _, err := store.CreationEntry(ctx, session.ID, first.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("未回滚来源: %v", err)
	}
	loaded, _ := store.CreationSession(ctx, project.ID)
	if loaded.ContentVersion != session.ContentVersion {
		t.Fatal("未回滚会话版本")
	}
	if _, err := store.db.ExecContext(ctx, `DROP TRIGGER fail_entry_head`); err != nil {
		t.Fatal(err)
	}
	running, err := session.AttachRun("operation", "run", now)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCreationRunSlot(ctx, running); err != nil {
		t.Fatal(err)
	}
	if err := store.AppendCreationEntry(ctx, session, first); !errors.Is(err, creationdomain.ErrSessionVersionConflict) {
		t.Fatalf("不能绕过活动运行: %v", err)
	}
	loaded, _ = store.CreationSession(ctx, project.ID)
	if loaded.ActiveRunID != "run" || loaded.ContentVersion != session.ContentVersion {
		t.Fatal("活动运行被改动")
	}
}

func TestCreationEntryMigrationKeepsLegacySessionUnchanged(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "legacy.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	migrationFS, err := fs.Sub(migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrationFS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 31); err != nil {
		t.Fatal(err)
	}
	now := formatTime(time.Unix(702, 0))
	if _, err := db.ExecContext(ctx, `INSERT INTO projects(id,title,created_at,updated_at) VALUES('legacy-project','旧作品',?,?)`, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO creation_sessions(id,project_id,branch_id,stage,advance_mode,intent_text,created_at,updated_at)
 VALUES('legacy-session','legacy-project','main','proposal','step_review','作者原有意图',?,?)`, now, now); err != nil {
		t.Fatal(err)
	}
	store := &Store{db: db}
	before, err := store.CreationSession(ctx, "legacy-project")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 32); err != nil {
		t.Fatal(err)
	}
	after, err := store.CreationSession(ctx, "legacy-project")
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatalf("迁移改变会话: %v", err)
	}
	if _, err := store.CurrentCreationEntry(ctx, "legacy-session"); !errors.Is(err, sql.ErrNoRows) {
		t.Fatalf("迁移伪造作者来源: %v", err)
	}
}

func TestCreationEntryConcurrentWritersOnlyOneWins(t *testing.T) {
	ctx := context.Background()
	store, project := createTestStore(t)
	defer store.Close()
	now := time.Unix(703, 0).UTC()
	session, _ := creationdomain.NewSession("entry-race", project.ID, "main", now)
	session, err := store.CreateCreationSession(ctx, session)
	if err != nil {
		t.Fatal(err)
	}
	ready := make(chan struct{})
	results := make(chan error, 2)
	for _, id := range []string{"left", "right"} {
		entry, _ := creationdomain.NewEntryRevision(id, session.ID, "", 1, creationdomain.EntryAuthorPlan, id, now)
		go func() { <-ready; results <- store.AppendCreationEntry(ctx, session, entry) }()
	}
	close(ready)
	success := 0
	for i := 0; i < 2; i++ {
		if err := <-results; err == nil {
			success++
		} else if !errors.Is(err, creationdomain.ErrSessionVersionConflict) {
			t.Fatalf("unexpected writer failure: %v", err)
		}
	}
	if success != 1 {
		t.Fatalf("成功写入数=%d", success)
	}
	var count int
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM creation_entry_revisions WHERE session_id=?`, session.ID).Scan(&count); err != nil || count != 1 {
		t.Fatalf("并发留下重复来源: %d %v", count, err)
	}
}
