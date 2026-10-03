package storyasset

// CharacterRevisionBase 冻结正式人物身份与最新版本，集合结构只作为不可修改的修订合同。
type CharacterRevisionBase struct {
	ProjectID     string `json:"projectId"`
	BranchID      string `json:"branchId"`
	AssetID       string `json:"assetId"`
	TypedID       string `json:"typedId"`
	EntityID      string `json:"entityId"`
	Revision      int64  `json:"revision"`
	Profile       string `json:"profile"`
	PayloadSHA256 string `json:"payloadSha256"`
	CastJSON      string `json:"castJson"`
	ContextJSON   string `json:"contextJson"`
	Digest        string `json:"digest"`
}

const FormalCharacterCandidatePrefix = "formal-revision:"
