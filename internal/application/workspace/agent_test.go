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
	got, err := service.ExecuteAgentRun(context.Background(), run.ID)
	if !errors.Is(err, ErrTokenBudgetExceeded) {
		t.Fatalf("ExecuteAgentRun() error = %v, want ErrTokenBudgetExceeded", err)
	}
	if providerFactory.calls != 1 {
		t.Fatalf("provider calls = %d, want 1", providerFactory.calls)
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
	got, err := service.ExecuteAgentRun(context.Background(), run.ID)
	if !errors.Is(err, ErrContextBudgetExceeded) {
		t.Fatalf("ExecuteAgentRun() error = %v, want ErrContextBudgetExceeded", err)
	}
	if providerFactory.calls != 0 {
		t.Fatalf("provider calls = %d, want 0", providerFactory.calls)
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
	result := make(chan struct {
		run AgentRunView
		err error
	}, 1)
	go func() {
		got, executeErr := service.ExecuteAgentRun(context.Background(), run.ID)
		result <- struct {
			run AgentRunView
			err error
		}{run: got, err: executeErr}
	}()
	select {
	case <-providerFactory.started:
	case <-time.After(3 * time.Second):
		t.Fatal("provider did not start")
	}
	if err := service.CancelAgentRun(context.Background(), run.ID); err != nil {
		t.Fatalf("CancelAgentRun() error = %v", err)
	}
	select {
	case outcome := <-result:
		if outcome.err != nil {
			t.Fatalf("ExecuteAgentRun() error after cancellation = %v", outcome.err)
		}
		if outcome.run.Status != "cancelled" {
			t.Fatalf("run status = %q", outcome.run.Status)
		}
		if len(outcome.run.Steps) != 3 ||
			outcome.run.Steps[0].Status != "cancelled" ||
			outcome.run.Steps[1].Status != "cancelled" {
			t.Fatalf("unexpected cancelled steps: %#v", outcome.run.Steps)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("cancelled run did not finish")
	}
}

func TestRetryAgentRunCreatesNewQueuedAuditRecord(t *testing.T) {
	service, chapter := configuredAgentService(t, failingModelProviderFactory{})
	defer service.Close()
	original, err := service.CreateChapterAgentRun(context.Background(), chapter.ID, "沿用原目标")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.ExecuteAgentRun(context.Background(), original.ID); err == nil {
		t.Fatal("expected provider failure")
	}
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
	if err := service.CancelAgentRun(context.Background(), run.ID); err != nil {
		t.Fatal(err)
	}
	<-done
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
