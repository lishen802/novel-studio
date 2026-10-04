package main

import (
	"context"

	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

type formalOutlineReorderUseCases interface {
	PreviewFormalOutlineReorder(context.Context, string, string, string, string) (asset.FormalOutlineReorderPreview, error)
	ConfirmFormalOutlineReorder(context.Context, asset.FormalOutlineReorderCommand) (asset.FormalOutlineReorderPreview, error)
	FormalOutlinePositionsAtChapter(context.Context, string) (map[string]int, error)
}

// PreviewFormalOutlineReorder 仅传作者选择，活动分支由工作区核对。
func (a *DesktopAPI) PreviewFormalOutlineReorder(firstAssetID, secondAssetID, chapterID, reason string) (asset.FormalOutlineReorderPreview, error) {
	svc, ok := a.workspace.(formalOutlineReorderUseCases)
	if !ok {
		return asset.FormalOutlineReorderPreview{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(svc.PreviewFormalOutlineReorder(a.ctx, firstAssetID, secondAssetID, chapterID, reason))
}

// ConfirmFormalOutlineReorder 以冻结摘要确认未来章节顺序。
func (a *DesktopAPI) ConfirmFormalOutlineReorder(command asset.FormalOutlineReorderCommand) (asset.FormalOutlineReorderPreview, error) {
	svc, ok := a.workspace.(formalOutlineReorderUseCases)
	if !ok {
		return asset.FormalOutlineReorderPreview{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(svc.ConfirmFormalOutlineReorder(a.ctx, command))
}

// FormalOutlinePositionsAtChapter 返回该章实际生效的正式目标顺序。
func (a *DesktopAPI) FormalOutlinePositionsAtChapter(chapterID string) (map[string]int, error) {
	svc, ok := a.workspace.(formalOutlineReorderUseCases)
	if !ok {
		return nil, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(svc.FormalOutlinePositionsAtChapter(a.ctx, chapterID))
}
