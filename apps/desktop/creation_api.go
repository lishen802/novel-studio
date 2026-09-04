package main

import (
	"context"

	creationapp "github.com/lishen802/novel-studio/internal/application/creation"
	"github.com/lishen802/novel-studio/internal/application/workspace"
)

type CreationUseCases interface {
	CreationWorkbench(ctx context.Context) (creationapp.WorkbenchView, error)
	SaveCreationIntent(ctx context.Context, text string) (creationapp.WorkbenchView, error)
	SetCreationAdvanceMode(ctx context.Context, mode string) (creationapp.WorkbenchView, error)
	SelectCreationDirection(ctx context.Context, candidateID string) (creationapp.WorkbenchView, error)
	AdvanceCreationStage(ctx context.Context) (creationapp.WorkbenchView, error)
	LookBackCreationStage(ctx context.Context, stage string) (creationapp.WorkbenchView, error)
	SaveCreationCandidateDraft(ctx context.Context, candidateID, payloadJSON string) (creationapp.WorkbenchView, error)
	LockCreationField(ctx context.Context, candidateID, path string) (creationapp.WorkbenchView, error)
	UnlockCreationField(ctx context.Context, candidateID, path string) (creationapp.WorkbenchView, error)
	GetCreationCandidate(ctx context.Context, candidateID string) (creationapp.CandidateView, error)
	PreviewCreationGenerate(ctx context.Context, stage string) (creationapp.CostPreview, error)
	ConfirmCreationGenerate(ctx context.Context, stage, digest, idempotencyKey string) (creationapp.GenerateResult, error)
	CancelCreationRun(ctx context.Context) error
	RecoverCreation(ctx context.Context) (creationapp.WorkbenchView, error)
	PreviewCreationArchive(ctx context.Context) (creationapp.ArchivePreview, error)
	ConfirmCreationArchive(ctx context.Context, digest, idempotencyKey string) (creationapp.ArchiveResult, error)
	ResolveCreationImpact(ctx context.Context, choice string) (creationapp.ResolveImpactResult, error)
}

type CreationGenerateInput struct {
	Stage              string `json:"stage"`
	ConfirmationDigest string `json:"confirmationDigest"`
	IdempotencyKey     string `json:"idempotencyKey"`
}

type CreationArchiveInput struct {
	ConfirmationDigest string `json:"confirmationDigest"`
	IdempotencyKey     string `json:"idempotencyKey"`
}

func (a *DesktopAPI) creationUseCases() (CreationUseCases, error) {
	if svc, ok := a.workspace.(CreationUseCases); ok {
		return svc, nil
	}
	return nil, errFeatureNotAvailable
}

func (a *DesktopAPI) CreationWorkbench() (creationapp.WorkbenchView, error) {
	svc, err := a.creationUseCases()
	if err != nil {
		return creationapp.WorkbenchView{}, safeDesktopError(err)
	}
	return safeResult(svc.CreationWorkbench(a.ctx))
}

func (a *DesktopAPI) SaveCreationIntent(text string) (creationapp.WorkbenchView, error) {
	svc, err := a.creationUseCases()
	if err != nil {
		return creationapp.WorkbenchView{}, safeDesktopError(err)
	}
	return safeResult(svc.SaveCreationIntent(a.ctx, text))
}

func (a *DesktopAPI) SetCreationAdvanceMode(mode string) (creationapp.WorkbenchView, error) {
	svc, err := a.creationUseCases()
	if err != nil {
		return creationapp.WorkbenchView{}, safeDesktopError(err)
	}
	return safeResult(svc.SetCreationAdvanceMode(a.ctx, mode))
}

func (a *DesktopAPI) SelectCreationDirection(candidateID string) (creationapp.WorkbenchView, error) {
	svc, err := a.creationUseCases()
	if err != nil {
		return creationapp.WorkbenchView{}, safeDesktopError(err)
	}
	return safeResult(svc.SelectCreationDirection(a.ctx, candidateID))
}

func (a *DesktopAPI) AdvanceCreationStage() (creationapp.WorkbenchView, error) {
	svc, err := a.creationUseCases()
	if err != nil {
		return creationapp.WorkbenchView{}, safeDesktopError(err)
	}
	return safeResult(svc.AdvanceCreationStage(a.ctx))
}

