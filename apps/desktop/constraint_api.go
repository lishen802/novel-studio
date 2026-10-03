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
