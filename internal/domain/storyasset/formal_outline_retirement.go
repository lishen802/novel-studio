package storyasset

// FormalOutlineRetireRequest 选择正式近期章节点及未来停用边界。
type FormalOutlineRetireRequest struct {
	ProjectID           string   `json:"projectId"`
	BranchID            string   `json:"branchId"`
	AssetID             string   `json:"assetId"`
	ChapterID           string   `json:"chapterId"`
	Reason              string   `json:"reason"`
	UnlinkStoryLineID   string   `json:"unlinkStoryLineId,omitempty"`
	UnlinkStoryLineIDs  []string `json:"unlinkStoryLineIds,omitempty"`
	RebindStoryLineID   string   `json:"rebindStoryLineId,omitempty"`
	RebindTargetAssetID string   `json:"rebindTargetAssetId,omitempty"`
}

// FormalStoryLineUnlink 保存确认时解除的故事线链接原始位置和时间。
type FormalStoryLineUnlink struct {
	StoryLineID string `json:"storyLineId"`
	Name        string `json:"name"`
	Position    int    `json:"position"`
	CreatedAt   string `json:"createdAt"`
}

// FormalStoryLineRebind 保存原故事线链接与新章节目标的冻结身份。
type FormalStoryLineRebind struct {
	FormalStoryLineUnlink
	TargetAssetID      string `json:"targetAssetId"`
	TargetNodeID       string `json:"targetNodeId"`
	TargetTitle        string `json:"targetTitle"`
	TargetRevision     int    `json:"targetRevision"`
	TargetParentID     string `json:"targetParentId"`
	TargetIntroducedAt int    `json:"targetIntroducedAt"`
}

// FormalOutlineRetirePreview 是停用操作的冻结摘要与受影响节点说明。
type FormalOutlineRetirePreview struct {
	Request            FormalOutlineRetireRequest `json:"request"`
	Digest             string                     `json:"digest"`
	NodeID             string                     `json:"nodeId"`
	Title              string                     `json:"title"`
	IntroducedAt       int                        `json:"introducedAt"`
	Position           int                        `json:"position"`
	UnlinkedStoryLine  *FormalStoryLineUnlink     `json:"unlinkedStoryLine,omitempty"`
	UnlinkedStoryLines []FormalStoryLineUnlink    `json:"unlinkedStoryLines,omitempty"`
	ReboundStoryLine   *FormalStoryLineRebind     `json:"reboundStoryLine,omitempty"`
}

// FormalOutlineRetireCommand 由作者确认摘要和幂等键。
type FormalOutlineRetireCommand struct {
	Request        FormalOutlineRetireRequest `json:"request"`
	Digest         string                     `json:"digest"`
	IdempotencyKey string                     `json:"idempotencyKey"`
}

// FormalOutlineReference 记录阻止未来停用的确定性结构引用。
type FormalOutlineReference struct {
	Kind  string `json:"kind"`
	ID    string `json:"id"`
	Label string `json:"label"`
}

// FormalOutlineReferenceView 返回节点身份及当前直接引用，不代表停用授权。
type FormalOutlineReferenceView struct {
	AssetID    string                   `json:"assetId"`
	NodeID     string                   `json:"nodeId"`
	Title      string                   `json:"title"`
	References []FormalOutlineReference `json:"references"`
}
