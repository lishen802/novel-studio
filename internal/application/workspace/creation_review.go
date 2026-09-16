package workspace

import (
	"context"

	creationapp "github.com/lishen802/novel-studio/internal/application/creation"
)

// SetCreationReview 记录作者对单个实体或分节的接受、排除与撤销决定，不写正式资产。
func (s *Service) SetCreationReview(ctx context.Context, request creationapp.ReviewRequest) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.SetReview(ctx, runtime.projectID, request); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}

// AcceptAllCreationReview 是显式“全部接受”命令，未列举单位仍保持未审阅。
func (s *Service) AcceptAllCreationReview(ctx context.Context, candidateID string) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.AcceptAllReview(ctx, runtime.projectID, candidateID); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}

// UseCreationCandidateVersion 显式切换该阶段的工作基线。
func (s *Service) UseCreationCandidateVersion(ctx context.Context, candidateID string) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.UseCandidateVersion(ctx, runtime.projectID, candidateID); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}

// CompareCreationCandidateVersions 只读比较版本，不切换工作基线。
func (s *Service) CompareCreationCandidateVersions(ctx context.Context, stage, baseCandidateID string) (creationapp.CandidateCompareView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.CandidateCompareView{}, err
	}
	return runtime.service.CompareCandidateVersions(ctx, runtime.projectID, stage, baseCandidateID)
}

// SaveCreationReviewCursor 保存作者审阅位置，不使生成输入或候选内容失效。
func (s *Service) SaveCreationReviewCursor(ctx context.Context, request creationapp.ReviewCursorRequest) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.SaveReviewCursor(ctx, runtime.projectID, request); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}
