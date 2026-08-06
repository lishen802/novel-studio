package workspace

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
)

// gatedModelProviderFactory produces providers that block on a release channel
// (and ctx.Done) so tests can hold a job inside its Model IO phase while
// exercising the workspace concurrently. started is closed on the first
// Generate call so the test knows the run has reached the provider.
type gatedModelProviderFactory struct {
	outputs map[string]string
	release chan struct{}
	started chan struct{}
	once    sync.Once
}

func newGatedModelProviderFactory(outputs map[string]string) *gatedModelProviderFactory {
	return &gatedModelProviderFactory{
		outputs: outputs,
		release: make(chan struct{}),
		started: make(chan struct{}),
	}
}

func (f *gatedModelProviderFactory) New(agentdomain.ProviderConnection) (ports.ModelProvider, error) {
	return gatedModelProvider{factory: f}, nil
}

type gatedModelProvider struct {
	factory *gatedModelProviderFactory
}

func (provider gatedModelProvider) Generate(
	ctx context.Context,
	_ string,
	request ports.ModelGenerateRequest,
	onDelta func(string),
) (ports.ModelGenerateResult, error) {
	provider.factory.once.Do(func() { close(provider.factory.started) })
	select {
	case <-provider.factory.release:
	case <-ctx.Done():
		return ports.ModelGenerateResult{}, ctx.Err()
	}
	output := provider.factory.outputs[request.Model]
	if onDelta != nil {
		onDelta(output)
	}
	return ports.ModelGenerateResult{
		Text:              output,
		ProviderRequestID: "request-" + request.Model,
		InputTokens:       10,
		OutputTokens:      5,
	}, nil
}

// gatedAuthoringOutputs returns the structured outputs the three-role authoring
// pipeline expects (planner JSON, writer prose, reviewer JSON).
func gatedAuthoringOutputs() map[string]string {
	return map[string]string{
		"planner":  `{"summary":"夜探宗门","scenes":[{"title":"潜入","goal":"取得名册","conflict":"守卫巡查","outcome":"发现暗门"}]}`,
		"writer":   "夜色压在山门之上，墨老沿着石阶无声而行。",
		"reviewer": `{"approved":true,"summary":"未发现连续性冲突","issues":[]}`,
	}
}

// waitForProviderStarted blocks until the gated provider signals its first
// Generate call or the test times out, proving the run reached Model IO.
func waitForProviderStarted(t *testing.T, factory *gatedModelProviderFactory) {
	t.Helper()
	select {
	case <-factory.started:
	case <-time.After(3 * time.Second):
		t.Fatal("provider never started; run did not reach Model IO")
	}
}

