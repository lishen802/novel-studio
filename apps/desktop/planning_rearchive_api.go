package main

import (
	"context"

	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

type planningRearchiveUseCases interface {
	PreviewPlanningRearchive(context.Context, string, string) (asset.PlanningRearchivePreview, error)
	ConfirmPlanningRearchive(context.Context, asset.PlanningRearchiveCommand) (asset.PlanningRearchivePreview, error)
}

// PreviewPlanningRearchive 将候选和边界交给工作区校验，不由前端提交差异载荷。
func (a *DesktopAPI) PreviewPlanningRearchive(chapterID, reason string) (asset.PlanningRearchivePreview, error) {
	svc, ok := a.workspace.(planningRearchiveUseCases)
	if !ok {
		return asset.PlanningRearchivePreview{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(svc.PreviewPlanningRearchive(a.ctx, chapterID, reason))
}

// ConfirmPlanningRearchive 使用冻结摘要和幂等键原子追加规划版本。
func (a *DesktopAPI) ConfirmPlanningRearchive(command asset.PlanningRearchiveCommand) (asset.PlanningRearchivePreview, error) {
	svc, ok := a.workspace.(planningRearchiveUseCases)
	if !ok {
		return asset.PlanningRearchivePreview{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(svc.ConfirmPlanningRearchive(a.ctx, command))
}
