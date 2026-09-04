package workspace

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/agentkernel"

	creationapp "github.com/lishen802/novel-studio/internal/application/creation"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	creationdomain "github.com/lishen802/novel-studio/internal/domain/creation"
)

func TestCreationRejectsLeaseFromPreviousProject(t *testing.T) {
	ctx := context.Background()
	service := newWorkbenchTestService(t)
	t.Cleanup(func() { _ = service.Close() })
	if _, err := service.CreateProject(ctx, t.TempDir(), "旧项目", "悬疑", ""); err != nil {
		t.Fatal(err)
	}
	coordinator := creationCoordinator{workspace: service, generation: service.currentGeneration()}
	if _, err := service.CreateProject(ctx, t.TempDir(), "新项目", "悬疑", ""); err != nil {
		t.Fatal(err)
	}
	if _, _, err := coordinator.BeginRun(ctx, "old-page-run"); !errors.Is(err, creationapp.ErrStaleStore) {
		t.Fatalf("old coordinator acquired new project: %v", err)
	}
}

func TestCreationKernelRejectsStaleLeaseEvenWithoutCancellation(t *testing.T) {
	ctx := context.Background()
	service := newWorkbenchTestService(t)
	t.Cleanup(func() { _ = service.Close() })
	if _, err := service.CreateProject(ctx, t.TempDir(), "租约测试", "悬疑", ""); err != nil {
		t.Fatal(err)
	}
	lease, release, err := service.BeginRun(ctx, "run-1")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	lease.(creationRunLease).lease.stale.Store(true)
	guarded := creationKernelStore{workspace: service, generation: service.currentGeneration()}
	// 终态写入会去掉取消信号，但不能去掉租约的过期保护。
	if err := guarded.SaveAgentStepBundle(context.WithoutCancel(lease.Context()), agentdomain.StepBundle{}); !errors.Is(err, creationapp.ErrStaleStore) {
		t.Fatalf("stale lease reached backend: %v", err)
	}
	creationStore := creationStore{guard: guarded}
	if err := creationStore.SaveCandidate(context.WithoutCancel(lease.Context()), creationdomain.Candidate{}); !errors.Is(err, creationapp.ErrStaleStore) {
		t.Fatalf("stale candidate reached backend: %v", err)
	}
	if err := creationStore.UpdateOperation(context.WithoutCancel(lease.Context()), creationdomain.Operation{}); !errors.Is(err, creationapp.ErrStaleStore) {
		t.Fatalf("stale receipt reached backend: %v", err)
	}
}

func TestCreationCloseCancelsRealKernelProvider(t *testing.T) {
	ctx := context.Background()
	factory := newGatedModelProviderFactory(gatedAuthoringOutputs())
	service, _ := configuredAgentService(t, factory)
	service.clock = fixedClock{now: time.Now().UTC()}
	t.Cleanup(func() { _ = service.Close() })
	runtime, err := service.creationRuntime(ctx, true)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := runtime.service.EnsureSession(ctx, runtime.projectID, runtime.branchID); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SaveCreationIntent(ctx, "雾港里的归乡人"); err != nil {
		t.Fatal(err)
	}
	preview, err := service.PreviewCreationGenerate(ctx, "directions")
	if err != nil {
		t.Fatal(err)
	}
	result, err := service.ConfirmCreationGenerate(ctx, "directions", preview.ConfirmationDigest, "close-run")
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-factory.started:
	case <-time.After(3 * time.Second):
		run, readErr := service.active.store.(agentkernel.Store).AgentRun(ctx, result.RunID)
		t.Fatalf("provider never started: view=%+v err=%v", run, readErr)
	}
	if !service.Live(result.RunID) {
		t.Fatal("model call has no workspace lease")
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	if service.Live(result.RunID) {
		t.Fatal("close retained running creation job")
	}
}
