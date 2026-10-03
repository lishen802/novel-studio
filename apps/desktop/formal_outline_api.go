package main

import (
	"context"

	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

type formalOutlineUseCases interface {
	PreviewFormalOutlineAppend(context.Context, string, string, string, string, string, string, string, string) (asset.FormalOutlineAppendPreview, error)
	ConfirmFormalOutlineAppend(context.Context, asset.FormalOutlineAppendCommand) (asset.FormalOutlineAppendPreview, error)
}

// PreviewFormalOutlineAppend 只收集作者输入，项目及活动分支由工作区绑定。
func (a *DesktopAPI) PreviewFormalOutlineAppend(input asset.FormalOutlineAppendRequest) (asset.FormalOutlineAppendPreview, error) {
	svc, ok := a.workspace.(formalOutlineUseCases)
	if !ok {
		return asset.FormalOutlineAppendPreview{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(svc.PreviewFormalOutlineAppend(a.ctx, input.ParentAssetID, input.ChapterID, input.Title, input.Summary, input.Goal, input.Conflict, input.Outcome, input.Reason))
}

// ConfirmFormalOutlineAppend 将冻结摘要与幂等键交给事务复核。
func (a *DesktopAPI) ConfirmFormalOutlineAppend(command asset.FormalOutlineAppendCommand) (asset.FormalOutlineAppendPreview, error) {
	svc, ok := a.workspace.(formalOutlineUseCases)
	if !ok {
		return asset.FormalOutlineAppendPreview{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(svc.ConfirmFormalOutlineAppend(a.ctx, command))
}
