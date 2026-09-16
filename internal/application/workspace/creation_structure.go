package workspace

import (
	"context"

	creationapp "github.com/lishen802/novel-studio/internal/application/creation"
)

// SaveCreationStructure 增删候选中的实体并返回新的工作台投影，不写正式资产。
func (s *Service) SaveCreationStructure(ctx context.Context, request creationapp.StructureEditRequest) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.SaveStructure(ctx, runtime.projectID, request); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}
