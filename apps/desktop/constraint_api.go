package main

import (
	"context"
	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

type constraintUseCases interface {
	Constraints(context.Context) ([]asset.ConstraintView, error)
	PreviewConstraintChange(context.Context, asset.ConstraintRequest) (asset.ConstraintPreview, error)
	ApplyConstraintChange(context.Context, asset.ConstraintCommand) (asset.ConstraintPreview, error)
}

// Constraints 返回正式约束的原文与生效记录。
func (a *DesktopAPI) Constraints() ([]asset.ConstraintView, error) {
	feature, ok := a.workspace.(constraintUseCases)
	if !ok {
		return nil, safeDesktopError(asset.ErrChangeUnsupported)
	}
	return safeResult(feature.Constraints(a.ctx))
}

// PreviewConstraintChange 只转发预览请求，业务校验由应用层执行。
func (a *DesktopAPI) PreviewConstraintChange(r asset.ConstraintRequest) (asset.ConstraintPreview, error) {
	feature, ok := a.workspace.(constraintUseCases)
	if !ok {
		return asset.ConstraintPreview{}, safeDesktopError(asset.ErrChangeUnsupported)
	}
	return safeResult(feature.PreviewConstraintChange(a.ctx, r))
}

// ApplyConstraintChange 转发作者已核对的摘要和幂等键。
func (a *DesktopAPI) ApplyConstraintChange(c asset.ConstraintCommand) (asset.ConstraintPreview, error) {
	feature, ok := a.workspace.(constraintUseCases)
	if !ok {
		return asset.ConstraintPreview{}, safeDesktopError(asset.ErrChangeUnsupported)
	}
	return safeResult(feature.ApplyConstraintChange(a.ctx, c))
}

// CharacterRevisions 转发正式人物档案的版本投影。
func (a *DesktopAPI) CharacterRevisions() ([]asset.ConstraintView, error) {
	reader, ok := a.workspace.(interface {
		CharacterRevisions(context.Context) ([]asset.ConstraintView, error)
	})
	if !ok {
		return nil, safeDesktopError(asset.ErrChangeUnsupported)
	}
	return safeResult(reader.CharacterRevisions(a.ctx))
}

// PlanningRevisions 转发正式方案与路线图的版本投影。
func (a *DesktopAPI) PlanningRevisions() ([]asset.ConstraintView, error) {
	reader, ok := a.workspace.(interface {
		PlanningRevisions(context.Context) ([]asset.ConstraintView, error)
	})
	if !ok {
		return nil, safeDesktopError(asset.ErrChangeUnsupported)
	}
	return safeResult(reader.PlanningRevisions(a.ctx))
}
