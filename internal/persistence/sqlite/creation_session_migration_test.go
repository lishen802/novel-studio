package sqlite

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"path/filepath"
	"testing"
	"time"

	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
	"github.com/pressly/goose/v3"
)

// TestCreationSessionVersionMigrationPreservesLegacyState 从真实 v28 表迁移，保留内容、方向选择和未完成运行。
func TestCreationSessionVersionMigrationPreservesLegacyState(t *testing.T) {
	ctx := context.Background()
	db, err := sql.Open("sqlite", filepath.Join(t.TempDir(), "project.db"))
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
	if _, err := provider.UpTo(ctx, 28); err != nil {
		t.Fatal(err)
	}
	now := formatTime(time.Unix(100, 0))
	if _, err := db.ExecContext(ctx, `INSERT INTO projects(id, title, created_at, updated_at) VALUES ('project-legacy', '旧项目', ?, ?)`, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := db.ExecContext(ctx, `INSERT INTO creation_sessions (
		id, project_id, branch_id, stage, advance_mode, intent_text,
		selected_direction_candidate_id, selected_direction_card_id, kept_direction_ids_json,
		active_operation_id, active_run_id, blocking_reason, archive_digest, created_at, updated_at
	) VALUES ('session-legacy', 'project-legacy', 'main', 'proposal', 'auto_advance', '旧库作者意图',
		'candidate-legacy', 'card-legacy', '["card-kept"]', 'operation-legacy', 'run-legacy', 'cost_authorization', 'digest-legacy', ?, ?)`, now, now); err != nil {
		t.Fatal(err)
	}
	if _, err := provider.UpTo(ctx, 29); err != nil {
		t.Fatal(err)
	}
	store := &Store{db: db}
	loaded, err := store.CreationSession(ctx, "project-legacy")
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ID != "session-legacy" || loaded.ContentVersion != 1 || loaded.RunSlotVersion != 0 || loaded.IntentText != "旧库作者意图" || loaded.SelectedDirectionCardID != "card-legacy" || len(loaded.KeptDirectionIDs) != 1 || loaded.KeptDirectionIDs[0] != "card-kept" || loaded.ActiveRunID != "run-legacy" || loaded.ActiveOperationID != "operation-legacy" || loaded.ArchiveDigest != "digest-legacy" || loaded.BlockingReason != creationdomain.BlockCostAuthorization {
		t.Fatalf("迁移改变既有状态: %+v", loaded)
	}
	if err := store.SaveCreationSession(ctx, loaded); err != nil {
		t.Fatal(err)
	}
	if err := store.SaveCreationSession(ctx, loaded); !errors.Is(err, creationdomain.ErrSessionVersionConflict) {
		t.Fatalf("旧库迁移后 CAS 未生效: %v", err)
	}
}
