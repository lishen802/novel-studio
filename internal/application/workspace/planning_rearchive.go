package workspace

import (
	"context"

	asset "github.com/lishen802/novel-studio/internal/domain/storyasset"
)

// PreviewPlanningRearchive 从活动分支读取已审阅候选，返回按章生效的整组差异。
func (s *Service) PreviewPlanningRearchive(ctx context.Context, chapterID, reason string) (asset.PlanningRearchivePreview, error) {
	runtime, err := s.creationRuntime(ctx, false)
	if err != nil {
		return asset.PlanningRearchivePreview{}, err
	}
	s.mu.RLock()
	branchID, err := s.constraintBranchLocked()
	s.mu.RUnlock()
	if err != nil {
		return asset.PlanningRearchivePreview{}, err
	}
	return runtime.service.PreviewPlanningRearchive(ctx, asset.PlanningRearchiveRequest{ProjectID: runtime.projectID, BranchID: branchID, ChapterID: chapterID, Reason: reason})
}

// ConfirmPlanningRearchive 仅确认相同摘要，活动分支和正式 head 在事务内复核。
func (s *Service) ConfirmPlanningRearchive(ctx context.Context, command asset.PlanningRearchiveCommand) (asset.PlanningRearchivePreview, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return asset.PlanningRearchivePreview{}, err
	}
	if command.Request.ProjectID != runtime.projectID {
		return asset.PlanningRearchivePreview{}, asset.ErrChangeConflict
	}
	return runtime.service.ConfirmPlanningRearchive(ctx, command)
}
