package storyasset

// FormalOutlineReorderRequest 只交换同一故事弧下相邻的两个正式近期章节目标。
type FormalOutlineReorderRequest struct {
	ProjectID     string `json:"projectId"`
	BranchID      string `json:"branchId"`
	FirstAssetID  string `json:"firstAssetId"`
	SecondAssetID string `json:"secondAssetId"`
	ChapterID     string `json:"chapterId"`
	Reason        string `json:"reason"`
}

// FormalOutlineReorderPreview 冻结节点身份、有效顺序、章序和摘要。
type FormalOutlineReorderPreview struct {
	Request        FormalOutlineReorderRequest `json:"request"`
	Digest         string                      `json:"digest"`
	FirstNodeID    string                      `json:"firstNodeId"`
	SecondNodeID   string                      `json:"secondNodeId"`
	FirstTitle     string                      `json:"firstTitle"`
	SecondTitle    string                      `json:"secondTitle"`
	ParentNodeID   string                      `json:"parentNodeId"`
	FirstPosition  int                         `json:"firstPosition"`
	SecondPosition int                         `json:"secondPosition"`
	Position       int                         `json:"position"`
	FirstRevision  int64                       `json:"firstRevision"`
	SecondRevision int64                       `json:"secondRevision"`
}

// FormalOutlineReorderCommand 以预览摘要和幂等键确认未来顺序。
type FormalOutlineReorderCommand struct {
	Request        FormalOutlineReorderRequest `json:"request"`
	Digest         string                      `json:"digest"`
	IdempotencyKey string                      `json:"idempotencyKey"`
}