func (a *DesktopAPI) LookBackCreationStage(stage string) (creationapp.WorkbenchView, error) {
	svc, err := a.creationUseCases()
	if err != nil {
		return creationapp.WorkbenchView{}, safeDesktopError(err)
	}
	return safeResult(svc.LookBackCreationStage(a.ctx, stage))
}

func (a *DesktopAPI) SaveCreationCandidateDraft(candidateID, payloadJSON string) (creationapp.WorkbenchView, error) {
	svc, err := a.creationUseCases()
	if err != nil {
		return creationapp.WorkbenchView{}, safeDesktopError(err)
	}
	return safeResult(svc.SaveCreationCandidateDraft(a.ctx, candidateID, payloadJSON))
}

func (a *DesktopAPI) LockCreationField(candidateID, path string) (creationapp.WorkbenchView, error) {
	svc, err := a.creationUseCases()
	if err != nil {
		return creationapp.WorkbenchView{}, safeDesktopError(err)
	}
	return safeResult(svc.LockCreationField(a.ctx, candidateID, path))
}

func (a *DesktopAPI) UnlockCreationField(candidateID, path string) (creationapp.WorkbenchView, error) {
	svc, err := a.creationUseCases()
	if err != nil {
		return creationapp.WorkbenchView{}, safeDesktopError(err)
	}
	return safeResult(svc.UnlockCreationField(a.ctx, candidateID, path))
}

func (a *DesktopAPI) GetCreationCandidate(candidateID string) (creationapp.CandidateView, error) {
	svc, err := a.creationUseCases()
	if err != nil {
		return creationapp.CandidateView{}, safeDesktopError(err)
	}
	return safeResult(svc.GetCreationCandidate(a.ctx, candidateID))
}

func (a *DesktopAPI) PreviewCreationGenerate(stage string) (creationapp.CostPreview, error) {
	svc, err := a.creationUseCases()
	if err != nil {
		return creationapp.CostPreview{}, safeDesktopError(err)
	}
	return safeResult(svc.PreviewCreationGenerate(a.ctx, stage))
}

func (a *DesktopAPI) ConfirmCreationGenerate(input CreationGenerateInput) (creationapp.GenerateResult, error) {
	svc, err := a.creationUseCases()
	if err != nil {
		return creationapp.GenerateResult{}, safeDesktopError(err)
	}
	return safeResult(svc.ConfirmCreationGenerate(a.ctx, input.Stage, input.ConfirmationDigest, input.IdempotencyKey))
}

func (a *DesktopAPI) CancelCreationRun() error {
	svc, err := a.creationUseCases()
	if err != nil {
		return safeDesktopError(err)
	}
	return safeDesktopError(svc.CancelCreationRun(a.ctx))
}

func (a *DesktopAPI) RecoverCreation() (creationapp.WorkbenchView, error) {
	svc, err := a.creationUseCases()
	if err != nil {
		return creationapp.WorkbenchView{}, safeDesktopError(err)
	}
	return safeResult(svc.RecoverCreation(a.ctx))
}

func (a *DesktopAPI) PreviewCreationArchive() (creationapp.ArchivePreview, error) {
	svc, err := a.creationUseCases()
	if err != nil {
		return creationapp.ArchivePreview{}, safeDesktopError(err)
	}
	return safeResult(svc.PreviewCreationArchive(a.ctx))
}

func (a *DesktopAPI) ConfirmCreationArchive(input CreationArchiveInput) (creationapp.ArchiveResult, error) {
	svc, err := a.creationUseCases()
	if err != nil {
		return creationapp.ArchiveResult{}, safeDesktopError(err)
	}
	return safeResult(svc.ConfirmCreationArchive(a.ctx, input.ConfirmationDigest, input.IdempotencyKey))
}

func (a *DesktopAPI) ResolveCreationImpact(choice string) (creationapp.ResolveImpactResult, error) {
	svc, err := a.creationUseCases()
	if err != nil {
		return creationapp.ResolveImpactResult{}, safeDesktopError(err)
	}
	return safeResult(svc.ResolveCreationImpact(a.ctx, choice))
}

var _ CreationUseCases = (*workspace.Service)(nil)
