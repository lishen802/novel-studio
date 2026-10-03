package storyasset

// FormalOutlineRetireRequest 选择正式近期章节点及未来停用边界。
type FormalOutlineRetireRequest struct {
	ProjectID string `json:"projectId"`
	BranchID  string `json:"branchId"`
	AssetID   string `json:"assetId"`
	ChapterID string `json:"chapterId"`
	Reason    string `json:"reason"`
}

// FormalOutlineRetirePreview 是停用操作的冻结摘要与受影响节点说明。
type FormalOutlineRetirePreview struct {
	Request      FormalOutlineRetireRequest `json:"request"`
	Digest       string                     `json:"digest"`
	NodeID       string                     `json:"nodeId"`
	Title        string                     `json:"title"`
	IntroducedAt int                        `json:"introducedAt"`
	Position     int                        `json:"position"`
}

// FormalOutlineRetireCommand 由作者确认摘要和幂等键。
type FormalOutlineRetireCommand struct {
	Request        FormalOutlineRetireRequest `json:"request"`
	Digest         string                     `json:"digest"`
	IdempotencyKey string                     `json:"idempotencyKey"`
}
