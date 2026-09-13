package sqlite

import (
	"context"
	"database/sql"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	"github.com/pressly/goose/v3"
	"io/fs"
	"path/filepath"
	"testing"
	"time"
)

// TestCandidateRevisionMigrationPreservesRunsAndReferences 验证真实 v33 升级后旧运行、工件与新任务均可读取。
func TestCandidateRevisionMigrationPreservesRunsAndReferences(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "project.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	migrationFS, err := fs.Sub(migrations, "migrations")
	if err != nil {
		t.Fatal(err)
	}
	provider, err := goose.NewProvider(goose.DialectSQLite3, db, migrationFS)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 33); err != nil {
		t.Fatal(err)
	}
	now := time.Unix(2000, 0).UTC()
	stamp := formatTime(now)
	if _, err := db.ExecContext(ctx, `INSERT INTO projects(id,title,created_at,updated_at) VALUES ('project','迁移测试',?,?)`, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO chapters(id,project_id,title,position,revision,created_at,updated_at) VALUES ('chapter','project','测试章',1,1,?,?)`, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	store := &Store{db: db}
	run := newR3PersistenceRun(t, "project", "chapter", 1, now)
	artifact := testAgentArtifact("kept-base", run.ID, "", agentdomain.ArtifactPlan, "保留的候选基线", now)
	if err := store.CreateAgentRunBundle(ctx, agentdomain.RunBundle{Run: run, Artifacts: []agentdomain.Artifact{artifact}}); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 34); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.AgentRun(ctx, run.ID)
	if err != nil || loaded.TaskKind != run.TaskKind || len(loaded.Steps) != len(run.Steps) {
		t.Fatalf("旧运行丢失: %v", err)
	}
	artifacts, err := store.ListAgentArtifacts(ctx, run.ID)
	if err != nil || len(artifacts) != 1 || artifacts[0].Content != artifact.Content {
		t.Fatalf("旧工件丢失: %v", err)
	}
	if _, err := db.ExecContext(ctx, `UPDATE agent_runs SET task_kind='revise_story_candidate' WHERE id=?`, run.ID); err != nil {
		t.Fatal(err)
	}
	loaded, err = store.AgentRun(ctx, run.ID)
	if err != nil || loaded.TaskKind != agentdomain.TaskReviseStoryCandidate {
		t.Fatalf("新任务无法读取: %v", err)
	}
	rows, err := db.QueryContext(ctx, "PRAGMA foreign_key_check")
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	if rows.Next() {
		t.Fatal("升级破坏外键")
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
}