// TestExecuteAgentRunDoesNotBlockCreateChapterDuringProviderCall asserts that a
// chapter creation (which needs the workspace write lock) succeeds while an
// agent run is blocked inside provider.Generate. Today the run holds
// s.mu.RLock() across the whole provider call, so CreateChapter's Lock blocks
// until the run finishes; this must change so Model IO no longer holds the
// workspace lock.
func TestExecuteAgentRunDoesNotBlockCreateChapterDuringProviderCall(t *testing.T) {
	ctx := context.Background()
	factory := newGatedModelProviderFactory(gatedAuthoringOutputs())
	service, chapter := configuredAgentService(t, factory)
	defer service.Close()

	run, err := service.CreateChapterAgentRun(ctx, chapter.ID, "续写这一章。")
	if err != nil {
		t.Fatal(err)
	}

	runDone := make(chan struct{})
	go func() {
		_, _ = service.ExecuteAgentRun(ctx, run.ID)
		close(runDone)
	}()

	waitForProviderStarted(t, factory)

	// CreateChapter needs the workspace write lock. While the provider call is
	// in flight it must succeed rather than wait for the run to finish.
	createDone := make(chan struct{})
	var created ChapterView
	var createErr error
	go func() {
		created, createErr = service.CreateChapter(ctx, "第二章")
		close(createDone)
	}()
	select {
	case <-createDone:
		if createErr != nil {
			t.Fatalf("CreateChapter during provider call: %v", createErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("CreateChapter was blocked behind the in-flight provider call")
	}

	close(factory.release)
	select {
	case <-runDone:
	case <-time.After(5 * time.Second):
		t.Fatal("ExecuteAgentRun did not complete after releasing the provider")
	}
	if created.ID == "" {
		t.Fatal("created chapter has no id")
	}
}

// TestExecuteAgentRunDoesNotBlockSaveDraftDuringProviderCall is a regression
// guard: SaveDraft uses a read lock and must continue to succeed while a
// provider call is in flight, both before and after the lock split.
func TestExecuteAgentRunDoesNotBlockSaveDraftDuringProviderCall(t *testing.T) {
	ctx := context.Background()
	factory := newGatedModelProviderFactory(gatedAuthoringOutputs())
	service, chapter := configuredAgentService(t, factory)
	defer service.Close()

	run, err := service.CreateChapterAgentRun(ctx, chapter.ID, "续写这一章。")
	if err != nil {
		t.Fatal(err)
	}

	runDone := make(chan struct{})
	go func() {
		_, _ = service.ExecuteAgentRun(ctx, run.ID)
		close(runDone)
	}()

	waitForProviderStarted(t, factory)

	saveDone := make(chan error, 1)
	go func() {
		saveDone <- service.SaveDraft(ctx, chapter.ID, "新恢复草稿", chapter.Revision)
	}()
	select {
	case err := <-saveDone:
		if err != nil {
			t.Fatalf("SaveDraft during provider call: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("SaveDraft was blocked behind the in-flight provider call")
	}

	close(factory.release)
	select {
	case <-runDone:
	case <-time.After(5 * time.Second):
		t.Fatal("ExecuteAgentRun did not complete after releasing the provider")
	}

	view, err := service.OpenChapter(ctx, chapter.ID)
	if err != nil {
		t.Fatalf("OpenChapter: %v", err)
	}
	if !view.HasRecoveryDraft {
		t.Fatal("recovery draft was not persisted")
	}
}

// stickyModelProviderFactory produces providers that block ONLY on a release
// channel and ignore ctx cancellation. This lets tests hold a job inside Model
// IO past the drain timeout so the lease is force-marked stale, exercising the
// generation safety net that prevents writes to a closed store.
type stickyModelProviderFactory struct {
	outputs map[string]string
	release chan struct{}
	started chan struct{}
	once    sync.Once
}

func newStickyModelProviderFactory(outputs map[string]string) *stickyModelProviderFactory {
	return &stickyModelProviderFactory{
		outputs: outputs,
		release: make(chan struct{}),
		started: make(chan struct{}),
	}
}

func (f *stickyModelProviderFactory) New(agentdomain.ProviderConnection) (ports.ModelProvider, error) {
	return stickyModelProvider{factory: f}, nil
}

type stickyModelProvider struct {
	factory *stickyModelProviderFactory
}

func (provider stickyModelProvider) Generate(
	_ context.Context,
	_ string,
	request ports.ModelGenerateRequest,
	onDelta func(string),
) (ports.ModelGenerateResult, error) {
	provider.factory.once.Do(func() { close(provider.factory.started) })
	<-provider.factory.release
	output := provider.factory.outputs[request.Model]
	if onDelta != nil {
		onDelta(output)
	}
	return ports.ModelGenerateResult{
		Text:              output,
		ProviderRequestID: "request-" + request.Model,
		InputTokens:       10,
		OutputTokens:      5,
	}, nil
}

// TestAgentRunSkipsPersistWhenStoreGenerationChanged asserts that when a project
// switch drains an in-flight agent run past the drain timeout, the job's
// subsequent persistence attempt is skipped and the run transitions to
// Recoverable instead of writing to the closed store.
func TestAgentRunSkipsPersistWhenStoreGenerationChanged(t *testing.T) {
	ctx := context.Background()
	factory := newStickyModelProviderFactory(gatedAuthoringOutputs())
	service, chapter := configuredAgentService(t, factory)
	// A zero drain timeout causes replaceActive to mark remaining leases stale
	// immediately, simulating a drain that cannot wait for the stuck provider.
	service.drainTimeout = 0

	run, err := service.CreateChapterAgentRun(ctx, chapter.ID, "续写这一章。")
	if err != nil {
		t.Fatal(err)
	}

	runDone := make(chan AgentRunView, 1)
	runErrCh := make(chan error, 1)
	go func() {
		got, executeErr := service.ExecuteAgentRun(ctx, run.ID)
		runDone <- got
		runErrCh <- executeErr
	}()

	waitForStickyProviderStarted(t, factory)

	// Switch to a new project. replaceActive will drain (timeout=0 → stale),
	// close the old store, and install the new generation.
	parentDirectory := t.TempDir()
	if _, err := service.CreateProject(ctx, parentDirectory, "新项目", "奇幻", "简介"); err != nil {
		t.Fatal(err)
	}

	// Release the stuck provider. The job will try to persist the step result
	// but the generation has changed and the lease is stale.
	close(factory.release)

	select {
	case got := <-runDone:
		if got.Status != "recoverable" {
			t.Fatalf("run status = %s, want recoverable", got.Status)
		}
		if got.ErrorCode != "store_generation_changed" {
			t.Fatalf("run error code = %s, want store_generation_changed", got.ErrorCode)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("ExecuteAgentRun did not return after releasing the provider")
	}

	select {
	case executeErr := <-runErrCh:
		if !errors.Is(executeErr, ErrAgentRunStaleStore) {
			t.Fatalf("ExecuteAgentRun error = %v, want ErrAgentRunStaleStore", executeErr)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ExecuteAgentRun error not received")
	}
}

// TestRetryAgentRunAfterProjectSwitchUsesNewStore asserts that after a stale
// run is recovered (via RecoverInterruptedAgentRuns on project reopen), the
// retry creates a new run on the reopened store rather than writing to the old
// closed one.
func TestRetryAgentRunAfterProjectSwitchUsesNewStore(t *testing.T) {
	ctx := context.Background()
	factory := newStickyModelProviderFactory(gatedAuthoringOutputs())
	service, chapter := configuredAgentService(t, factory)
	service.drainTimeout = 0
	projectPath := service.active.packageInfo.Root

	run, err := service.CreateChapterAgentRun(ctx, chapter.ID, "续写这一章。")
	if err != nil {
		t.Fatal(err)
	}

	runDone := make(chan struct{})
	go func() {
		_, _ = service.ExecuteAgentRun(ctx, run.ID)
		close(runDone)
	}()

	waitForStickyProviderStarted(t, factory)

	// Switch to a new project — the old store is closed and the lease is stale.
	parentDirectory := t.TempDir()
	if _, err := service.CreateProject(ctx, parentDirectory, "新项目", "奇幻", "简介"); err != nil {
		t.Fatal(err)
	}
	close(factory.release)
	<-runDone

	// Reopen the original project. RecoverInterruptedAgentRuns should
	// transition the stuck Running run to Recoverable.
	if _, err := service.OpenProject(ctx, projectPath); err != nil {
		t.Fatal(err)
	}

	recovered, err := service.AgentRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if recovered.Status != "recoverable" {
		t.Fatalf("recovered run status = %s, want recoverable", recovered.Status)
	}

	// Retry on the reopened store. This must succeed and create a new queued run.
	retry, err := service.RetryAgentRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("RetryAgentRun: %v", err)
	}
	if retry.ID == run.ID {
		t.Fatal("retry run ID must differ from original")
	}
	if retry.Status != "queued" {
		t.Fatalf("retry status = %s, want queued", retry.Status)
	}
	if retry.RetryOf != run.ID {
		t.Fatalf("retry.RetryOf = %s, want %s", retry.RetryOf, run.ID)
	}
}

func waitForStickyProviderStarted(t *testing.T, factory *stickyModelProviderFactory) {
	t.Helper()
	select {
	case <-factory.started:
	case <-time.After(3 * time.Second):
		t.Fatal("sticky provider never started; run did not reach Model IO")
	}
}
