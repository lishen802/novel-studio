package storyasset

import "time"

// Anchor 是正式约束资产的可查询投影，作用域必须指向稳定目标。
type Anchor struct {
	ID        string
	AssetID   string
	ProjectID string
	Strength  string
	ScopeKind string
	ScopeID   string
	Statement string
	CreatedAt time.Time
}
