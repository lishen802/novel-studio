package workspace

import (
	"context"
	"fmt"
	"strings"

	"github.com/lishen802/novel-studio/internal/agentassets"
	"github.com/lishen802/novel-studio/internal/application/agentkernel"
	creationapp "github.com/lishen802/novel-studio/internal/application/creation"
	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

type creationKernel struct {
	inner *agentkernel.Service
	store agentkernel.Store
}

func (kernel creationKernel) CreateRun(ctx context.Context, request agentkernel.CreateRequest) (agentdomain.Run, error) {
	return kernel.inner.CreateRun(ctx, request)
}

func (kernel creationKernel) ExecuteRun(ctx context.Context, runID, canonicalHead string) (agentdomain.Run, error) {
	return kernel.inner.ExecuteRun(ctx, runID, canonicalHead)
}

func (kernel creationKernel) Artifacts(ctx context.Context, runID string) ([]agentdomain.Artifact, error) {
	return kernel.store.ListAgentArtifacts(ctx, runID)
}

func (s *Service) CreationWorkbench(ctx context.Context) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, false)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}

func (s *Service) SaveCreationIntent(ctx context.Context, text string) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.SaveIntent(ctx, runtime.projectID, text); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}

func (s *Service) SetCreationAdvanceMode(ctx context.Context, mode string) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.SetAdvanceMode(ctx, runtime.projectID, creationdomain.AdvanceMode(mode)); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}

func (s *Service) SelectCreationDirection(ctx context.Context, candidateID string) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.SelectDirection(ctx, runtime.projectID, candidateID); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}

func (s *Service) AdvanceCreationStage(ctx context.Context) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.AdvanceStage(ctx, runtime.projectID); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}

func (s *Service) LookBackCreationStage(ctx context.Context, stage string) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.LookBackStage(ctx, runtime.projectID, creationdomain.Stage(stage)); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}

func (s *Service) SaveCreationCandidateDraft(ctx context.Context, candidateID, payloadJSON string) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.SaveCandidateDraft(ctx, runtime.projectID, candidateID, payloadJSON); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}

func (s *Service) LockCreationField(ctx context.Context, candidateID, path string) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.LockCandidateField(ctx, runtime.projectID, candidateID, path); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}

func (s *Service) UnlockCreationField(ctx context.Context, candidateID, path string) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.UnlockCandidateField(ctx, runtime.projectID, candidateID, path); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}

func (s *Service) GetCreationCandidate(ctx context.Context, candidateID string) (creationapp.CandidateView, error) {
	runtime, err := s.creationRuntime(ctx, false)
	if err != nil {
		return creationapp.CandidateView{}, err
	}
	candidate, err := runtime.service.CandidatePayload(ctx, runtime.projectID, candidateID)
	if err != nil {
		return creationapp.CandidateView{}, err
	}
	locks := candidate.LockMap
	if locks == nil {
		locks = map[string]bool{}
	}
	return creationapp.CandidateView{
		ID: candidate.ID, Stage: candidate.Stage, Kind: string(candidate.Kind), Version: candidate.Version,
		Status: string(candidate.Status), PayloadJSON: candidate.PayloadJSON, LockMap: locks,
	}, nil
}

func (s *Service) PreviewCreationGenerate(ctx context.Context, stage string) (creationapp.CostPreview, error) {
	runtime, err := s.creationRuntime(ctx, false)
	if err != nil {
		return creationapp.CostPreview{}, err
	}
	profile, err := s.creationProfile(ctx)
	if err != nil {
		return creationapp.CostPreview{}, err
	}
	return runtime.service.PreviewGenerate(ctx, creationapp.GenerateRequest{
		ProjectID: runtime.projectID, BranchID: runtime.branchID, ChapterID: "planning",
		CanonicalHead: "planning", Stage: creationdomain.Stage(stage), Profile: profile,
	})
}

func (s *Service) ConfirmCreationGenerate(ctx context.Context, stage, digest, idempotencyKey string) (creationapp.GenerateResult, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.GenerateResult{}, err
	}
	profile, err := s.creationProfile(ctx)
	if err != nil {
		return creationapp.GenerateResult{}, err
	}
	chapterID, head, err := s.ensurePlanningChapter(ctx)
	if err != nil {
		return creationapp.GenerateResult{}, err
	}
	return runtime.service.ConfirmGenerate(ctx, creationapp.GenerateRequest{
		ProjectID: runtime.projectID, BranchID: runtime.branchID, ChapterID: chapterID,
		CanonicalHead: head, Stage: creationdomain.Stage(stage), Profile: profile,
		ConfirmationDigest: digest, IdempotencyKey: idempotencyKey,
	})
}

func (s *Service) CancelCreationRun(ctx context.Context) error {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return err
	}
	return runtime.service.CancelRun(ctx, runtime.projectID)
}

func (s *Service) RecoverCreation(ctx context.Context) (creationapp.WorkbenchView, error) {
	runtime, err := s.creationRuntime(ctx, false)
	if err != nil {
		return creationapp.WorkbenchView{}, err
	}
	if _, err := runtime.service.Recover(ctx, runtime.projectID); err != nil {
		return creationapp.WorkbenchView{}, err
	}
	return runtime.service.Workbench(ctx, runtime.projectID, runtime.branchID)
}

