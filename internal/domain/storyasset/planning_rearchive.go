package storyasset

// PlanningArchiveAsset 保留首次入档和最新版本的完整载荷，用于稳定身份核验。
type PlanningArchiveAsset struct {
	AssetID           string `json:"assetId"`
	Kind              string `json:"kind"`
	TypedID           string `json:"typedId"`
	SourceCandidateID string `json:"sourceCandidateId"`
	Original          string `json:"original"`
	Current           string `json:"current"`
	Revision          int64  `json:"revision"`
}

// PlanningRearchiveRequest 绑定已入档候选与一个未来空章节。
type PlanningRearchiveRequest struct {
	ProjectID string `json:"projectId"`
	BranchID  string `json:"branchId"`
	ChapterID string `json:"chapterId"`
	Reason    string `json:"reason"`
}

// PlanningRearchiveChange 展示一项正式资产即将发生的版本变化。
type PlanningRearchiveChange struct {
	AssetID string            `json:"assetId"`
	Kind    string            `json:"kind"`
	Before  string            `json:"before"`
	After   string            `json:"after"`
	Request ConstraintRequest `json:"request"`
	Digest  string            `json:"digest"`
}

// PlanningRearchivePreview 是整组确认回执的冻结输入。
type PlanningRearchivePreview struct {
	Request           PlanningRearchiveRequest  `json:"request"`
	Digest            string                    `json:"digest"`
	Changes           []PlanningRearchiveChange `json:"changes"`
	Position          int                       `json:"position"`
	ProtectedChapters int                       `json:"protectedChapters"`
	FutureChapters    int                       `json:"futureChapters"`
}

// PlanningRearchiveCommand 只携带作者确认过的摘要和幂等键。
type PlanningRearchiveCommand struct {
	Request        PlanningRearchiveRequest `json:"request"`
	Digest         string                   `json:"digest"`
	IdempotencyKey string                   `json:"idempotencyKey"`
}

// PlanningRearchiveGuard 将通过审阅的候选精确绑定到确认事务。
type PlanningRearchiveGuard struct {
	CandidateID   string `json:"candidateId"`
	ContentSHA256 string `json:"contentSha256"`
	ReviewVersion int64  `json:"reviewVersion"`
}

// PlanningRearchiveApply 由应用层计算，仓储在短事务中逐项复验。
type PlanningRearchiveApply struct {
	Preview PlanningRearchivePreview `json:"preview"`
	Assets  []PlanningArchiveAsset   `json:"assets"`
	Guards  []PlanningRearchiveGuard `json:"guards"`
	Key     string                   `json:"key"`
}
