package storyasset

// FormalOutlineAppendRequest 为正式故事弧追加未来近期章节目标。
type FormalOutlineAppendRequest struct {
	ProjectID     string `json:"projectId"`
	BranchID      string `json:"branchId"`
	ParentAssetID string `json:"parentAssetId"`
	ChapterID     string `json:"chapterId"`
	Title         string `json:"title"`
	Summary       string `json:"summary"`
	Goal          string `json:"goal"`
	Conflict      string `json:"conflict"`
	Outcome       string `json:"outcome"`
	Reason        string `json:"reason"`
}

// FormalOutlineAppendPreview 冻结父节点、章节边界与将新增的节点位置。
type FormalOutlineAppendPreview struct {
	Request      FormalOutlineAppendRequest `json:"request"`
	Digest       string                     `json:"digest"`
	Position     int                        `json:"position"`
	NodePosition int                        `json:"nodePosition"`
	ParentTitle  string                     `json:"parentTitle"`
	AssetID      string                     `json:"assetId,omitempty"`
	NodeID       string                     `json:"nodeId,omitempty"`
}

// FormalOutlineAppendCommand 只确认预览摘要，不信任客户端提供的新节点位置。
type FormalOutlineAppendCommand struct {
	Request        FormalOutlineAppendRequest `json:"request"`
	Digest         string                     `json:"digest"`
	IdempotencyKey string                     `json:"idempotencyKey"`
}
