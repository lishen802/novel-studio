package main

import (
	"context"

	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

type formalOutlineRetireUseCases interface {
	PreviewFormalOutlineRetire(context.Context, string, string, string) (asset.FormalOutlineRetirePreview, error)
	ConfirmFormalOutlineRetire(context.Context, asset.FormalOutlineRetireCommand) (asset.FormalOutlineRetirePreview, error)
}

// PreviewFormalOutlineRetire 只传作者选择，项目分支由工作区读取。
func (a *DesktopAPI) PreviewFormalOutlineRetire(assetID, chapterID, reason string) (asset.FormalOutlineRetirePreview, error) {
	svc, ok := a.workspace.(formalOutlineRetireUseCases)
	if !ok {
		return asset.FormalOutlineRetirePreview{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(svc.PreviewFormalOutlineRetire(a.ctx, assetID, chapterID, reason))
}

// ConfirmFormalOutlineRetire 用冻结摘要确认停用边界。
func (a *DesktopAPI) ConfirmFormalOutlineRetire(command asset.FormalOutlineRetireCommand) (asset.FormalOutlineRetirePreview, error) {
	svc, ok := a.workspace.(formalOutlineRetireUseCases)
	if !ok {
		return asset.FormalOutlineRetirePreview{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(svc.ConfirmFormalOutlineRetire(a.ctx, command))
}
