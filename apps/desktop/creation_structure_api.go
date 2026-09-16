package main

import (
	"context"

	creationapp "github.com/lishen802/novel-studio/internal/application/creation"
	"github.com/lishen802/novel-studio/internal/application/workspace"
)

type creationStructureUseCases interface {
	SaveCreationStructure(context.Context, creationapp.StructureEditRequest) (creationapp.WorkbenchView, error)
}

// SaveCreationStructure 转发作者的结构增删命令，不自动接受阶段或写入正式资产。
func (a *DesktopAPI) SaveCreationStructure(request creationapp.StructureEditRequest) (creationapp.WorkbenchView, error) {
	service, ok := a.workspace.(creationStructureUseCases)
	if !ok {
		return creationapp.WorkbenchView{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(service.SaveCreationStructure(a.ctx, request))
}

var _ creationStructureUseCases = (*workspace.Service)(nil)
