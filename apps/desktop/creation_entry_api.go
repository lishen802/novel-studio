package main

import (
	"context"
	creationapp "github.com/lishen802/novel-studio/internal/application/creation"
)

type creationEntryUseCases interface {
	SaveCreationEntry(context.Context, creationapp.SaveEntryRequest) (creationapp.WorkbenchView, error)
	UseCreationMaterial(context.Context, creationapp.UseMaterialRequest) (creationapp.WorkbenchView, error)
}

// SaveCreationEntry 转发带会话版本的本地材料保存命令。
func (a *DesktopAPI) SaveCreationEntry(request creationapp.SaveEntryRequest) (creationapp.WorkbenchView, error) {
	service, ok := a.workspace.(creationEntryUseCases)
	if !ok {
		return creationapp.WorkbenchView{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(service.SaveCreationEntry(a.ctx, request))
}

// UseCreationMaterial 显式复用已验证的作者材料候选。
func (a *DesktopAPI) UseCreationMaterial(request creationapp.UseMaterialRequest) (creationapp.WorkbenchView, error) {
	service, ok := a.workspace.(creationEntryUseCases)
	if !ok {
		return creationapp.WorkbenchView{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(service.UseCreationMaterial(a.ctx, request))
}
