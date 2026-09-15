package workspace

import (
	"context"
	creationapp "github.com/lishen802/novel-studio/internal/application/creation"
)

// PreviewCreationRevision 保存带目标和预算的提案，不创建章节或调用模型。
func (s *Service) PreviewCreationRevision(ctx context.Context, request creationapp.RevisionRequest) (creationapp.RevisionView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.RevisionView{}, err
	}
	profile, err := s.creationProfile(ctx)
	if err != nil {
		return creationapp.RevisionView{}, err
	}
	return runtime.service.PreviewRevision(ctx, runtime.projectID, request, profile)
}

// SendCreationMessage 保存绑定当前策划对象的消息，并返回由该消息形成的修改提案。
func (s *Service) SendCreationMessage(ctx context.Context, request creationapp.SendMessageRequest) (creationapp.RevisionView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.RevisionView{}, err
	}
	profile, err := s.creationProfile(ctx)
	if err != nil {
		return creationapp.RevisionView{}, err
	}
	return runtime.service.RecordRevisionConversation(ctx, runtime.projectID, request, profile)
}

// ConfirmCreationRevision 异步启动已确认的项目级修订。
func (s *Service) ConfirmCreationRevision(ctx context.Context, command creationapp.RevisionCommand) (creationapp.RevisionView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.RevisionView{}, err
	}
	profile, err := s.creationProfile(ctx)
	if err != nil {
		return creationapp.RevisionView{}, err
	}
	return runtime.service.ConfirmRevision(ctx, runtime.projectID, command, profile)
}

// RejectCreationRevision 明确拒绝待确认提案，刷新后不再阻塞作者推进。
func (s *Service) RejectCreationRevision(ctx context.Context, actionID string) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.RejectAction(ctx, runtime.projectID, actionID); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return s.creationWorkbench(ctx, false)
}

// ApplyCreationRevision 只采用已经独立审阅并仍匹配基线的候选。
func (s *Service) ApplyCreationRevision(ctx context.Context, command creationapp.RevisionCommand) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.ApplyRevision(ctx, runtime.projectID, command); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return s.creationWorkbench(ctx, false)
}
