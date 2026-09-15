package main

import (
	"context"
	creationapp "github.com/lishen802/novel-studio/internal/application/creation"
)

type creationRevisionUseCases interface {
	PreviewCreationRevision(context.Context, creationapp.RevisionRequest) (creationapp.RevisionView, error)
	ConfirmCreationRevision(context.Context, creationapp.RevisionCommand) (creationapp.RevisionView, error)
	RejectCreationRevision(context.Context, string) (creationapp.WorkbenchView, error)
	ApplyCreationRevision(context.Context, creationapp.RevisionCommand) (creationapp.WorkbenchView, error)
}

type creationConversationUseCases interface {
	SendCreationMessage(context.Context, creationapp.SendMessageRequest) (creationapp.RevisionView, error)
}

type creationDiscussionUseCases interface {
	PreviewCreationDiscussionAuthorization(context.Context, creationapp.DiscussionAuthorizationRequest) (creationapp.DiscussionTurnView, error)
	ConfirmCreationDiscussionAuthorization(context.Context, creationapp.DiscussionAuthorizationCommand) (creationapp.DiscussionTurnView, error)
	SendCreationDiscussionMessage(context.Context, creationapp.DiscussionMessageRequest) (creationapp.DiscussionTurnView, error)
	RevokeCreationDiscussionAuthorization(context.Context, string) (creationapp.WorkbenchView, error)
	PreviewCreationDiscussionSuggestion(context.Context, creationapp.DiscussionSuggestionRequest) (creationapp.RevisionView, error)
}

// SendCreationMessage 保存绑定当前对象的公开消息，并形成待确认修改提案。
func (a *DesktopAPI) SendCreationMessage(request creationapp.SendMessageRequest) (creationapp.RevisionView, error) {
	service, ok := a.workspace.(creationConversationUseCases)
	if !ok {
		return creationapp.RevisionView{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(service.SendCreationMessage(a.ctx, request))
}

// PreviewCreationRevision 转发修改提案的预览与持久化命令。
func (a *DesktopAPI) PreviewCreationRevision(request creationapp.RevisionRequest) (creationapp.RevisionView, error) {
	service, ok := a.workspace.(creationRevisionUseCases)
	if !ok {
		return creationapp.RevisionView{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(service.PreviewCreationRevision(a.ctx, request))
}

// ConfirmCreationRevision 转发确认参数，不在绑定层执行模型调用。
func (a *DesktopAPI) ConfirmCreationRevision(command creationapp.RevisionCommand) (creationapp.RevisionView, error) {
	service, ok := a.workspace.(creationRevisionUseCases)
	if !ok {
		return creationapp.RevisionView{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(service.ConfirmCreationRevision(a.ctx, command))
}

// RejectCreationRevision 拒绝尚未执行的提案。
func (a *DesktopAPI) RejectCreationRevision(actionID string) (creationapp.WorkbenchView, error) {
	service, ok := a.workspace.(creationRevisionUseCases)
	if !ok {
		return creationapp.WorkbenchView{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(service.RejectCreationRevision(a.ctx, actionID))
}

// ApplyCreationRevision 转发作者明确采用，不自动确认档案。
func (a *DesktopAPI) ApplyCreationRevision(command creationapp.RevisionCommand) (creationapp.WorkbenchView, error) {
	service, ok := a.workspace.(creationRevisionUseCases)
	if !ok {
		return creationapp.WorkbenchView{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(service.ApplyCreationRevision(a.ctx, command))
}

// PreviewCreationDiscussionAuthorization 转发讨论授权预览，不调用模型。
func (a *DesktopAPI) PreviewCreationDiscussionAuthorization(request creationapp.DiscussionAuthorizationRequest) (creationapp.DiscussionTurnView, error) {
	service, ok := a.workspace.(creationDiscussionUseCases)
	if !ok {
		return creationapp.DiscussionTurnView{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(service.PreviewCreationDiscussionAuthorization(a.ctx, request))
}

// ConfirmCreationDiscussionAuthorization 转发授权确认与首轮讨论。
func (a *DesktopAPI) ConfirmCreationDiscussionAuthorization(command creationapp.DiscussionAuthorizationCommand) (creationapp.DiscussionTurnView, error) {
	service, ok := a.workspace.(creationDiscussionUseCases)
	if !ok {
		return creationapp.DiscussionTurnView{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(service.ConfirmCreationDiscussionAuthorization(a.ctx, command))
}

// SendCreationDiscussionMessage 转发已授权的后续讨论。
func (a *DesktopAPI) SendCreationDiscussionMessage(request creationapp.DiscussionMessageRequest) (creationapp.DiscussionTurnView, error) {
	service, ok := a.workspace.(creationDiscussionUseCases)
	if !ok {
		return creationapp.DiscussionTurnView{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(service.SendCreationDiscussionMessage(a.ctx, request))
}

// RevokeCreationDiscussionAuthorization 转发撤销讨论授权。
func (a *DesktopAPI) RevokeCreationDiscussionAuthorization(authorizationID string) (creationapp.WorkbenchView, error) {
	service, ok := a.workspace.(creationDiscussionUseCases)
	if !ok {
		return creationapp.WorkbenchView{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(service.RevokeCreationDiscussionAuthorization(a.ctx, authorizationID))
}

// PreviewCreationDiscussionSuggestion 转发讨论建议到修订预览。
func (a *DesktopAPI) PreviewCreationDiscussionSuggestion(request creationapp.DiscussionSuggestionRequest) (creationapp.RevisionView, error) {
	service, ok := a.workspace.(creationDiscussionUseCases)
	if !ok {
		return creationapp.RevisionView{}, safeDesktopError(errFeatureNotAvailable)
	}
	return safeResult(service.PreviewCreationDiscussionSuggestion(a.ctx, request))
}
