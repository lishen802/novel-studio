package main

import (
	"context"

	creationapp "github.com/lishen802/novel-studio/internal/application/creation"
	"github.com/lishen802/novel-studio/internal/application/workspace"
)

type creationDirectionUseCases interface {
	PreviewRegenerateDirection(context.Context, creationapp.DirectionActionRequest) (creationapp.DirectionActionView, error)
	ConfirmRegenerateDirection(context.Context, creationapp.DirectionActionCommand) (creationapp.DirectionActionView, error)
	PreviewCombineDirections(context.Context, creationapp.DirectionActionRequest) (creationapp.DirectionActionView, error)
	ConfirmCombineDirections(context.Context, creationapp.DirectionActionCommand) (creationapp.DirectionActionView, error)
	RejectDirectionAction(context.Context, string) (creationapp.WorkbenchView, error)
}

// PreviewRegenerateDirection 返回单卡重做的冻结输入和费用预览。
func (a *DesktopAPI) PreviewRegenerateDirection(request creationapp.DirectionActionRequest) (creationapp.DirectionActionView, error) {
	service, ok := a.workspace.(creationDirectionUseCases)
	if !ok {
		return creationapp.DirectionActionView{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(service.PreviewRegenerateDirection(a.ctx, request))
}

// ConfirmRegenerateDirection 启动已经确认的单卡重做。
func (a *DesktopAPI) ConfirmRegenerateDirection(command creationapp.DirectionActionCommand) (creationapp.DirectionActionView, error) {
	service, ok := a.workspace.(creationDirectionUseCases)
	if !ok {
		return creationapp.DirectionActionView{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(service.ConfirmRegenerateDirection(a.ctx, command))
}

// PreviewCombineDirections 返回已保留方向组合的冻结输入和费用预览。
func (a *DesktopAPI) PreviewCombineDirections(request creationapp.DirectionActionRequest) (creationapp.DirectionActionView, error) {
	service, ok := a.workspace.(creationDirectionUseCases)
	if !ok {
		return creationapp.DirectionActionView{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(service.PreviewCombineDirections(a.ctx, request))
}

// ConfirmCombineDirections 启动已经确认的方向组合。
func (a *DesktopAPI) ConfirmCombineDirections(command creationapp.DirectionActionCommand) (creationapp.DirectionActionView, error) {
	service, ok := a.workspace.(creationDirectionUseCases)
	if !ok {
		return creationapp.DirectionActionView{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(service.ConfirmCombineDirections(a.ctx, command))
}

// RejectDirectionAction 放弃待确认的方向操作。
func (a *DesktopAPI) RejectDirectionAction(actionID string) (creationapp.WorkbenchView, error) {
	service, ok := a.workspace.(creationDirectionUseCases)
	if !ok {
		return creationapp.WorkbenchView{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(service.RejectDirectionAction(a.ctx, actionID))
}

var _ creationDirectionUseCases = (*workspace.Service)(nil)
