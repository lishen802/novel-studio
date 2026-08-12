package workspace

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/lishen802/novel-studio/internal/application/license"
	"github.com/lishen802/novel-studio/internal/application/ports"
	agentdomain "github.com/lishen802/novel-studio/internal/domain/agent"
	"github.com/lishen802/novel-studio/internal/persistence/sqlite"
	"github.com/lishen802/novel-studio/internal/platform/ids"
	"github.com/lishen802/novel-studio/internal/platform/projectfs"
)

func TestAgentAuthoringCreatesReviewableChapterChangeSet(t *testing.T) {
	ctx := context.Background()
	credentials := &fakeCredentialStore{secrets: map[string]string{}}
	providerFactory := &fakeModelProviderFactory{
		outputs: map[string]string{
			"planner-model":  `{"summary":"夜探宗门","scenes":[{"title":"潜入","goal":"取得名册","conflict":"守卫巡查","outcome":"发现暗门"}]}`,
			"writer-model":   "夜色压在山门之上，墨老沿着石阶无声而行。",
			"reviewer-model": `{"approved":true,"summary":"未发现连续性冲突","issues":[]}`,
		},
	}
	ids := &sequenceIDs{ids: []string{
		"project-1", "chapter-1",
		"provider-1", "provider-2",
		"profile-1", "profile-2", "profile-3",
		"run-1", "step-1", "step-2", "step-3",
		"artifact-context", "artifact-plan", "artifact-draft", "artifact-review",
		"change-set-1", "operation-1",
	}}
	service := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Unix(200, 0)},
		ids,
		&fakeSnapshotArchive{},
	)
	service.ConfigureAgentRuntime(credentials, providerFactory)
	project, err := service.CreateProject(ctx, t.TempDir(), "凡人修仙", "修仙", "墨老企图夺舍主角")
	if err != nil {
		t.Fatal(err)
	}
	defer service.Close()
	chapter, err := service.CreateChapter(ctx, "第一章")
	if err != nil {
		t.Fatal(err)
	}

	openAI, err := service.SaveProviderConnection(ctx, ProviderConnectionInput{
		Name: "OpenAI", Kind: "openai", BaseURL: "https://api.openai.com/v1", APIKey: "key-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	anthropic, err := service.SaveProviderConnection(ctx, ProviderConnectionInput{
		Name: "Anthropic", Kind: "anthropic", BaseURL: "https://api.anthropic.com", APIKey: "key-b",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []AgentProfileInput{
		{Role: "planner", ConnectionID: openAI.ID, Model: "planner-model", MaxOutputTokens: 1000, Temperature: 0.2, Enabled: true},
		{Role: "writer", ConnectionID: openAI.ID, Model: "writer-model", MaxOutputTokens: 3000, Temperature: 0.7, Enabled: true},
		{Role: "continuity_reviewer", ConnectionID: anthropic.ID, Model: "reviewer-model", MaxOutputTokens: 1000, Temperature: 0.1, Enabled: true},
	} {
		if _, err := service.SaveAgentProfile(ctx, input); err != nil {
			t.Fatal(err)
		}
	}

	run, err := service.CreateChapterAgentRun(ctx, chapter.ID, "续写这一章，保持克制的第三人称。")
	if err != nil {
		t.Fatal(err)
	}
	run, err = service.ExecuteAgentRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	waitForAgentRunStatus(t, service, run.ID, "completed", 5*time.Second)
	run, err = service.AgentRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if run.Status != "completed" || run.ChangeSetID == "" {
		t.Fatalf("unexpected run: %#v", run)
	}
	if run.InputTokens != 30 || run.OutputTokens != 15 {
		t.Fatalf("unexpected run token usage: %#v", run)
	}
	if run.Steps[0].InputTokens != 10 || run.Steps[0].OutputTokens != 5 {
		t.Fatalf("unexpected step token usage: %#v", run.Steps[0])
	}
	if len(providerFactory.requests) != 3 {
		t.Fatalf("provider calls = %d", len(providerFactory.requests))
	}
	if providerFactory.requests[1].Input == "" || providerFactory.requests[2].Input == "" {
		t.Fatal("writer and reviewer must receive assembled context")
	}
	changeSets, err := service.ChangeSets(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(changeSets) != 1 ||
		changeSets[0].Operations[0].AfterValue != providerFactory.outputs["writer-model"] {
		t.Fatalf("unexpected change sets: %#v", changeSets)
	}
	if project.ID == "" {
		t.Fatal("project id missing")
	}
}

func TestCreateChapterAgentRunRejectsConfiguredOutputAboveBudget(t *testing.T) {
	service, chapter := configuredAgentService(t, failingModelProviderFactory{})
	defer service.Close()
	if _, err := service.SaveAgentRuntimeSettings(context.Background(), AgentRuntimeSettingsInput{
		MaxRunTokens:    4999,
		MaxContextChars: 120000,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.CreateChapterAgentRun(context.Background(), chapter.ID, "继续写作"); !errors.Is(err, ErrTokenBudgetExceeded) {
		t.Fatalf("CreateChapterAgentRun() error = %v, want ErrTokenBudgetExceeded", err)
	}
}

func TestAgentRunStopsBeforeNextStepWhenActualUsageExhaustsBudget(t *testing.T) {
	providerFactory := &usageModelProviderFactory{}
	service, chapter := configuredAgentService(t, providerFactory)
	defer service.Close()
	if _, err := service.SaveAgentRuntimeSettings(context.Background(), AgentRuntimeSettingsInput{
		MaxRunTokens:    5000,
		MaxContextChars: 120000,
	}); err != nil {
		t.Fatal(err)
	}
	run, err := service.CreateChapterAgentRun(context.Background(), chapter.ID, "继续写作")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ExecuteAgentRun(context.Background(), run.ID); err != nil {
		t.Fatal(err)
	}
	waitForAgentRunStatus(t, service, run.ID, "failed", 5*time.Second)
	if providerFactory.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", providerFactory.calls)
	}
	got, err := service.AgentRun(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "failed" || got.InputTokens+got.OutputTokens <= got.TokenBudget {
		t.Fatalf("unexpected budget failure run: %#v", got)
	}
}

func TestAgentRunRejectsRequiredContextAboveBudgetBeforeProviderCall(t *testing.T) {
	providerFactory := &usageModelProviderFactory{}
	service, chapter := configuredAgentService(t, providerFactory)
	defer service.Close()
	if _, err := service.SaveAgentRuntimeSettings(context.Background(), AgentRuntimeSettingsInput{
		MaxRunTokens:    5000,
		MaxContextChars: 1,
	}); err != nil {
		t.Fatal(err)
	}
	run, err := service.CreateChapterAgentRun(context.Background(), chapter.ID, "继续写作")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ExecuteAgentRun(context.Background(), run.ID); err != nil {
		t.Fatal(err)
	}
	waitForAgentRunStatus(t, service, run.ID, "failed", 5*time.Second)
	if providerFactory.calls != 0 {
		t.Fatalf("provider calls = %d, want 0", providerFactory.calls)
	}
	got, err := service.AgentRun(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "failed" || got.ErrorCode != "context_budget_exceeded" {
		t.Fatalf("unexpected context budget failure run: %#v", got)
	}
}

func TestCreateChapterAgentRunRequiresIndependentReviewer(t *testing.T) {
	profiles := []agentdomain.AgentProfile{
		{Role: agentdomain.RolePlanner, ConnectionID: "one", Model: "model", Enabled: true},
		{Role: agentdomain.RoleWriter, ConnectionID: "one", Model: "model", Enabled: true},
		{Role: agentdomain.RoleContinuityReviewer, ConnectionID: "one", Model: "model", Enabled: true},
	}
	if err := agentdomain.ValidateAuthoringProfiles(profiles); !errors.Is(err, agentdomain.ErrReviewerNotIndependent) {
		t.Fatalf("expected independent reviewer error, got %v", err)
	}
}

func TestCancelAgentRunPropagatesToProviderAndPersistsCancelledState(t *testing.T) {
	providerFactory := &blockingModelProviderFactory{started: make(chan struct{})}
	service, chapter := configuredAgentService(t, providerFactory)
	defer service.Close()
	run, err := service.CreateChapterAgentRun(context.Background(), chapter.ID, "继续写作")
	if err != nil {
		t.Fatal(err)
	}
	// ExecuteAgentRun returns immediately with a running view; the blocking
	// provider runs in the background goroutine.
	if _, err := service.ExecuteAgentRun(context.Background(), run.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case <-providerFactory.started:
	case <-time.After(3 * time.Second):
		t.Fatal("provider did not start")
	}
	if err := service.CancelAgentRun(context.Background(), run.ID); err != nil {
		t.Fatalf("CancelAgentRun() error = %v", err)
	}
	waitForAgentRunStatus(t, service, run.ID, "cancelled", 5*time.Second)
	got, err := service.AgentRun(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "cancelled" {
		t.Fatalf("run status = %q", got.Status)
	}
	if len(got.Steps) != 3 ||
		got.Steps[0].Status != "cancelled" ||
		got.Steps[1].Status != "cancelled" {
		t.Fatalf("unexpected cancelled steps: %#v", got.Steps)
	}
}

func TestExecuteAgentRunPersistsCancellationBeforeStartup(t *testing.T) {
	providerFactory := &usageModelProviderFactory{}
	service, chapter := configuredAgentService(t, providerFactory)
	defer service.Close()
	run, err := service.CreateChapterAgentRun(context.Background(), chapter.ID, "继续写作")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got, err := service.ExecuteAgentRun(ctx, run.ID)
	if err != nil {
		t.Fatalf("ExecuteAgentRun() error = %v", err)
	}
	if got.Status != "cancelled" || got.ErrorCode != "cancelled" {
		t.Fatalf("unexpected cancelled run: %#v", got)
	}
	if providerFactory.calls != 0 {
		t.Fatalf("provider calls = %d, want 0", providerFactory.calls)
	}
	persisted, err := service.AgentRun(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if persisted.Status != "cancelled" {
		t.Fatalf("persisted status = %s, want cancelled", persisted.Status)
	}
}

func TestAgentRunPreservesCompletedStepArtifactWhenCancelledAfterProviderSuccess(t *testing.T) {
	providerFactory := &cancelAfterGenerateProviderFactory{}
	service, chapter := configuredAgentService(t, providerFactory)
	defer service.Close()
	run, err := service.CreateChapterAgentRun(context.Background(), chapter.ID, "继续写作")
	if err != nil {
		t.Fatal(err)
	}
	// The provider cancels the in-flight job (via CancelAgentRun) after the
	// planner step generates successfully, mirroring a user-initiated cancel
	// mid-pipeline. The detached job context lets the cancellation reach the
	// background goroutine even though ExecuteAgentRun has already returned.
	providerFactory.cancel = func() {
		_ = service.CancelAgentRun(context.Background(), run.ID)
	}
	got, err := service.ExecuteAgentRun(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("ExecuteAgentRun() error = %v", err)
	}
	if got.Status != "running" {
		t.Fatalf("unexpected initial run status: %#v", got)
	}
	waitForAgentRunStatus(t, service, run.ID, "cancelled", 5*time.Second)
	got, err = service.AgentRun(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != "cancelled" ||
		got.Steps[0].Status != "completed" ||
		got.Steps[1].Status != "cancelled" {
		t.Fatalf("unexpected cancelled run: %#v", got)
	}
	service.mu.RLock()
	store, err := service.agentStoreLocked()
	service.mu.RUnlock()
	if err != nil {
		t.Fatal(err)
	}
	artifacts, err := store.ListAgentArtifacts(context.Background(), run.ID)
	if err != nil {
		t.Fatal(err)
	}
	artifactKinds := make(map[agentdomain.ArtifactKind]bool, len(artifacts))
	for _, artifact := range artifacts {
		artifactKinds[artifact.Kind] = true
	}
	if len(artifacts) != 2 ||
		!artifactKinds[agentdomain.ArtifactContext] ||
		!artifactKinds[agentdomain.ArtifactPlan] {
		t.Fatalf("completed artifacts were not preserved: %#v", artifacts)
	}
}

func TestRetryAgentRunCreatesNewQueuedAuditRecord(t *testing.T) {
	service, chapter := configuredAgentService(t, failingModelProviderFactory{})
	defer service.Close()
	original, err := service.CreateChapterAgentRun(context.Background(), chapter.ID, "沿用原目标")
	if err != nil {
		t.Fatal(err)
	}
	// ExecuteAgentRun returns a running view immediately; the provider failure
	// is persisted by the background goroutine, so wait for the failed terminal
	// state before retrying.
	if _, err := service.ExecuteAgentRun(context.Background(), original.ID); err != nil {
		t.Fatalf("ExecuteAgentRun() error = %v", err)
	}
	waitForAgentRunStatus(t, service, original.ID, "failed", 5*time.Second)
	retry, err := service.RetryAgentRun(context.Background(), original.ID)
	if err != nil {
		t.Fatalf("RetryAgentRun() error = %v", err)
	}
	if retry.ID == original.ID || retry.RetryOf != original.ID ||
		retry.Prompt != original.Prompt || retry.Status != "queued" {
		t.Fatalf("unexpected retry: %#v", retry)
	}
}

func TestVersionOperationIsRejectedWhileAgentRunIsActive(t *testing.T) {
	providerFactory := &blockingModelProviderFactory{started: make(chan struct{})}
	service, chapter := configuredAgentService(t, providerFactory)
	defer service.Close()
	run, err := service.CreateChapterAgentRun(context.Background(), chapter.ID, "继续写作")
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() {
		_, _ = service.ExecuteAgentRun(context.Background(), run.ID)
		close(done)
	}()
	select {
	case <-providerFactory.started:
	case <-time.After(3 * time.Second):
		t.Fatal("provider did not start")
	}
	if _, err := service.SwitchBranch(context.Background(), "main"); !errors.Is(err, ErrAgentRunActive) {
		t.Fatalf("SwitchBranch() error = %v, want ErrAgentRunActive", err)
	}
	if _, err := service.ConfirmBranchMerge(context.Background(), "preview-token"); !errors.Is(err, ErrAgentRunActive) {
		t.Fatalf("ConfirmBranchMerge() error = %v, want ErrAgentRunActive", err)
	}
	if _, err := service.RestoreSnapshot(context.Background(), "snapshot-1"); !errors.Is(err, ErrAgentRunActive) {
		t.Fatalf("RestoreSnapshot() error = %v, want ErrAgentRunActive", err)
	}
	if err := service.CancelAgentRun(context.Background(), run.ID); err != nil {
		t.Fatal(err)
	}
	<-done
}

func TestCreateProjectCancelsActiveAgentRunBeforeReplacingWorkspace(t *testing.T) {
	providerFactory := &blockingModelProviderFactory{started: make(chan struct{})}
	service, chapter := configuredAgentService(t, providerFactory)
	defer service.Close()
	projectPath := service.active.packageInfo.Root
	run, err := service.CreateChapterAgentRun(context.Background(), chapter.ID, "继续写作")
	if err != nil {
		t.Fatal(err)
	}
	// ExecuteAgentRun returns immediately with a running view; the blocking
	// provider runs in the background goroutine.
	got, err := service.ExecuteAgentRun(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("ExecuteAgentRun() error = %v", err)
	}
	if got.Status != "running" {
		t.Fatalf("unexpected initial run status: %#v", got)
	}
	select {
	case <-providerFactory.started:
	case <-time.After(3 * time.Second):
		t.Fatal("provider did not start")
	}

	// Switching projects drains the in-flight job: the drain cancels the job
	// context, the blocking provider unblocks, the goroutine persists the
	// cancelled state to the soon-to-close store, and releases the lease. This
	// must complete (not block past the drain timeout) so the workspace can be
	// replaced.
	parentDirectory := t.TempDir()
	if _, err := service.CreateProject(
		context.Background(),
		parentDirectory,
		"新项目",
		"奇幻",
		"简介",
	); err != nil {
		t.Fatalf("CreateProject() error = %v", err)
	}

	// Reopen the original project to verify the run was cancelled before the
	// store closed (not left running or corrupted). RecoverInterruptedAgentRuns
	// only transitions running runs; a cancelled run stays cancelled.
	if _, err := service.OpenProject(context.Background(), projectPath); err != nil {
		t.Fatalf("OpenProject() error = %v", err)
	}
	recovered, err := service.AgentRun(context.Background(), run.ID)
	if err != nil {
		t.Fatalf("AgentRun() error = %v", err)
	}
	if recovered.Status != "cancelled" {
		t.Fatalf("run status = %s, want cancelled", recovered.Status)
	}
}

func configuredAgentService(
	t *testing.T,
	providerFactory ports.ModelProviderFactory,
) (*Service, ChapterView) {
	t.Helper()
	ctx := context.Background()
	credentials := &fakeCredentialStore{secrets: map[string]string{}}
	service := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Unix(200, 0)},
		ids.UUIDGenerator{},
		&fakeSnapshotArchive{},
	)
	service.ConfigureAgentRuntime(credentials, providerFactory)
	if _, err := service.CreateProject(ctx, t.TempDir(), "凡人修仙", "修仙", "简介"); err != nil {
		t.Fatal(err)
	}
	chapter, err := service.CreateChapter(ctx, "第一章")
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.SaveProviderConnection(ctx, ProviderConnectionInput{
		Name: "第一连接", Kind: "openai", BaseURL: "https://api.openai.com/v1", APIKey: "key-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.SaveProviderConnection(ctx, ProviderConnectionInput{
		Name: "第二连接", Kind: "anthropic", BaseURL: "https://api.anthropic.com", APIKey: "key-b",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []AgentProfileInput{
		{Role: "planner", ConnectionID: first.ID, Model: "planner", MaxOutputTokens: 1000, Temperature: 0.2, Enabled: true},
		{Role: "writer", ConnectionID: first.ID, Model: "writer", MaxOutputTokens: 3000, Temperature: 0.7, Enabled: true},
		{Role: "continuity_reviewer", ConnectionID: second.ID, Model: "reviewer", MaxOutputTokens: 1000, Temperature: 0.1, Enabled: true},
	} {
		if _, err := service.SaveAgentProfile(ctx, input); err != nil {
			t.Fatal(err)
		}
	}
	return service, chapter
}

type blockingModelProviderFactory struct {
	started chan struct{}
	once    sync.Once
}

func (factory *blockingModelProviderFactory) New(agentdomain.ProviderConnection) (ports.ModelProvider, error) {
	return blockingModelProvider{factory: factory}, nil
}

type blockingModelProvider struct {
	factory *blockingModelProviderFactory
}

func (provider blockingModelProvider) Generate(
	ctx context.Context,
	_ string,
	_ ports.ModelGenerateRequest,
	_ func(string),
) (ports.ModelGenerateResult, error) {
	provider.factory.once.Do(func() { close(provider.factory.started) })
	<-ctx.Done()
	return ports.ModelGenerateResult{}, ctx.Err()
}

type failingModelProviderFactory struct{}

func (failingModelProviderFactory) New(agentdomain.ProviderConnection) (ports.ModelProvider, error) {
	return failingModelProvider{}, nil
}

type failingModelProvider struct{}

func (failingModelProvider) Generate(
	context.Context,
	string,
	ports.ModelGenerateRequest,
	func(string),
) (ports.ModelGenerateResult, error) {
	return ports.ModelGenerateResult{}, errors.New("provider unavailable")
}

type fakeCredentialStore struct {
	secrets map[string]string
}

func (store *fakeCredentialStore) Save(_ context.Context, reference, secret string) error {
	store.secrets[reference] = secret
	return nil
}

func (store *fakeCredentialStore) Get(_ context.Context, reference string) (string, error) {
	value, exists := store.secrets[reference]
	if !exists {
		return "", errors.New("credential missing")
	}
	return value, nil
}

func (store *fakeCredentialStore) Delete(_ context.Context, reference string) error {
	delete(store.secrets, reference)
	return nil
}

type capturedModelRequest struct {
	Model string
	Input string
}

type fakeModelProviderFactory struct {
	outputs  map[string]string
	requests []capturedModelRequest
}

func (factory *fakeModelProviderFactory) New(agentdomain.ProviderConnection) (ports.ModelProvider, error) {
	return fakeModelProvider{factory: factory}, nil
}

type fakeModelProvider struct {
	factory *fakeModelProviderFactory
}

func (provider fakeModelProvider) Generate(
	_ context.Context,
	_ string,
	request ports.ModelGenerateRequest,
	onDelta func(string),
) (ports.ModelGenerateResult, error) {
	provider.factory.requests = append(provider.factory.requests, capturedModelRequest{
		Model: request.Model,
		Input: request.Input,
	})
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

type usageModelProviderFactory struct {
	calls int
}

func (factory *usageModelProviderFactory) New(agentdomain.ProviderConnection) (ports.ModelProvider, error) {
	return usageModelProvider{factory: factory}, nil
}

type usageModelProvider struct {
	factory *usageModelProviderFactory
}

type cancelAfterGenerateProviderFactory struct {
	// cancel is invoked after the first Generate succeeds. It is set after the
	// service and run exist so it can call Service.CancelAgentRun, which cancels
	// the detached job context (the request ctx is no longer plumbed into the
	// background pipeline once ExecuteAgentRun returns).
	cancel func()
}

func (factory *cancelAfterGenerateProviderFactory) New(agentdomain.ProviderConnection) (ports.ModelProvider, error) {
	return cancelAfterGenerateProvider{factory: factory}, nil
}

type cancelAfterGenerateProvider struct {
	factory *cancelAfterGenerateProviderFactory
}

func (provider cancelAfterGenerateProvider) Generate(
	_ context.Context,
	_ string,
	_ ports.ModelGenerateRequest,
	_ func(string),
) (ports.ModelGenerateResult, error) {
	if provider.factory.cancel != nil {
		provider.factory.cancel()
	}
	return ports.ModelGenerateResult{
		Text:              `{"summary":"计划","scenes":[{"title":"场景","goal":"目标","conflict":"冲突","outcome":"结果"}]}`,
		ProviderRequestID: "request-cancel-after-success",
		InputTokens:       10,
		OutputTokens:      5,
	}, nil
}

func (provider usageModelProvider) Generate(
	_ context.Context,
	_ string,
	request ports.ModelGenerateRequest,
	_ func(string),
) (ports.ModelGenerateResult, error) {
	provider.factory.calls++
	return ports.ModelGenerateResult{
		Text:              `{"summary":"计划","scenes":[{"title":"场景","goal":"目标","conflict":"冲突","outcome":"结果"}]}`,
		ProviderRequestID: "request-usage",
		InputTokens:       5000,
		OutputTokens:      1,
	}, nil
}

// fakeLegacyImporter records calls to ImportLegacyProject for test assertions.
type fakeLegacyImporter struct {
	calls     int
	snapshots []ports.LegacyAgentSettingsSnapshot
	result    ports.LegacyImportResult
	err       error
}

func (f *fakeLegacyImporter) ImportLegacyProject(_ context.Context, snapshot ports.LegacyAgentSettingsSnapshot) (ports.LegacyImportResult, error) {
	f.calls++
	f.snapshots = append(f.snapshots, snapshot)
	return f.result, f.err
}

// createLegacyProject creates a project with provider connections and agent
// profiles but no profile selection pin. The returned service is closed; the
// caller reopens the project path with a fresh service to exercise migration.
func createLegacyProject(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	credentials := &fakeCredentialStore{secrets: map[string]string{}}
	service := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Unix(200, 0)},
		ids.UUIDGenerator{},
		&fakeSnapshotArchive{},
	)
	service.ConfigureAgentRuntime(credentials, &fakeModelProviderFactory{})
	// Deliberately do NOT call ConfigureProjectDefaults so no pin is written.
	view, err := service.CreateProject(ctx, t.TempDir(), "凡人修仙", "修仙", "简介")
	if err != nil {
		t.Fatal(err)
	}
	first, err := service.SaveProviderConnection(ctx, ProviderConnectionInput{
		Name: "OpenAI", Kind: "openai", BaseURL: "https://api.openai.com/v1", APIKey: "key-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := service.SaveProviderConnection(ctx, ProviderConnectionInput{
		Name: "Anthropic", Kind: "anthropic", BaseURL: "https://api.anthropic.com", APIKey: "key-b",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []AgentProfileInput{
		{Role: "planner", ConnectionID: first.ID, Model: "planner", MaxOutputTokens: 1000, Temperature: 0.2, Enabled: true},
		{Role: "writer", ConnectionID: first.ID, Model: "writer", MaxOutputTokens: 3000, Temperature: 0.7, Enabled: true},
		{Role: "continuity_reviewer", ConnectionID: second.ID, Model: "reviewer", MaxOutputTokens: 1000, Temperature: 0.1, Enabled: true},
	} {
		if _, err := service.SaveAgentProfile(ctx, input); err != nil {
			t.Fatal(err)
		}
	}
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	return view.Path
}

func TestLegacyOpenTriggersMigrationAndPinsProfile(t *testing.T) {
	ctx := context.Background()
	projectPath := createLegacyProject(t)

	importer := &fakeLegacyImporter{
		result: ports.LegacyImportResult{
			ProfileID:      "migrated-profile-1",
			ProfileVersion: 1,
			ProviderIDs:    []string{"provider-1", "provider-2"},
		},
	}

	service := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Unix(300, 0)},
		ids.UUIDGenerator{},
		&fakeSnapshotArchive{},
	)
	service.ConfigureLegacyImporter(importer)
	t.Cleanup(func() { _ = service.Close() })

	view, err := service.OpenProject(ctx, projectPath)
	if err != nil {
		t.Fatalf("OpenProject: %v", err)
	}
	if importer.calls != 1 {
		t.Fatalf("importer calls = %d, want 1", importer.calls)
	}
	if view.ProfileSelection.ProfileID != "migrated-profile-1" || view.ProfileSelection.Version != 1 {
		t.Fatalf("selection = %+v, want migrated-profile-1 v1", view.ProfileSelection)
	}
	if len(importer.snapshots) != 1 {
		t.Fatalf("snapshot count = %d", len(importer.snapshots))
	}
	snapshot := importer.snapshots[0]
	if snapshot.ProjectID == "" || snapshot.ProjectTitle != "凡人修仙" {
		t.Fatalf("snapshot project = %q/%q", snapshot.ProjectID, snapshot.ProjectTitle)
	}
	if len(snapshot.Connections) != 2 || len(snapshot.Profiles) != 3 {
		t.Fatalf("snapshot connections = %d, profiles = %d", len(snapshot.Connections), len(snapshot.Profiles))
	}
	if snapshot.SourceDigest == "" {
		t.Fatal("snapshot source digest is empty")
	}

	// Reopen: the pin written by the first open must prevent a second migration.
	if err := service.Close(); err != nil {
		t.Fatal(err)
	}
	reopened := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Unix(400, 0)},
		ids.UUIDGenerator{},
		&fakeSnapshotArchive{},
	)
	reopened.ConfigureLegacyImporter(importer)
	t.Cleanup(func() { _ = reopened.Close() })

	reopenedView, err := reopened.OpenProject(ctx, projectPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if importer.calls != 1 {
		t.Fatalf("importer calls after reopen = %d, want 1 (pin should skip migration)", importer.calls)
	}
	if reopenedView.ProfileSelection.ProfileID != "migrated-profile-1" || reopenedView.ProfileSelection.Version != 1 {
		t.Fatalf("reopen selection = %+v, want migrated-profile-1 v1", reopenedView.ProfileSelection)
	}
}

func TestLegacyReadOnlyOpenDoesNotWriteEitherDatabase(t *testing.T) {
	ctx := context.Background()
	projectPath := createLegacyProject(t)

	importer := &fakeLegacyImporter{
		result: ports.LegacyImportResult{
			ProfileID:      "migrated-profile-1",
			ProfileVersion: 1,
			ProviderIDs:    []string{"provider-1", "provider-2"},
		},
	}

	readOnly := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Unix(300, 0)},
		ids.UUIDGenerator{},
		&fakeSnapshotArchive{},
	)
	readOnly.ConfigureLegacyImporter(importer)
	t.Cleanup(func() { _ = readOnly.Close() })

	if _, err := readOnly.OpenProjectReadOnly(ctx, projectPath); err != nil {
		t.Fatalf("OpenProjectReadOnly: %v", err)
	}
	if importer.calls != 0 {
		t.Fatalf("importer calls after read-only open = %d, want 0", importer.calls)
	}

	// Close the read-only service and reopen editable. If the read-only open
	// had written a pin, the editable open would skip migration. Since it did
	// not, the editable open must trigger migration now.
	if err := readOnly.Close(); err != nil {
		t.Fatal(err)
	}
	editable := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Unix(400, 0)},
		ids.UUIDGenerator{},
		&fakeSnapshotArchive{},
	)
	editable.ConfigureLegacyImporter(importer)
	t.Cleanup(func() { _ = editable.Close() })

	if _, err := editable.OpenProject(ctx, projectPath); err != nil {
		t.Fatalf("OpenProject after read-only: %v", err)
	}
	if importer.calls != 1 {
		t.Fatalf("importer calls after editable open = %d, want 1 (read-only must not have written a pin)", importer.calls)
	}
}

func TestLegacyOpenAlreadyPinnedSkipsMigration(t *testing.T) {
	ctx := context.Background()

	// Create the project WITH projectDefaults so a pin is written during
	// CreateProject. When the project is reopened with a legacy importer
	// configured, the existing pin must prevent migration.
	createDefaults := &fakeProjectDefaults{selection: ports.ProjectProfileSelection{ProfileID: "pinned-profile-1", Version: 3}}
	createService := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Unix(200, 0)},
		ids.UUIDGenerator{},
		&fakeSnapshotArchive{},
	)
	createService.ConfigureProjectDefaults(createDefaults)
	createService.ConfigureAgentRuntime(
		&fakeCredentialStore{secrets: map[string]string{}},
		&fakeModelProviderFactory{},
	)
	view, err := createService.CreateProject(ctx, t.TempDir(), "已固定项目", "修仙", "")
	if err != nil {
		t.Fatal(err)
	}
	// Add legacy provider connections and profiles so the migration trigger
	// would fire if the pin did not exist.
	first, err := createService.SaveProviderConnection(ctx, ProviderConnectionInput{
		Name: "OpenAI", Kind: "openai", BaseURL: "https://api.openai.com/v1", APIKey: "key-a",
	})
	if err != nil {
		t.Fatal(err)
	}
	second, err := createService.SaveProviderConnection(ctx, ProviderConnectionInput{
		Name: "Anthropic", Kind: "anthropic", BaseURL: "https://api.anthropic.com", APIKey: "key-b",
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, input := range []AgentProfileInput{
		{Role: "planner", ConnectionID: first.ID, Model: "planner", MaxOutputTokens: 1000, Temperature: 0.2, Enabled: true},
		{Role: "writer", ConnectionID: first.ID, Model: "writer", MaxOutputTokens: 3000, Temperature: 0.7, Enabled: true},
		{Role: "continuity_reviewer", ConnectionID: second.ID, Model: "reviewer", MaxOutputTokens: 1000, Temperature: 0.1, Enabled: true},
	} {
		if _, err := createService.SaveAgentProfile(ctx, input); err != nil {
			t.Fatal(err)
		}
	}
	if err := createService.Close(); err != nil {
		t.Fatal(err)
	}

	importer := &fakeLegacyImporter{
		result: ports.LegacyImportResult{
			ProfileID:      "migrated-profile-1",
			ProfileVersion: 1,
		},
	}
	reopened := NewService(
		projectfs.Manager{},
		sqlite.Factory{},
		license.NoopService{},
		fixedClock{now: time.Unix(300, 0)},
		ids.UUIDGenerator{},
		&fakeSnapshotArchive{},
	)
	reopened.ConfigureLegacyImporter(importer)
	t.Cleanup(func() { _ = reopened.Close() })

	reopenedView, err := reopened.OpenProject(ctx, view.Path)
	if err != nil {
		t.Fatalf("OpenProject: %v", err)
	}
	if importer.calls != 0 {
		t.Fatalf("importer calls = %d, want 0 (pin should skip migration)", importer.calls)
	}
	if reopenedView.ProfileSelection.ProfileID != "pinned-profile-1" || reopenedView.ProfileSelection.Version != 3 {
		t.Fatalf("selection = %+v, want pinned-profile-1 v3", reopenedView.ProfileSelection)
	}
}
