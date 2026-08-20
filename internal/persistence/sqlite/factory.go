package sqlite

import (
	"context"
	"fmt"

	"github.com/lishen802/novel-studio/internal/application/ports"
	projectdomain "github.com/lishen802/novel-studio/internal/domain/project"
)

// Factory 创建并打开项目数据库 Store，实现 ports.ProjectStoreFactory。
type Factory struct{}

func (Factory) Create(ctx context.Context, databasePath string, project projectdomain.Project) (ports.ProjectStore, error) {
	db, err := OpenDatabase(ctx, databasePath)
	if err != nil {
		return nil, err
	}
	store := &Store{db: db}

	_, err = db.ExecContext(ctx, `
		INSERT INTO projects (id, title, genre, summary, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`,
		project.ID,
		project.Title,
		project.Genre,
		project.Summary,
		formatTime(project.CreatedAt),
		formatTime(project.UpdatedAt),
	)
	if err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("create project record: %w", err)
	}
	return store, nil
}

func (Factory) Open(ctx context.Context, databasePath string) (ports.ProjectStore, error) {
	db, err := OpenExistingDatabase(ctx, databasePath)
	if err != nil {
		return nil, err
	}
	store := &Store{db: db}

	var count int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM projects").Scan(&count); err != nil {
		_ = store.Close()
		return nil, fmt.Errorf("count projects: %w", err)
	}
	if count != 1 {
		_ = store.Close()
		return nil, fmt.Errorf("%w: found %d", ErrProjectCount, count)
	}
	return store, nil
}

// RecoveryRequired 执行只读工作区预检中的迁移恢复探测：
// 仅 stat 固定恢复产物，不触发迁移恢复，也不打开可写连接。
func (Factory) RecoveryRequired(databasePath string) (bool, error) {
	return migrationRecoveryRequired(databasePath)
}

// OpenReadOnly 打开已迁移至当前 schema 的项目数据库，不做迁移或恢复。
// 返回的 Store 与可编辑 Store 共享读 API；SQLite mode=ro 与 query_only
// 会拒绝任何意外写入。
func (Factory) OpenReadOnly(ctx context.Context, databasePath string) (ports.ProjectStore, error) {
	db, err := OpenReadOnlyDatabase(ctx, databasePath)
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

// OpenReadOnlyImmutable 打开不可变的分支头快照。与 OpenReadOnly 分离：
// 活动项目仍需读取 live WAL，而分支头在只读比对期间不应产生 WAL/SHM 协调文件。
func (Factory) OpenReadOnlyImmutable(ctx context.Context, databasePath string) (ports.ProjectStore, error) {
	db, err := OpenReadOnlyImmutableDatabase(ctx, databasePath)
	if err != nil {
		return nil, err
	}
	return &Store{db: db}, nil
}

var _ ports.ProjectStoreFactory = Factory{}
