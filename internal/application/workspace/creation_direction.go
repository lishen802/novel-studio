package workspace

import (
	"context"

	creationapp "github.com/lishen802/novel-studio/internal/application/creation"
)

// PreviewRegenerateDirection 冻结单卡重做输入和费用范围。
func (s *Service) PreviewRegenerateDirection(ctx context.Context, request creationapp.DirectionActionRequest) (creationapp.DirectionActionView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.DirectionActionView{}, err
	}
	profile, err := s.creationProfile(ctx)
	if err != nil {
		return creationapp.DirectionActionView{}, err
	}
	return runtime.service.PreviewRegenerateDirection(ctx, runtime.projectID, request, profile)
}

// ConfirmRegenerateDirection 确认后异步启动单卡重做。
func (s *Service) ConfirmRegenerateDirection(ctx context.Context, command creationapp.DirectionActionCommand) (creationapp.DirectionActionView, error) {
	return s.confirmDirectionAction(ctx, command)
}

// PreviewCombineDirections 冻结已保留来源卡和组合要求。
func (s *Service) PreviewCombineDirections(ctx context.Context, request creationapp.DirectionActionRequest) (creationapp.DirectionActionView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.DirectionActionView{}, err
	}
	profile, err := s.creationProfile(ctx)
	if err != nil {
		return creationapp.DirectionActionView{}, err
	}
	return runtime.service.PreviewCombineDirections(ctx, runtime.projectID, request, profile)
}

// ConfirmCombineDirections 确认后异步启动方向组合。
func (s *Service) ConfirmCombineDirections(ctx context.Context, command creationapp.DirectionActionCommand) (creationapp.DirectionActionView, error) {
	return s.confirmDirectionAction(ctx, command)
}

// RejectDirectionAction 放弃尚未确认的方向操作并刷新工作台。
func (s *Service) RejectDirectionAction(ctx context.Context, actionID string) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.RejectAction(ctx, runtime.projectID, actionID); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return s.creationWorkbench(ctx, false)
}

func (s *Service) confirmDirectionAction(ctx context.Context, command creationapp.DirectionActionCommand) (creationapp.DirectionActionView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.DirectionActionView{}, err
	}
	profile, err := s.creationProfile(ctx)
	if err != nil {
		return creationapp.DirectionActionView{}, err
	}
	return runtime.service.ConfirmDirectionAction(ctx, runtime.projectID, command, profile)
}