func (s *Service) PreviewCreationArchive(ctx context.Context) (creationapp.ArchivePreview, error) {
	runtime, err := s.creationRuntime(ctx, false)
	if err != nil {
		return creationapp.ArchivePreview{}, err
	}
	return runtime.service.PreviewConfirmArchive(ctx, runtime.projectID)
}

func (s *Service) ConfirmCreationArchive(ctx context.Context, digest, idempotencyKey string) (creationapp.ArchiveResult, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.ArchiveResult{}, err
	}
	return runtime.service.ConfirmArchive(ctx, creationapp.ArchiveRequest{
		ProjectID: runtime.projectID, ConfirmationDigest: digest, IdempotencyKey: idempotencyKey,
	})
}

func (s *Service) ResolveCreationImpact(ctx context.Context, choice string) (creationapp.ResolveImpactResult, error) {
	runtime, err := s.creationRuntime(ctx, true)
	if err != nil {
		return creationapp.ResolveImpactResult{}, err
	}
	return runtime.service.ResolveImpact(ctx, creationapp.ResolveImpactRequest{
		ProjectID: runtime.projectID, Choice: creationdomain.ImpactChoice(choice),
	})
}

type creationRuntime struct {
	service   *creationapp.Service
	projectID string
	branchID  string
}

func (s *Service) creationRuntime(ctx context.Context, write bool) (creationRuntime, error) {
	if err := ctx.Err(); err != nil {
		return creationRuntime{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.active == nil {
		return creationRuntime{}, ErrNoActiveProject
	}
	if write {
		if err := s.requireEditableLocked(); err != nil {
			return creationRuntime{}, err
		}
	}
	project, err := s.active.store.Project(ctx)
	if err != nil {
		return creationRuntime{}, err
	}
	backend, ok := s.active.store.(creationapp.SQLBackend)
	if !ok {
		return creationRuntime{}, ErrCreationUnavailable
	}
	kernelStore, ok := s.active.store.(agentkernel.Store)
	if !ok {
		return creationRuntime{}, ErrCreationUnavailable
	}
	if s.active.creation == nil {
		registry, regErr := agentassets.Builtin()
		if regErr != nil {
			return creationRuntime{}, fmt.Errorf("%w: %v", ErrCreationUnavailable, regErr)
		}
		onUpdated := func(context.Context, string) {}
		if events, ok := s.active.store.(ports.JobEventStore); ok {
			onUpdated = func(callback context.Context, runID string) {
				s.notifyJobUpdated(callback, events, runID, agentdomain.JobKindAgent)
			}
		}
		kernel := agentkernel.NewService(kernelStore, s.credentials, s.providers, agentkernel.Registries{
			Workflows: registry, Prompts: registry, Schemas: registry, Assemblers: registry,
		}, s.clock, s.ids, onUpdated)
		svc, svcErr := creationapp.New(creationapp.Config{
			Store: creationapp.NewSQLiteStore(backend), Kernel: creationKernel{inner: kernel, store: kernelStore},
			Clock: s.clock, IDs: s.ids,
			Generation: func() uint64 {
				s.mu.RLock()
				defer s.mu.RUnlock()
				if s.active == nil {
					return 0
				}
				return s.active.generation
			},
		})
		if svcErr != nil {
			return creationRuntime{}, svcErr
		}
		s.active.creation = svc
	}
	branchID := "main"
	return creationRuntime{service: s.active.creation, projectID: project.ID, branchID: branchID}, nil
}

func (s *Service) creationProfile(ctx context.Context) (agentdomain.ResolvedCreativeProfile, error) {
	s.mu.RLock()
	if s.active == nil {
		s.mu.RUnlock()
		return agentdomain.ResolvedCreativeProfile{}, ErrNoActiveProject
	}
	store := s.active.store
	s.mu.RUnlock()
	if s.profileResolver == nil {
		return agentdomain.ResolvedCreativeProfile{}, ErrAgentRuntimeUnavailable
	}
	selection, found, err := readProfileSelection(ctx, store)
	if err != nil {
		return agentdomain.ResolvedCreativeProfile{}, err
	}
	if !found || strings.TrimSpace(selection.ProfileID) == "" {
		return agentdomain.ResolvedCreativeProfile{}, ErrProjectProfileNotConfigured
	}
	return s.profileResolver.ResolveCreativeProfile(ctx, selection.ProfileID, selection.Version)
}

func (s *Service) ensurePlanningChapter(ctx context.Context) (string, string, error) {
	s.mu.RLock()
	if s.active == nil {
		s.mu.RUnlock()
		return "", "", ErrNoActiveProject
	}
	store := s.active.store
	s.mu.RUnlock()
	chapters, err := store.ListChapters(ctx)
	if err != nil {
		return "", "", err
	}
	if len(chapters) == 0 {
		created, createErr := s.CreateChapter(ctx, "作品规划")
		if createErr != nil {
			return "", "", createErr
		}
		return created.ID, created.ID, nil
	}
	return chapters[0].ID, chapters[0].ID, nil
}
