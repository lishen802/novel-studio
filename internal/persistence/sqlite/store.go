package sqlite

import (
	"database/sql"
	"time"
)

// Store 是 SQLite 持久化的统一实现，承载所有领域 store 方法。
// 测试钩子字段（importJobReadHook 等）仅用于在测试中注入确定性事件，
// 生产路径不读写。
type Store struct {
	db                              *sql.DB
	importJobReadHook               func()
	importApplyAfterMaxPositionHook func()
}

// Close 释放底层 SQLite 连接，重复调用安全。
func (s *Store) Close() error {
	if s.db == nil {
		return nil
	}
	return s.db.Close()
}

// scanner 抽象 *sql.Row 与 *sql.Rows 共享的 Scan 行为，便于复用扫描函数。
type scanner interface {
	Scan(dest ...any) error
}

// formatTime 将时间统一以 UTC RFC3339Nano 文本写入数据库。
func formatTime(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

// parseTime 解析数据库中的 RFC3339Nano 时间文本。
func parseTime(value string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, value)
}

func boolToInt(value bool) int {
	if value {
		return 1
	}
	return 0
}
