package storyasset

import "errors"

var (
	ErrChangeConflict    = errors.New("constraint change preview is stale or conflicts with history")
	ErrChangeUnsupported = errors.New("constraint change intent or target is not supported")
)

// ConstraintView 保留原设定与最新版本，避免档案把未来语句冒充全书事实。
type ConstraintView struct {
	AssetID  string             `json:"assetId"`
	Kind     string             `json:"kind"`
	Name     string             `json:"name"`
	Original string             `json:"original"`
	Current  string             `json:"current"`
	Revision int64              `json:"revision"`
	Locked   bool               `json:"locked"`
	Changes  []ConstraintChange `json:"changes"`
}

type ConstraintChange struct {
	AssetID   string `json:"assetId"`
	Revision  int64  `json:"revision"`
	ChapterID string `json:"chapterId"`
	Position  int    `json:"position"`
	Text      string `json:"text"`
	Reason    string `json:"reason"`
}

type ConstraintRequest struct {
	ProjectID string `json:"projectId"`
	BranchID  string `json:"branchId"`
	AssetID   string `json:"assetId"`
	Intent    string `json:"intent"`
	ChapterID string `json:"chapterId"`
	Text      string `json:"text"`
	Reason    string `json:"reason"`
}

type ConstraintPreview struct {
	Request           ConstraintRequest `json:"request"`
	Digest            string            `json:"digest"`
	Before            string            `json:"before"`
	Revision          int64             `json:"revision"`
	Position          int               `json:"position"`
	ProtectedChapters int               `json:"protectedChapters"`
	FutureChapters    int               `json:"futureChapters"`
	CanApply          bool              `json:"canApply"`
	BlockedReason     string            `json:"blockedReason"`
}

type ConstraintCommand struct {
	Request        ConstraintRequest `json:"request"`
	Digest         string            `json:"digest"`
	IdempotencyKey string            `json:"idempotencyKey"`
}

// EffectiveConstraint 标明章节上下文使用的精确资产版本和原有作用域。
type EffectiveConstraint struct {
	AssetID   string `json:"assetId"`
	Kind      string `json:"kind"`
	TypedID   string `json:"typedId"`
	Revision  int64  `json:"revision"`
	ScopeKind string `json:"scopeKind"`
	ScopeID   string `json:"scopeId"`
	Text      string `json:"text"`
}
