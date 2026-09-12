package workspace

import (
	"context"
	creationapp "github.com/lishen802/novel-studio/internal/application/creation"
)

// SaveCreationEntry 校验请求项目与当前工作区一致，保存后返回权威投影。
func (s *Service) SaveCreationEntry(ctx context.Context, request creationapp.SaveEntryRequest) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if request.ProjectID != runtime.projectID {
		return creationapp.WorkbenchView{}, creationapp.ErrStaleStore
	}
	if _, err := runtime.service.SaveEntry(ctx, request); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}

// UseCreationMaterial 在可写工作区内复用来源候选，不绕过项目切换保护。
func (s *Service) UseCreationMaterial(ctx context.Context, request creationapp.UseMaterialRequest) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if request.ProjectID != runtime.projectID {
		return creationapp.WorkbenchView{}, creationapp.ErrStaleStore
	}
	if _, err := runtime.service.UseMaterialCandidate(ctx, request); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}

// SaveCreationSample 在当前可写工作区内保存试写准备，不启动模型。
func (s *Service) SaveCreationSample(ctx context.Context, request creationapp.SaveSampleRequest) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if request.ProjectID != runtime.projectID {
		return creationapp.WorkbenchView{}, creationapp.ErrStaleStore
	}
	if _, err := runtime.service.SaveSample(ctx, request); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}
