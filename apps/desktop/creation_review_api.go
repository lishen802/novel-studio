package main

import (
	"context"

	creationapp "github.com/lishen802/novel-studio/internal/application/creation"
	"github.com/lishen802/novel-studio/internal/application/workspace"
)

type creationReviewUseCases interface {
	SetCreationReview(context.Context, creationapp.ReviewRequest) (creationapp.WorkbenchView, error)
	AcceptAllCreationReview(context.Context, string) (creationapp.WorkbenchView, error)
	UseCreationCandidateVersion(context.Context, string) (creationapp.WorkbenchView, error)
	CompareCreationCandidateVersions(context.Context, string, string) (creationapp.CandidateCompareView, error)
	SaveCreationReviewCursor(context.Context, creationapp.ReviewCursorRequest) (creationapp.WorkbenchView, error)
}

// SetCreationReview 记录单个实体/分节的接受、排除或撤销决定，不写正式资产。
func (a *DesktopAPI) SetCreationReview(request creationapp.ReviewRequest) (creationapp.WorkbenchView, error) {
	service, ok := a.workspace.(creationReviewUseCases)
	if !ok {
		return creationapp.WorkbenchView{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(service.SetCreationReview(a.ctx, request))
}

// AcceptAllCreationReview 显式接受当前候选的全部审阅单位，未列举单位仍保持未审阅。
func (a *DesktopAPI) AcceptAllCreationReview(candidateID string) (creationapp.WorkbenchView, error) {
	service, ok := a.workspace.(creationReviewUseCases)
	if !ok {
		return creationapp.WorkbenchView{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(service.AcceptAllCreationReview(a.ctx, candidateID))
}

// UseCreationCandidateVersion 显式切换工作基线，比较或打开结果不会切换。
func (a *DesktopAPI) UseCreationCandidateVersion(candidateID string) (creationapp.WorkbenchView, error) {
	service, ok := a.workspace.(creationReviewUseCases)
	if !ok {
		return creationapp.WorkbenchView{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(service.UseCreationCandidateVersion(a.ctx, candidateID))
}

// CompareCreationCandidateVersions 只读比较同阶段版本差异。
func (a *DesktopAPI) CompareCreationCandidateVersions(stage, baseCandidateID string) (creationapp.CandidateCompareView, error) {
	service, ok := a.workspace.(creationReviewUseCases)
	if !ok {
		return creationapp.CandidateCompareView{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(service.CompareCreationCandidateVersions(a.ctx, stage, baseCandidateID))
}

// SaveCreationReviewCursor 保存作者审阅位置，供重启后恢复同一对象。
func (a *DesktopAPI) SaveCreationReviewCursor(request creationapp.ReviewCursorRequest) (creationapp.WorkbenchView, error) {
	service, ok := a.workspace.(creationReviewUseCases)
	if !ok {
		return creationapp.WorkbenchView{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(service.SaveCreationReviewCursor(a.ctx, request))
}

var _ creationReviewUseCases = (*workspace.Service)(nil)
