package creation

import (
	"context"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

// formalCharacterReader 由修订用例消费最新正式快照，不依赖原类型行或候选选择状态。
type formalCharacterReader interface {
	FormalCharacterRevisionBase(context.Context, string, string, string) (asset.CharacterRevisionBase, error)
}

func allowedFormalCharacterField(field string) bool {
	switch field {
	case "name", "summary", "motivation", "fear", "secret", "arc":
		return true
	}
	return false
}

// FormalCharacterRevisionBase 委托正式快照读取；缺少后端能力时不伪造基线。
func (store sqliteStore) FormalCharacterRevisionBase(ctx context.Context, project, branch, id string) (asset.CharacterRevisionBase, error) {
	reader, ok := store.inner.(formalCharacterReader)
	if !ok {
		return asset.CharacterRevisionBase{}, ErrUnavailable
	}
	return reader.FormalCharacterRevisionBase(ctx, project, branch, id)
}

// FormalCharacterRequest 只接收作者选择的档案字段，集合路径由服务端按稳定来源实体构造。
type FormalCharacterRequest struct {
	AssetID          string `json:"assetId"`
	ExpectedRevision int64  `json:"expectedRevision"`
	Field            string `json:"field"`
	Instruction      string `json:"instruction"`
	IdempotencyKey   string `json:"idempotencyKey"`
}

// PreviewFormalCharacterRevision 将最新正式身份映射到统一修订协议，预览不付费也不写入候选。
func (service *Service) PreviewFormalCharacterRevision(ctx context.Context, project string, request FormalCharacterRequest, profile agentdomain.ResolvedCreativeProfile) (RevisionView, error) {
	if !allowedFormalCharacterField(request.Field) {
		return RevisionView{}, asset.ErrChangeUnsupported
	}
	session, err := service.loadSession(ctx, project)
	if err != nil {
		return RevisionView{}, err
	}
	reader, ok := service.store.(formalCharacterReader)
	if !ok {
		return RevisionView{}, ErrUnavailable
	}
	base, err := reader.FormalCharacterRevisionBase(ctx, project, session.BranchID, request.AssetID)
	if err != nil {
		return RevisionView{}, err
	}
	return service.PreviewRevision(ctx, project, RevisionRequest{FormalAssetID: request.AssetID, ExpectedRevision: request.ExpectedRevision, Paths: []string{"/characters/" + escapeField("@"+base.EntityID) + "/" + request.Field}, Instruction: request.Instruction, IdempotencyKey: request.IdempotencyKey}, profile)
}
